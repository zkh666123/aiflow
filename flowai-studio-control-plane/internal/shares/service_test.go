package shares

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeShareStore struct {
	ownerID      string
	share        Share
	public       PublicApplication
	err          error
	createdLink  string
	updated      UpdateInput
	deletedAppID string
	incremented  string
}

func (store *fakeShareStore) ApplicationOwnerID(context.Context, string) (string, error) {
	if store.err != nil {
		return "", store.err
	}
	return store.ownerID, nil
}

func (store *fakeShareStore) GetShareByApplicationID(context.Context, string) (Share, error) {
	if store.err != nil {
		return Share{}, store.err
	}
	if store.share.ID == "" {
		return Share{}, ErrShareNotFound
	}
	return store.share, nil
}

func (store *fakeShareStore) CreateShare(_ context.Context, applicationID, shareLink string) (Share, error) {
	store.createdLink = shareLink
	if store.err != nil {
		return Share{}, store.err
	}
	store.share = sampleShare()
	store.share.ApplicationID = applicationID
	store.share.ShareLink = shareLink
	store.share.IsPublic = true
	return store.share, nil
}

func (store *fakeShareStore) UpdateShare(_ context.Context, applicationID string, input UpdateInput) (Share, error) {
	store.updated = input
	if store.err != nil {
		return Share{}, store.err
	}
	share := store.share
	share.ApplicationID = applicationID
	if input.SetIsPublic {
		share.IsPublic = input.IsPublic
	}
	if input.SetEmbedConfig {
		share.EmbedConfig = input.EmbedConfig
	}
	return share, nil
}

func (store *fakeShareStore) DeleteShare(_ context.Context, applicationID string) error {
	store.deletedAppID = applicationID
	return store.err
}

func (store *fakeShareStore) GetPublicShareByLink(context.Context, string) (PublicApplication, error) {
	if store.err != nil {
		return PublicApplication{}, store.err
	}
	if store.public.ID == "" {
		return PublicApplication{}, ErrShareNotFound
	}
	return store.public, nil
}

func (store *fakeShareStore) IncrementShareAccess(_ context.Context, shareID string) error {
	store.incremented = shareID
	return store.err
}

func sampleShare() Share {
	now := time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC)
	return Share{
		ID: "516f794a-eb09-4e7d-bdbd-d4e2bc5f14da", ApplicationID: "7a611d9a-b555-4469-a289-f1672daefce3",
		ShareLink: "share-00112233445566778899aabbccddeeff", IsPublic: true, CreatedAt: now, UpdatedAt: now,
	}
}

func shareRandom() []byte {
	return []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
}

