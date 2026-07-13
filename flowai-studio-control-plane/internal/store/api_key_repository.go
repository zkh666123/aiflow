package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/apikeys"
	controlstore "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type apiKeyQueries interface {
	CreateAPIKey(context.Context, controlstore.CreateAPIKeyParams) (controlstore.ControlApiKey, error)
	ListAPIKeys(context.Context, controlstore.ListAPIKeysParams) ([]controlstore.ControlApiKey, error)
	GetAPIKeyByDigest(context.Context, []byte) (controlstore.ControlApiKey, error)
	DeleteAPIKey(context.Context, controlstore.DeleteAPIKeyParams) (pgtype.UUID, error)
	SetAPIKeyActive(context.Context, controlstore.SetAPIKeyActiveParams) (controlstore.ControlApiKey, error)
	TouchAPIKey(context.Context, pgtype.UUID) error
	GetApplicationOwnerID(context.Context, pgtype.UUID) (pgtype.UUID, error)
}

type APIKeyRepository struct {
	queries apiKeyQueries
}

func NewAPIKeyRepository(queries apiKeyQueries) *APIKeyRepository {
	return &APIKeyRepository{queries: queries}
}

func (repository *APIKeyRepository) CreateAPIKey(ctx context.Context, record apikeys.Record) (apikeys.Record, error) {
	userID, err := parseUUID(record.UserID)
	if err != nil {
		return apikeys.Record{}, fmt.Errorf("invalid API key user: %w", err)
	}
	applicationID, err := nullableUUID(record.ApplicationID)
	if err != nil {
		return apikeys.Record{}, fmt.Errorf("invalid API key application: %w", err)
	}
	scopes, err := json.Marshal(record.Scopes)
	if err != nil {
		return apikeys.Record{}, fmt.Errorf("encode API key scopes: %w", err)
	}
	row, err := repository.queries.CreateAPIKey(ctx, controlstore.CreateAPIKeyParams{
		Name:          record.Name,
		KeyDigest:     append([]byte(nil), record.Digest...),
		KeyPrefix:     record.KeyPrefix,
		Scopes:        scopes,
		ExpiresAt:     nullableTimestamp(record.ExpiresAt),
		UserID:        userID,
		ApplicationID: applicationID,
	})
	if err != nil {
		return apikeys.Record{}, mapAPIKeyError(err)
	}
	return apiKeyRecord(row)
}

