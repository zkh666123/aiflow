package shares

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

var (
	ErrShareNotFound       = errors.New("share not found")
	ErrApplicationNotFound = errors.New("application not found")
)

type Theme string

const (
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"
	ThemeAuto  Theme = "auto"
)

type EmbedConfig struct {
	AllowedOrigins []string `json:"allowedOrigins,omitempty"`
	Theme          Theme    `json:"theme,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty"`
	Width          string   `json:"width,omitempty"`
	Height         string   `json:"height,omitempty"`
	ShowHeader     *bool    `json:"showHeader,omitempty"`
}

type Share struct {
	ID            string       `json:"id"`
	ApplicationID string       `json:"applicationId"`
	ShareLink     string       `json:"shareLink"`
	IsPublic      bool         `json:"isPublic"`
	AccessCount   int32        `json:"accessCount"`
	EmbedConfig   *EmbedConfig `json:"embedConfig"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`
}

type PublicApplication struct {
	ID            string       `json:"-"`
	ApplicationID string       `json:"id"`
	ShareLink     string       `json:"shareLink"`
	IsPublic      bool         `json:"isPublic"`
	Name          string       `json:"name"`
	Description   *string      `json:"description"`
	Icon          *string      `json:"icon"`
	Status        string       `json:"status"`
	EmbedConfig   *EmbedConfig `json:"embedConfig"`
}

type UpdateInput struct {
	SetIsPublic    bool
	IsPublic       bool
	SetEmbedConfig bool
	EmbedConfig    *EmbedConfig
}

