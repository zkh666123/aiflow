package apikeys

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeAPIKeyStore struct {
	records          map[string]Record
	created          Record
	ownerID          string
	err              error
	touchErr         error
	touched          string
	deleted          string
	toggled          string
	applicationOwner string
}

func (store *fakeAPIKeyStore) CreateAPIKey(_ context.Context, record Record) (Record, error) {
	store.created = record
	if store.err != nil {
		return Record{}, store.err
	}
	record.ID = "4ab1ea39-b2a4-4850-b719-ae5ad57773f1"
	record.CreatedAt = time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	record.IsActive = true
	return record, nil
}

func (store *fakeAPIKeyStore) ListAPIKeys(context.Context, string, *string) ([]Record, error) {
	result := make([]Record, 0, len(store.records))
	for _, record := range store.records {
		result = append(result, record)
	}
	return result, store.err
}

func (store *fakeAPIKeyStore) GetAPIKeyByDigest(_ context.Context, digest []byte) (Record, error) {
	record, exists := store.records[hex.EncodeToString(digest)]
	if !exists {
		return Record{}, ErrAPIKeyNotFound
	}
	return record, store.err
}

func (store *fakeAPIKeyStore) DeleteAPIKey(_ context.Context, userID, keyID string) error {
	store.ownerID = userID
	store.deleted = keyID
	return store.err
}

func (store *fakeAPIKeyStore) SetAPIKeyActive(_ context.Context, userID, keyID string, active bool) (Record, error) {
	store.ownerID = userID
	store.toggled = keyID
	return Record{APIKey: APIKey{ID: keyID, Name: "key", IsActive: active}}, store.err
}

func (store *fakeAPIKeyStore) TouchAPIKey(_ context.Context, keyID string) error {
	store.touched = keyID
	if store.touchErr != nil {
		return store.touchErr
	}
	return store.err
}

func (store *fakeAPIKeyStore) ApplicationOwnerID(context.Context, string) (string, error) {
	if store.err != nil {
		return "", store.err
	}
	return store.applicationOwner, nil
}

func deterministicRandom() []byte {
	value := make([]byte, 32)
	for index := range value {
		value[index] = byte(index)
	}
	return value
}

