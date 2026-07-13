package apikeys

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrAPIKeyNotFound      = errors.New("API key not found")
	ErrApplicationNotFound = errors.New("application not found")
	ErrInvalidAPIKey       = errors.New("invalid API key")
)

type Scope string

const (
	ScopeAppRead         Scope = "app:read"
	ScopeAppWrite        Scope = "app:write"
	ScopeAppExecute      Scope = "app:execute"
	ScopeWorkflowRead    Scope = "workflow:read"
	ScopeWorkflowWrite   Scope = "workflow:write"
	ScopeWorkflowExecute Scope = "workflow:execute"
	ScopeKnowledgeRead   Scope = "knowledge:read"
	ScopeKnowledgeWrite  Scope = "knowledge:write"
)

var defaultScopes = []Scope{ScopeAppRead, ScopeWorkflowExecute}

type APIKey struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	KeyPrefix     string     `json:"keyPrefix"`
	Scopes        []Scope    `json:"scopes"`
	IsActive      bool       `json:"isActive"`
	LastUsedAt    *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	UserID        string     `json:"-"`
	ApplicationID *string    `json:"applicationId,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

type Record struct {
	APIKey
	Digest []byte
}

type CreatedAPIKey struct {
	APIKey
	Key string `json:"key"`
}

type Credential struct {
	UserID        string
	ApplicationID *string
	Scopes        []Scope
}

type CreateInput struct {
	Name          string
	ApplicationID *string
	Scopes        []Scope
	ExpiresAt     *time.Time
}

type ErrorKind string

const (
	ErrorInvalidInput ErrorKind = "invalid_input"
	ErrorForbidden    ErrorKind = "forbidden"
	ErrorNotFound     ErrorKind = "not_found"
	ErrorInternal     ErrorKind = "internal"
)

type ServiceError struct {
	Kind    ErrorKind
	Message string
	Cause   error
}

func (err *ServiceError) Error() string {
	return err.Message
}

func (err *ServiceError) Unwrap() error {
	return err.Cause
}

type Store interface {
	CreateAPIKey(context.Context, Record) (Record, error)
	ListAPIKeys(context.Context, string, *string) ([]Record, error)
	GetAPIKeyByDigest(context.Context, []byte) (Record, error)
	DeleteAPIKey(context.Context, string, string) error
	SetAPIKeyActive(context.Context, string, string, bool) (Record, error)
	TouchAPIKey(context.Context, string) error
	ApplicationOwnerID(context.Context, string) (string, error)
}

type Service struct {
	store          Store
	currentSecret  []byte
	previousSecret []byte
	random         io.Reader
	now            func() time.Time
}

func NewService(store Store, currentSecret, previousSecret string, random io.Reader, now func() time.Time) (*Service, error) {
	if store == nil {
		return nil, errors.New("API key store is required")
	}
	if len(currentSecret) < 32 || strings.TrimSpace(currentSecret) == "" {
		return nil, errors.New("current API key HMAC secret must contain at least 32 non-blank characters")
	}
	if previousSecret != "" && (len(previousSecret) < 32 || strings.TrimSpace(previousSecret) == "") {
		return nil, errors.New("previous API key HMAC secret must contain at least 32 non-blank characters")
	}
	if previousSecret != "" && hmac.Equal([]byte(currentSecret), []byte(previousSecret)) {
		return nil, errors.New("current and previous API key HMAC secrets must differ")
	}
	if random == nil {
		return nil, errors.New("API key random source is required")
	}
	if now == nil {
		return nil, errors.New("API key clock is required")
	}
	return &Service{
		store:          store,
		currentSecret:  []byte(currentSecret),
		previousSecret: []byte(previousSecret),
		random:         random,
		now:            now,
	}, nil
}

func (service *Service) Create(ctx context.Context, userID string, input CreateInput) (CreatedAPIKey, error) {
	if err := service.validateCreateInput(ctx, userID, &input); err != nil {
		return CreatedAPIKey{}, err
	}

	material := make([]byte, 32)
	if _, err := io.ReadFull(service.random, material); err != nil {
		return CreatedAPIKey{}, &ServiceError{Kind: ErrorInternal, Message: "Failed to generate API key", Cause: err}
	}
	rawKey := "sk-" + hex.EncodeToString(material)
	record, err := service.store.CreateAPIKey(ctx, Record{
		APIKey: APIKey{
			Name:          input.Name,
			KeyPrefix:     rawKey[:7],
			Scopes:        cloneScopes(input.Scopes),
			ExpiresAt:     input.ExpiresAt,
			UserID:        userID,
			ApplicationID: input.ApplicationID,
		},
		Digest: digestKey(service.currentSecret, rawKey),
	})
	if err != nil {
		return CreatedAPIKey{}, apiKeyStoreError("Failed to create API key", err)
	}
	return CreatedAPIKey{APIKey: publicAPIKey(record), Key: rawKey}, nil
}

func (service *Service) List(ctx context.Context, userID string, applicationID *string) ([]APIKey, error) {
	records, err := service.store.ListAPIKeys(ctx, userID, applicationID)
	if err != nil {
		return nil, apiKeyStoreError("Failed to list API keys", err)
	}
	keys := make([]APIKey, 0, len(records))
	for _, record := range records {
		keys = append(keys, publicAPIKey(record))
	}
	return keys, nil
}

func (service *Service) Delete(ctx context.Context, userID, keyID string) error {
	if err := service.store.DeleteAPIKey(ctx, userID, keyID); err != nil {
		return apiKeyStoreError("Failed to delete API key", err)
	}
	return nil
}

func (service *Service) Toggle(ctx context.Context, userID, keyID string, active bool) (APIKey, error) {
	record, err := service.store.SetAPIKeyActive(ctx, userID, keyID, active)
	if err != nil {
		return APIKey{}, apiKeyStoreError("Failed to update API key", err)
	}
	return publicAPIKey(record), nil
}

func (service *Service) Validate(ctx context.Context, rawKey string) (Credential, error) {
	if !validRawKey(rawKey) {
		return Credential{}, ErrInvalidAPIKey
	}

	record, candidateDigest, err := service.findRecord(ctx, rawKey)
	if err != nil {
		return Credential{}, err
	}
	if !hmac.Equal(record.Digest, candidateDigest) || !record.IsActive {
		return Credential{}, ErrInvalidAPIKey
	}
	now := service.now()
	if record.ExpiresAt != nil && !record.ExpiresAt.After(now) {
		return Credential{}, ErrInvalidAPIKey
	}
	if err := service.store.TouchAPIKey(ctx, record.ID); err != nil {
		return Credential{}, &ServiceError{Kind: ErrorInternal, Message: "Failed to update API key usage", Cause: err}
	}
	return Credential{
		UserID:        record.UserID,
		ApplicationID: cloneStringPointer(record.ApplicationID),
		Scopes:        cloneScopes(record.Scopes),
	}, nil
}

func (service *Service) findRecord(ctx context.Context, rawKey string) (Record, []byte, error) {
	secrets := [][]byte{service.currentSecret}
	if len(service.previousSecret) != 0 {
		secrets = append(secrets, service.previousSecret)
	}
	for _, secret := range secrets {
		digest := digestKey(secret, rawKey)
		record, err := service.store.GetAPIKeyByDigest(ctx, digest)
		if err == nil {
			return record, digest, nil
		}
		if !errors.Is(err, ErrAPIKeyNotFound) {
			return Record{}, nil, &ServiceError{Kind: ErrorInternal, Message: "Failed to validate API key", Cause: err}
		}
	}
	return Record{}, nil, ErrInvalidAPIKey
}

func (service *Service) validateCreateInput(ctx context.Context, userID string, input *CreateInput) error {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 100 {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Name must contain 1-100 characters"}
	}
	if input.Scopes == nil {
		input.Scopes = cloneScopes(defaultScopes)
	}
	seen := make(map[Scope]struct{}, len(input.Scopes))
	for _, scope := range input.Scopes {
		if !ValidScope(scope) {
			return &ServiceError{Kind: ErrorInvalidInput, Message: fmt.Sprintf("Unsupported API key scope: %s", scope)}
		}
		if _, exists := seen[scope]; exists {
			return &ServiceError{Kind: ErrorInvalidInput, Message: fmt.Sprintf("Duplicate API key scope: %s", scope)}
		}
		seen[scope] = struct{}{}
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(service.now()) {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Expiration time must be in the future"}
	}
	if input.ApplicationID == nil {
		return nil
	}
	ownerID, err := service.store.ApplicationOwnerID(ctx, *input.ApplicationID)
	if err != nil {
		return apiKeyStoreError("Failed to verify application ownership", err)
	}
	if ownerID != userID {
		return &ServiceError{Kind: ErrorForbidden, Message: "Only the application owner can create an API key"}
	}
	return nil
}

func ValidScope(scope Scope) bool {
	switch scope {
	case ScopeAppRead, ScopeAppWrite, ScopeAppExecute,
		ScopeWorkflowRead, ScopeWorkflowWrite, ScopeWorkflowExecute,
		ScopeKnowledgeRead, ScopeKnowledgeWrite:
		return true
	default:
		return false
	}
}

func validRawKey(rawKey string) bool {
	if len(rawKey) != 67 || !strings.HasPrefix(rawKey, "sk-") {
		return false
	}
	for _, character := range rawKey[3:] {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func digestKey(secret []byte, rawKey string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(rawKey))
	return mac.Sum(nil)
}

func publicAPIKey(record Record) APIKey {
	key := record.APIKey
	key.Scopes = cloneScopes(record.Scopes)
	key.ApplicationID = cloneStringPointer(record.ApplicationID)
	return key
}

func cloneScopes(scopes []Scope) []Scope {
	if scopes == nil {
		return nil
	}
	result := make([]Scope, len(scopes))
	copy(result, scopes)
	return result
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func apiKeyStoreError(message string, err error) *ServiceError {
	if errors.Is(err, ErrAPIKeyNotFound) {
		return &ServiceError{Kind: ErrorNotFound, Message: "API key not found", Cause: err}
	}
	if errors.Is(err, ErrApplicationNotFound) {
		return &ServiceError{Kind: ErrorNotFound, Message: "Application not found", Cause: err}
	}
	return &ServiceError{Kind: ErrorInternal, Message: message, Cause: err}
}
