package store

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/apikeys"
	controlstore "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeAPIKeyQueries struct {
	row         controlstore.ControlApiKey
	rows        []controlstore.ControlApiKey
	err         error
	create      controlstore.CreateAPIKeyParams
	list        controlstore.ListAPIKeysParams
	deleted     controlstore.DeleteAPIKeyParams
	toggled     controlstore.SetAPIKeyActiveParams
	touched     pgtype.UUID
	ownerLookup pgtype.UUID
}

func (queries *fakeAPIKeyQueries) CreateAPIKey(_ context.Context, params controlstore.CreateAPIKeyParams) (controlstore.ControlApiKey, error) {
	queries.create = params
	return queries.row, queries.err
}

func (queries *fakeAPIKeyQueries) ListAPIKeys(_ context.Context, params controlstore.ListAPIKeysParams) ([]controlstore.ControlApiKey, error) {
	queries.list = params
	return queries.rows, queries.err
}

func (queries *fakeAPIKeyQueries) GetAPIKeyByDigest(context.Context, []byte) (controlstore.ControlApiKey, error) {
	return queries.row, queries.err
}

func (queries *fakeAPIKeyQueries) DeleteAPIKey(_ context.Context, params controlstore.DeleteAPIKeyParams) (pgtype.UUID, error) {
	queries.deleted = params
	return params.ID, queries.err
}

func (queries *fakeAPIKeyQueries) SetAPIKeyActive(_ context.Context, params controlstore.SetAPIKeyActiveParams) (controlstore.ControlApiKey, error) {
	queries.toggled = params
	return queries.row, queries.err
}

func (queries *fakeAPIKeyQueries) TouchAPIKey(_ context.Context, id pgtype.UUID) error {
	queries.touched = id
	return queries.err
}

func (queries *fakeAPIKeyQueries) GetApplicationOwnerID(_ context.Context, id pgtype.UUID) (pgtype.UUID, error) {
	queries.ownerLookup = id
	return queries.row.UserID, queries.err
}

func TestAPIKeyRepositoryUsesJSONBAndConvertsRecords(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	row := sampleDatabaseAPIKey(t, now)
	queries := &fakeAPIKeyQueries{row: row, rows: []controlstore.ControlApiKey{row}}
	repository := NewAPIKeyRepository(queries)
	expiresAt := now.Add(time.Hour)
	applicationID := "7a611d9a-b555-4469-a289-f1672daefce3"

	created, err := repository.CreateAPIKey(context.Background(), apikeys.Record{
		APIKey: apikeys.APIKey{
			Name: "deploy", KeyPrefix: "sk-0000", Scopes: []apikeys.Scope{apikeys.ScopeAppRead, apikeys.ScopeWorkflowExecute},
			ExpiresAt: &expiresAt, UserID: "e9f6332d-da39-44b2-917c-da5ff30aca9d", ApplicationID: &applicationID,
		},
		Digest: []byte("hmac digest"),
	})
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if string(queries.create.Scopes) != `["app:read","workflow:execute"]` || !bytes.Equal(queries.create.KeyDigest, []byte("hmac digest")) {
		t.Fatalf("create params = %#v", queries.create)
	}
	if created.ID != "4ab1ea39-b2a4-4850-b719-ae5ad57773f1" || !bytes.Equal(created.Digest, row.KeyDigest) {
		t.Fatalf("created = %#v", created)
	}

	listed, err := repository.ListAPIKeys(context.Background(), rowUserID(t), &applicationID)
	if err != nil || len(listed) != 1 || listed[0].Scopes[1] != apikeys.ScopeWorkflowExecute || listed[0].ApplicationID == nil {
		t.Fatalf("listed = %#v, error = %v", listed, err)
	}
	if !queries.list.ApplicationID.Valid {
		t.Fatalf("list params = %#v", queries.list)
	}
}

func TestAPIKeyRepositoryKeepsOwnerScopeAndMapsMissingRows(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	queries := &fakeAPIKeyQueries{row: sampleDatabaseAPIKey(t, now)}
	repository := NewAPIKeyRepository(queries)
	userID := rowUserID(t)
	keyID := "4ab1ea39-b2a4-4850-b719-ae5ad57773f1"

	if err := repository.DeleteAPIKey(context.Background(), userID, keyID); err != nil {
		t.Fatalf("DeleteAPIKey() error = %v", err)
	}
	if _, err := repository.SetAPIKeyActive(context.Background(), userID, keyID, false); err != nil {
		t.Fatalf("SetAPIKeyActive() error = %v", err)
	}
	if queries.deleted.UserID != queries.toggled.UserID || queries.deleted.ID != queries.toggled.ID || queries.toggled.IsActive {
		t.Fatalf("delete = %#v, toggle = %#v", queries.deleted, queries.toggled)
	}

	queries.err = pgx.ErrNoRows
	if err := repository.DeleteAPIKey(context.Background(), userID, keyID); !errors.Is(err, apikeys.ErrAPIKeyNotFound) {
		t.Fatalf("missing delete error = %v", err)
	}
	if _, err := repository.SetAPIKeyActive(context.Background(), userID, "not-a-uuid", true); !errors.Is(err, apikeys.ErrAPIKeyNotFound) {
		t.Fatalf("invalid toggle error = %v", err)
	}
}

func sampleDatabaseAPIKey(t *testing.T, now time.Time) controlstore.ControlApiKey {
	t.Helper()
	return controlstore.ControlApiKey{
		ID:            mustDatabaseUUID(t, "4ab1ea39-b2a4-4850-b719-ae5ad57773f1"),
		Name:          "deploy",
		KeyDigest:     []byte("hmac digest"),
		KeyPrefix:     "sk-0000",
		Scopes:        []byte(`["app:read","workflow:execute"]`),
		IsActive:      true,
		UserID:        mustDatabaseUUID(t, "e9f6332d-da39-44b2-917c-da5ff30aca9d"),
		ApplicationID: mustDatabaseUUID(t, "7a611d9a-b555-4469-a289-f1672daefce3"),
		CreatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
	}
}

func rowUserID(t *testing.T) string {
	t.Helper()
	return "e9f6332d-da39-44b2-917c-da5ff30aca9d"
}