func newAPIKeyService(t *testing.T, store Store, current, previous string, random []byte, now time.Time) *Service {
	t.Helper()
	service, err := NewService(store, current, previous, bytes.NewReader(random), func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func TestCreateAPIKeyUsesRandomMaterialAndHMACOnly(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	secret := strings.Repeat("k", 32)
	store := &fakeAPIKeyStore{}
	service := newAPIKeyService(t, store, secret, "", deterministicRandom(), now)

	result, err := service.Create(context.Background(), "user-1", CreateInput{Name: "deploy"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(result.Key) != 67 || !strings.HasPrefix(result.Key, "sk-") {
		t.Fatalf("raw key = %q", result.Key)
	}
	if _, err := hex.DecodeString(result.Key[3:]); err != nil {
		t.Fatalf("key is not lowercase hex: %v", err)
	}
	if result.KeyPrefix != result.Key[:7] || store.created.KeyPrefix != result.Key[:7] {
		t.Fatalf("prefixes = %q / %q", result.KeyPrefix, store.created.KeyPrefix)
	}
	wantMAC := hmac.New(sha256.New, []byte(secret))
	_, _ = wantMAC.Write([]byte(result.Key))
	wantDigest := wantMAC.Sum(nil)
	if !hmac.Equal(store.created.Digest, wantDigest) {
		t.Fatalf("digest = %x, want %x", store.created.Digest, wantDigest)
	}
	plainSHA := sha256.Sum256([]byte(result.Key))
	if bytes.Equal(store.created.Digest, plainSHA[:]) {
		t.Fatal("API key used unkeyed SHA-256")
	}
	if len(result.Scopes) != 2 || result.Scopes[0] != ScopeAppRead || result.Scopes[1] != ScopeWorkflowExecute {
		t.Fatalf("default scopes = %#v", result.Scopes)
	}
}

func TestCreateAPIKeyPreservesExplicitEmptyScopes(t *testing.T) {
	store := &fakeAPIKeyStore{}
	service := newAPIKeyService(
		t,
		store,
		strings.Repeat("k", 32),
		"",
		deterministicRandom(),
		time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC),
	)

	created, err := service.Create(context.Background(), "user-1", CreateInput{Name: "no-access", Scopes: []Scope{}})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Scopes == nil || len(created.Scopes) != 0 || store.created.Scopes == nil {
		t.Fatalf("explicit empty scopes became defaults or null: created=%#v stored=%#v", created.Scopes, store.created.Scopes)
	}
}

func TestCreateAPIKeyValidatesInputAndApplicationOwnership(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	store := &fakeAPIKeyStore{applicationOwner: "someone-else"}
	service := newAPIKeyService(t, store, strings.Repeat("k", 32), "", deterministicRandom(), now)
	applicationID := "app-1"

	_, err := service.Create(context.Background(), "user-1", CreateInput{Name: "key", ApplicationID: &applicationID})
	assertAPIKeyError(t, err, ErrorForbidden)

	tests := []CreateInput{
		{Name: ""},
		{Name: "key", Scopes: []Scope{"unknown"}},
		{Name: "key", ExpiresAt: timePointer(now)},
	}
	for _, input := range tests {
		_, err := service.Create(context.Background(), "user-1", input)
		assertAPIKeyError(t, err, ErrorInvalidInput)
	}
}

func TestListNeverExposesDigestOrPlaintext(t *testing.T) {
	store := &fakeAPIKeyStore{records: map[string]Record{
		"one": {APIKey: APIKey{ID: "key-1", Name: "deploy", KeyPrefix: "sk-0000", Scopes: []Scope{ScopeAppRead}}, Digest: []byte("secret digest")},
	}}
	service := newAPIKeyService(t, store, strings.Repeat("k", 32), "", deterministicRandom(), time.Now())

	keys, err := service.List(context.Background(), "user", nil)
	if err != nil || len(keys) != 1 || keys[0].ID != "key-1" {
		t.Fatalf("keys = %#v, error = %v", keys, err)
	}
}

func TestValidateSupportsPreviousSecretAndRejectsInactiveExpiredOrWrongKeys(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	current := strings.Repeat("c", 32)
	previous := strings.Repeat("p", 32)
	raw := "sk-" + strings.Repeat("a", 64)
	previousDigest := digestKey([]byte(previous), raw)
	record := Record{APIKey: APIKey{
		ID: "key-1", UserID: "user-1", Scopes: []Scope{ScopeAppRead}, IsActive: true,
	}, Digest: previousDigest}
	store := &fakeAPIKeyStore{records: map[string]Record{hex.EncodeToString(previousDigest): record}}
	service := newAPIKeyService(t, store, current, previous, deterministicRandom(), now)

	credential, err := service.Validate(context.Background(), raw)
	if err != nil || credential.UserID != "user-1" || store.touched != "key-1" {
		t.Fatalf("credential = %#v, touched = %q, error = %v", credential, store.touched, err)
	}

	for _, mutate := range []func(*Record){
		func(record *Record) { record.IsActive = false },
		func(record *Record) { expired := now.Add(-time.Second); record.ExpiresAt = &expired },
	} {
		invalid := record
		mutate(&invalid)
		store.records[hex.EncodeToString(previousDigest)] = invalid
		_, err := service.Validate(context.Background(), raw)
		if !errors.Is(err, ErrInvalidAPIKey) {
			t.Fatalf("error = %v, want ErrInvalidAPIKey", err)
		}
	}

	_, err = service.Validate(context.Background(), "sk-"+strings.Repeat("b", 64))
	if !errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("wrong key error = %v", err)
	}
}

func TestValidateKeepsCredentialValidWhenUsageTouchFails(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	secret := strings.Repeat("k", 32)
	raw := "sk-" + strings.Repeat("a", 64)
	digest := digestKey([]byte(secret), raw)
	store := &fakeAPIKeyStore{
		records: map[string]Record{
			hex.EncodeToString(digest): {
				APIKey: APIKey{ID: "key-1", UserID: "user-1", IsActive: true},
				Digest: digest,
			},
		},
		touchErr: errors.New("touch unavailable"),
	}
	service := newAPIKeyService(t, store, secret, "", deterministicRandom(), now)

	credential, err := service.Validate(context.Background(), raw)
	if err != nil || credential.UserID != "user-1" || store.touched != "key-1" {
		t.Fatalf("credential = %#v, touched = %q, error = %v", credential, store.touched, err)
	}
}

func TestToggleAndDeleteKeepOwnerScopeInTheStore(t *testing.T) {
	store := &fakeAPIKeyStore{}
	service := newAPIKeyService(t, store, strings.Repeat("k", 32), "", deterministicRandom(), time.Now())

	key, err := service.Toggle(context.Background(), "owner", "key-1", false)
	if err != nil || key.IsActive || store.ownerID != "owner" || store.toggled != "key-1" {
		t.Fatalf("key = %#v, owner = %q, toggled = %q, error = %v", key, store.ownerID, store.toggled, err)
	}
	if err := service.Delete(context.Background(), "owner", "key-1"); err != nil || store.deleted != "key-1" {
		t.Fatalf("deleted = %q, error = %v", store.deleted, err)
	}
}

func timePointer(value time.Time) *time.Time {
	return &value
}

func assertAPIKeyError(t *testing.T, err error, kind ErrorKind) *ServiceError {
	t.Helper()
	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) {
		t.Fatalf("error = %v, want ServiceError", err)
	}
	if serviceErr.Kind != kind {
		t.Fatalf("kind = %q, want %q", serviceErr.Kind, kind)
	}
	return serviceErr
}