func newShareService(t *testing.T, store Store) *Service {
	t.Helper()
	service, err := NewService(store, bytes.NewReader(shareRandom()), "http://127.0.0.1:5173")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func TestGenerateShareIsOwnerOnlyIdempotentAndUses128RandomBits(t *testing.T) {
	store := &fakeShareStore{ownerID: "owner"}
	service := newShareService(t, store)

	created, err := service.Generate(context.Background(), "owner", "app-1")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if store.createdLink != "share-00112233445566778899aabbccddeeff" || created.ShareLink != store.createdLink || !created.IsPublic {
		t.Fatalf("created = %#v, stored link = %q", created, store.createdLink)
	}

	store.createdLink = ""
	existing, err := service.Generate(context.Background(), "owner", "app-1")
	if err != nil || existing.ID != created.ID || store.createdLink != "" {
		t.Fatalf("existing = %#v, created link = %q, error = %v", existing, store.createdLink, err)
	}

	store.ownerID = "someone-else"
	_, err = service.Generate(context.Background(), "owner", "app-1")
	assertShareError(t, err, ErrorForbidden)
}

func TestShareManagementValidatesSettingsAndDelegatesOwnerOperations(t *testing.T) {
	share := sampleShare()
	store := &fakeShareStore{ownerID: "owner", share: share}
	service := newShareService(t, store)

	got, err := service.Get(context.Background(), "owner", share.ApplicationID)
	if err != nil || got.ID != share.ID {
		t.Fatalf("Get() = %#v, %v", got, err)
	}

	showHeader := false
	updated, err := service.Update(context.Background(), "owner", share.ApplicationID, UpdateInput{
		SetIsPublic:    true,
		IsPublic:       false,
		SetEmbedConfig: true,
		EmbedConfig: &EmbedConfig{
			AllowedOrigins: []string{"https://example.com"}, Theme: ThemeDark, Width: "100%", Height: "600px", ShowHeader: &showHeader,
		},
	})
	if err != nil || updated.IsPublic || !store.updated.SetEmbedConfig || store.updated.EmbedConfig.Theme != ThemeDark {
		t.Fatalf("Update() = %#v, stored = %#v, error = %v", updated, store.updated, err)
	}

	badInputs := []UpdateInput{
		{},
		{SetEmbedConfig: true, EmbedConfig: &EmbedConfig{Theme: Theme("neon")}},
		{SetEmbedConfig: true, EmbedConfig: &EmbedConfig{AllowedOrigins: []string{"javascript:alert(1)"}}},
		{SetEmbedConfig: true, EmbedConfig: &EmbedConfig{AllowedOrigins: []string{"https://example.com/path"}}},
	}
	for _, input := range badInputs {
		_, err := service.Update(context.Background(), "owner", share.ApplicationID, input)
		assertShareError(t, err, ErrorInvalidInput)
	}

	if err := service.Revoke(context.Background(), "owner", share.ApplicationID); err != nil || store.deletedAppID != share.ApplicationID {
		t.Fatalf("Revoke() deleted = %q, error = %v", store.deletedAppID, err)
	}
}

func TestPublicShareValidatesIdentifierAndIncrementsAccess(t *testing.T) {
	share := sampleShare()
	store := &fakeShareStore{public: PublicApplication{
		ID: share.ID, ApplicationID: share.ApplicationID, ShareLink: share.ShareLink, IsPublic: true, Name: "Demo", Status: "published",
	}}
	service := newShareService(t, store)

	application, err := service.GetPublic(context.Background(), share.ShareLink)
	if err != nil || application.Name != "Demo" || store.incremented != share.ID {
		t.Fatalf("application = %#v, incremented = %q, error = %v", application, store.incremented, err)
	}

	_, err = service.GetPublic(context.Background(), "share-invalid")
	assertShareError(t, err, ErrorNotFound)

	store.public = PublicApplication{}
	_, err = service.GetPublic(context.Background(), share.ShareLink)
	assertShareError(t, err, ErrorNotFound)
}

func TestEmbedCodeUsesServerBaseURLValidatedIdentifierAndTheme(t *testing.T) {
	share := sampleShare()
	share.EmbedConfig = &EmbedConfig{Theme: ThemeAuto}
	store := &fakeShareStore{ownerID: "owner", share: share}
	service := newShareService(t, store)

	embed, err := service.Embed(context.Background(), "owner", share.ApplicationID)
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if embed.ScriptTag != embed.ScriptCode || !strings.Contains(embed.ShareURL, "/share/"+share.ShareLink) {
		t.Fatalf("embed = %#v", embed)
	}
	if !strings.Contains(embed.IframeCode, `src="http://127.0.0.1:5173/share/`+share.ShareLink+`"`) ||
		!strings.Contains(embed.ScriptTag, `data-theme="auto"`) {
		t.Fatalf("unsafe or incompatible embed = %#v", embed)
	}

	store.share.ShareLink = `share-"><script>`
	_, err = service.Embed(context.Background(), "owner", share.ApplicationID)
	assertShareError(t, err, ErrorInternal)
}

func TestShareStoreErrorsMapToStableDomainErrors(t *testing.T) {
	store := &fakeShareStore{ownerID: "owner", err: ErrApplicationNotFound}
	service := newShareService(t, store)
	_, err := service.Get(context.Background(), "owner", "app")
	assertShareError(t, err, ErrorNotFound)

	store.err = errors.New("database unavailable")
	_, err = service.Get(context.Background(), "owner", "app")
	assertShareError(t, err, ErrorInternal)
}

func assertShareError(t *testing.T, err error, kind ErrorKind) *ServiceError {
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