func (repository *APIKeyRepository) ListAPIKeys(
	ctx context.Context,
	userValue string,
	applicationValue *string,
) ([]apikeys.Record, error) {
	userID, err := parseUUID(userValue)
	if err != nil {
		return nil, fmt.Errorf("invalid API key user: %w", err)
	}
	applicationID, err := nullableUUID(applicationValue)
	if err != nil {
		return nil, fmt.Errorf("invalid API key application: %w", err)
	}
	rows, err := repository.queries.ListAPIKeys(ctx, controlstore.ListAPIKeysParams{
		UserID:        userID,
		ApplicationID: applicationID,
	})
	if err != nil {
		return nil, err
	}
	records := make([]apikeys.Record, 0, len(rows))
	for _, row := range rows {
		record, err := apiKeyRecord(row)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func (repository *APIKeyRepository) GetAPIKeyByDigest(ctx context.Context, digest []byte) (apikeys.Record, error) {
	row, err := repository.queries.GetAPIKeyByDigest(ctx, digest)
	if err != nil {
		return apikeys.Record{}, mapAPIKeyError(err)
	}
	return apiKeyRecord(row)
}

func (repository *APIKeyRepository) DeleteAPIKey(ctx context.Context, userValue, keyValue string) error {
	params, err := ownerScopedAPIKeyParams(userValue, keyValue)
	if err != nil {
		return apikeys.ErrAPIKeyNotFound
	}
	if _, err := repository.queries.DeleteAPIKey(ctx, controlstore.DeleteAPIKeyParams{
		ID: params.keyID, UserID: params.userID,
	}); err != nil {
		return mapAPIKeyError(err)
	}
	return nil
}

func (repository *APIKeyRepository) SetAPIKeyActive(
	ctx context.Context,
	userValue, keyValue string,
	active bool,
) (apikeys.Record, error) {
	params, err := ownerScopedAPIKeyParams(userValue, keyValue)
	if err != nil {
		return apikeys.Record{}, apikeys.ErrAPIKeyNotFound
	}
	row, err := repository.queries.SetAPIKeyActive(ctx, controlstore.SetAPIKeyActiveParams{
		ID: params.keyID, UserID: params.userID, IsActive: active,
	})
	if err != nil {
		return apikeys.Record{}, mapAPIKeyError(err)
	}
	return apiKeyRecord(row)
}

func (repository *APIKeyRepository) TouchAPIKey(ctx context.Context, keyValue string) error {
	keyID, err := parseUUID(keyValue)
	if err != nil {
		return apikeys.ErrAPIKeyNotFound
	}
	return repository.queries.TouchAPIKey(ctx, keyID)
}

func (repository *APIKeyRepository) ApplicationOwnerID(ctx context.Context, applicationValue string) (string, error) {
	applicationID, err := parseUUID(applicationValue)
	if err != nil {
		return "", apikeys.ErrApplicationNotFound
	}
	ownerID, err := repository.queries.GetApplicationOwnerID(ctx, applicationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apikeys.ErrApplicationNotFound
	}
	if err != nil {
		return "", err
	}
	return uuidString(ownerID)
}

type ownerScopedParams struct {
	userID pgtype.UUID
	keyID  pgtype.UUID
}

func ownerScopedAPIKeyParams(userValue, keyValue string) (ownerScopedParams, error) {
	userID, err := parseUUID(userValue)
	if err != nil {
		return ownerScopedParams{}, err
	}
	keyID, err := parseUUID(keyValue)
	if err != nil {
		return ownerScopedParams{}, err
	}
	return ownerScopedParams{userID: userID, keyID: keyID}, nil
}

func apiKeyRecord(row controlstore.ControlApiKey) (apikeys.Record, error) {
	id, err := uuidString(row.ID)
	if err != nil {
		return apikeys.Record{}, fmt.Errorf("decode API key ID: %w", err)
	}
	userID, err := uuidString(row.UserID)
	if err != nil {
		return apikeys.Record{}, fmt.Errorf("decode API key user: %w", err)
	}
	applicationID, err := uuidPointer(row.ApplicationID)
	if err != nil {
		return apikeys.Record{}, fmt.Errorf("decode API key application: %w", err)
	}
	var scopes []apikeys.Scope
	if err := json.Unmarshal(row.Scopes, &scopes); err != nil {
		return apikeys.Record{}, fmt.Errorf("decode API key scopes: %w", err)
	}
	return apikeys.Record{
		APIKey: apikeys.APIKey{
			ID:            id,
			Name:          row.Name,
			KeyPrefix:     row.KeyPrefix,
			Scopes:        scopes,
			IsActive:      row.IsActive,
			LastUsedAt:    timestampPointer(row.LastUsedAt),
			ExpiresAt:     timestampPointer(row.ExpiresAt),
			UserID:        userID,
			ApplicationID: applicationID,
			CreatedAt:     row.CreatedAt.Time,
		},
		Digest: append([]byte(nil), row.KeyDigest...),
	}, nil
}

func mapAPIKeyError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apikeys.ErrAPIKeyNotFound
	}
	return err
}

func nullableUUID(value *string) (pgtype.UUID, error) {
	if value == nil {
		return pgtype.UUID{}, nil
	}
	return parseUUID(*value)
}

func uuidPointer(value pgtype.UUID) (*string, error) {
	if !value.Valid {
		return nil, nil
	}
	decoded, err := uuidString(value)
	if err != nil {
		return nil, err
	}
	return &decoded, nil
}

func nullableTimestamp(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func timestampPointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}
