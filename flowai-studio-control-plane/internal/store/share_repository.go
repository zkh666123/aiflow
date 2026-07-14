package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/shares"
	controlstore "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ShareRepository struct {
	pool    *pgxpool.Pool
	queries *controlstore.Queries
}

func NewShareRepository(pool *pgxpool.Pool) *ShareRepository {
	return &ShareRepository{pool: pool, queries: controlstore.New(pool)}
}

func (repository *ShareRepository) ApplicationOwnerID(ctx context.Context, applicationValue string) (string, error) {
	applicationID, err := parseUUID(applicationValue)
	if err != nil {
		return "", shares.ErrApplicationNotFound
	}
	ownerID, err := repository.queries.GetApplicationOwnerID(ctx, applicationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", shares.ErrApplicationNotFound
	}
	if err != nil {
		return "", err
	}
	return uuidString(ownerID)
}

func (repository *ShareRepository) GetShareByApplicationID(ctx context.Context, applicationValue string) (shares.Share, error) {
	applicationID, err := parseUUID(applicationValue)
	if err != nil {
		return shares.Share{}, shares.ErrShareNotFound
	}
	row, err := repository.queries.GetAppShareByApplicationID(ctx, applicationID)
	if err != nil {
		return shares.Share{}, mapShareError(err)
	}
	return shareRecord(row)
}

func (repository *ShareRepository) CreateShare(ctx context.Context, applicationValue, shareLink string) (shares.Share, error) {
	applicationID, err := parseUUID(applicationValue)
	if err != nil {
		return shares.Share{}, shares.ErrApplicationNotFound
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return shares.Share{}, fmt.Errorf("begin share transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := repository.queries.WithTx(tx)
	row, err := queries.CreateAppShare(ctx, controlstore.CreateAppShareParams{
		ApplicationID: applicationID,
		ShareLink:     shareLink,
		IsPublic:      true,
	})
	if err != nil {
		return shares.Share{}, mapShareError(err)
	}
	if err := queries.SetApplicationShareLink(ctx, controlstore.SetApplicationShareLinkParams{
		ID: applicationID, ShareLink: pgtype.Text{String: shareLink, Valid: true},
	}); err != nil {
		return shares.Share{}, err
	}
	share, err := shareRecord(row)
	if err != nil {
		return shares.Share{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return shares.Share{}, fmt.Errorf("commit share transaction: %w", err)
	}
	return share, nil
}

func (repository *ShareRepository) UpdateShare(
	ctx context.Context,
	applicationValue string,
	input shares.UpdateInput,
) (shares.Share, error) {
	applicationID, err := parseUUID(applicationValue)
	if err != nil {
		return shares.Share{}, shares.ErrShareNotFound
	}
	var embedConfig []byte
	if input.SetEmbedConfig {
		embedConfig, err = json.Marshal(input.EmbedConfig)
		if err != nil {
			return shares.Share{}, fmt.Errorf("encode share embed config: %w", err)
		}
	}
	row, err := repository.queries.UpdateAppShare(ctx, controlstore.UpdateAppShareParams{
		SetIsPublic:    input.SetIsPublic,
		IsPublic:       input.IsPublic,
		SetEmbedConfig: input.SetEmbedConfig,
		EmbedConfig:    embedConfig,
		ApplicationID:  applicationID,
	})
	if err != nil {
		return shares.Share{}, mapShareError(err)
	}
	return shareRecord(row)
}

func (repository *ShareRepository) DeleteShare(ctx context.Context, applicationValue string) error {
	applicationID, err := parseUUID(applicationValue)
	if err != nil {
		return nil
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin share revocation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := repository.queries.WithTx(tx)
	if _, err := queries.DeleteAppShare(ctx, applicationID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err := queries.SetApplicationShareLink(ctx, controlstore.SetApplicationShareLinkParams{
		ID: applicationID, ShareLink: pgtype.Text{},
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit share revocation: %w", err)
	}
	return nil
}

func (repository *ShareRepository) GetPublicShareByLink(ctx context.Context, shareLink string) (shares.PublicApplication, error) {
	row, err := repository.queries.GetPublicAppShareByLink(ctx, shareLink)
	if err != nil {
		return shares.PublicApplication{}, mapShareError(err)
	}
	return publicShareRecord(row)
}

func (repository *ShareRepository) IncrementShareAccess(ctx context.Context, shareValue string) error {
	shareID, err := parseUUID(shareValue)
	if err != nil {
		return shares.ErrShareNotFound
	}
	return repository.queries.IncrementAppShareAccess(ctx, shareID)
}

func shareRecord(row controlstore.ControlAppShare) (shares.Share, error) {
	id, err := uuidString(row.ID)
	if err != nil {
		return shares.Share{}, fmt.Errorf("decode share ID: %w", err)
	}
	applicationID, err := uuidString(row.ApplicationID)
	if err != nil {
		return shares.Share{}, fmt.Errorf("decode share application: %w", err)
	}
	embedConfig, err := decodeEmbedConfig(row.EmbedConfig)
	if err != nil {
		return shares.Share{}, err
	}
	return shares.Share{
		ID: id, ApplicationID: applicationID, ShareLink: row.ShareLink, IsPublic: row.IsPublic,
		AccessCount: row.AccessCount, EmbedConfig: embedConfig, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func publicShareRecord(row controlstore.GetPublicAppShareByLinkRow) (shares.PublicApplication, error) {
	shareID, err := uuidString(row.ID)
	if err != nil {
		return shares.PublicApplication{}, fmt.Errorf("decode public share ID: %w", err)
	}
	applicationID, err := uuidString(row.ApplicationID)
	if err != nil {
		return shares.PublicApplication{}, fmt.Errorf("decode public share application: %w", err)
	}
	embedConfig, err := decodeEmbedConfig(row.EmbedConfig)
	if err != nil {
		return shares.PublicApplication{}, err
	}
	return shares.PublicApplication{
		ID: shareID, ApplicationID: applicationID, ShareLink: row.ShareLink, IsPublic: row.IsPublic,
		Name: row.Name, Description: textPointer(row.Description), Icon: textPointer(row.Icon), Status: row.Status,
		EmbedConfig: embedConfig,
	}, nil
}

func decodeEmbedConfig(value []byte) (*shares.EmbedConfig, error) {
	if len(value) == 0 || string(value) == "null" {
		return nil, nil
	}
	var config shares.EmbedConfig
	if err := json.Unmarshal(value, &config); err != nil {
		return nil, fmt.Errorf("decode share embed config: %w", err)
	}
	return &config, nil
}

func mapShareError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return shares.ErrShareNotFound
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23503" {
		return shares.ErrApplicationNotFound
	}
	return err
}