type EmbedCode struct {
	ShareURL    string       `json:"shareUrl"`
	IframeCode  string       `json:"iframeCode"`
	ScriptTag   string       `json:"scriptTag"`
	ScriptCode  string       `json:"scriptCode"`
	EmbedConfig *EmbedConfig `json:"embedConfig"`
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

func (err *ServiceError) Error() string { return err.Message }
func (err *ServiceError) Unwrap() error { return err.Cause }

type Store interface {
	ApplicationOwnerID(context.Context, string) (string, error)
	GetShareByApplicationID(context.Context, string) (Share, error)
	CreateShare(context.Context, string, string) (Share, error)
	UpdateShare(context.Context, string, UpdateInput) (Share, error)
	DeleteShare(context.Context, string) error
	GetPublicShareByLink(context.Context, string) (PublicApplication, error)
	IncrementShareAccess(context.Context, string) error
}

type Service struct {
	store       Store
	random      io.Reader
	frontendURL string
}

func NewService(store Store, random io.Reader, frontendURL string) (*Service, error) {
	if store == nil {
		return nil, errors.New("share store is required")
	}
	if random == nil {
		return nil, errors.New("share random source is required")
	}
	parsed, err := url.Parse(frontendURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return nil, errors.New("frontend URL must be an absolute HTTP(S) URL without credentials")
	}
	return &Service{store: store, random: random, frontendURL: strings.TrimRight(frontendURL, "/")}, nil
}

func (service *Service) Generate(ctx context.Context, userID, applicationID string) (Share, error) {
	if err := service.authorizeOwner(ctx, userID, applicationID); err != nil {
		return Share{}, err
	}
	existing, err := service.store.GetShareByApplicationID(ctx, applicationID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrShareNotFound) {
		return Share{}, shareStoreError("Failed to get application share", err)
	}

	material := make([]byte, 16)
	if _, err := io.ReadFull(service.random, material); err != nil {
		return Share{}, &ServiceError{Kind: ErrorInternal, Message: "Failed to generate share link", Cause: err}
	}
	share, err := service.store.CreateShare(ctx, applicationID, "share-"+hex.EncodeToString(material))
	if err != nil {
		return Share{}, shareStoreError("Failed to create application share", err)
	}
	return share, nil
}

func (service *Service) Get(ctx context.Context, userID, applicationID string) (Share, error) {
	if err := service.authorizeOwner(ctx, userID, applicationID); err != nil {
		return Share{}, err
	}
	share, err := service.store.GetShareByApplicationID(ctx, applicationID)
	if err != nil {
		return Share{}, shareStoreError("Failed to get application share", err)
	}
	return share, nil
}

func (service *Service) Update(ctx context.Context, userID, applicationID string, input UpdateInput) (Share, error) {
	if !input.SetIsPublic && !input.SetEmbedConfig {
		return Share{}, &ServiceError{Kind: ErrorInvalidInput, Message: "At least one share setting is required"}
	}
	if input.SetEmbedConfig {
		if input.EmbedConfig == nil {
			return Share{}, &ServiceError{Kind: ErrorInvalidInput, Message: "embedConfig must be an object"}
		}
		if err := validateEmbedConfig(*input.EmbedConfig); err != nil {
			return Share{}, err
		}
	}
	if err := service.authorizeOwner(ctx, userID, applicationID); err != nil {
		return Share{}, err
	}
	share, err := service.store.UpdateShare(ctx, applicationID, input)
	if err != nil {
		return Share{}, shareStoreError("Failed to update application share", err)
	}
	return share, nil
}

func (service *Service) Revoke(ctx context.Context, userID, applicationID string) error {
	if err := service.authorizeOwner(ctx, userID, applicationID); err != nil {
		return err
	}
	if err := service.store.DeleteShare(ctx, applicationID); err != nil {
		return shareStoreError("Failed to revoke application share", err)
	}
	return nil
}

func (service *Service) GetPublic(ctx context.Context, shareLink string) (PublicApplication, error) {
	if !validShareLink(shareLink) {
		return PublicApplication{}, &ServiceError{Kind: ErrorNotFound, Message: "Shared application not found"}
	}
	application, err := service.store.GetPublicShareByLink(ctx, shareLink)
	if err != nil {
		return PublicApplication{}, shareStoreError("Failed to get shared application", err)
	}
	// Access statistics are best-effort and do not block a valid public response.
	_ = service.store.IncrementShareAccess(ctx, application.ID)
	return application, nil
}

func (service *Service) Embed(ctx context.Context, userID, applicationID string) (EmbedCode, error) {
	share, err := service.Get(ctx, userID, applicationID)
	if err != nil {
		if serviceErr := new(ServiceError); errors.As(err, &serviceErr) && serviceErr.Kind == ErrorNotFound {
			return EmbedCode{}, &ServiceError{Kind: ErrorForbidden, Message: "Please generate a share link first", Cause: err}
		}
		return EmbedCode{}, err
	}
	if !validShareLink(share.ShareLink) {
		return EmbedCode{}, &ServiceError{Kind: ErrorInternal, Message: "Stored share link is invalid"}
	}
	theme := ThemeLight
	if share.EmbedConfig != nil && share.EmbedConfig.Theme != "" {
		theme = share.EmbedConfig.Theme
	}
	if !validTheme(theme) {
		return EmbedCode{}, &ServiceError{Kind: ErrorInternal, Message: "Stored embed theme is invalid"}
	}
	shareURL := service.frontendURL + "/share/" + share.ShareLink
	iframe := fmt.Sprintf(`<iframe src="%s" width="100%%" height="600" frameborder="0" style="border-radius: 8px;"></iframe>`, shareURL)
	script := fmt.Sprintf(`<script src="%s/embed.js" data-app="%s" data-theme="%s"></script>`, service.frontendURL, share.ShareLink, theme)
	return EmbedCode{ShareURL: shareURL, IframeCode: iframe, ScriptTag: script, ScriptCode: script, EmbedConfig: share.EmbedConfig}, nil
}

func (service *Service) authorizeOwner(ctx context.Context, userID, applicationID string) error {
	ownerID, err := service.store.ApplicationOwnerID(ctx, applicationID)
	if err != nil {
		return shareStoreError("Failed to verify application ownership", err)
	}
	if ownerID != userID {
		return &ServiceError{Kind: ErrorForbidden, Message: "Only the application owner can manage sharing"}
	}
	return nil
}

func validateEmbedConfig(config EmbedConfig) error {
	if config.Theme != "" && !validTheme(config.Theme) {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Theme must be light, dark, or auto"}
	}
	for _, origin := range config.AllowedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil ||
			(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return &ServiceError{Kind: ErrorInvalidInput, Message: "Allowed origins must be absolute HTTP(S) origins"}
		}
	}
	if len(config.Width) > 32 || len(config.Height) > 32 {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Embed dimensions are too long"}
	}
	return nil
}

func validTheme(theme Theme) bool {
	return theme == ThemeLight || theme == ThemeDark || theme == ThemeAuto
}

func validShareLink(value string) bool {
	if len(value) != 38 || !strings.HasPrefix(value, "share-") {
		return false
	}
	for _, character := range value[6:] {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func shareStoreError(message string, err error) *ServiceError {
	switch {
	case errors.Is(err, ErrApplicationNotFound):
		return &ServiceError{Kind: ErrorNotFound, Message: "Application not found", Cause: err}
	case errors.Is(err, ErrShareNotFound):
		return &ServiceError{Kind: ErrorNotFound, Message: "Application share not found", Cause: err}
	default:
		return &ServiceError{Kind: ErrorInternal, Message: message, Cause: err}
	}
}
