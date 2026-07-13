package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/applications"
	controlstore "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type applicationQueries interface {
	CreateApplication(context.Context, controlstore.CreateApplicationParams) (controlstore.ControlApplication, error)
	ListApplicationsForUser(context.Context, pgtype.UUID) ([]controlstore.ListApplicationsForUserRow, error)
	GetApplicationByID(context.Context, pgtype.UUID) (controlstore.ControlApplication, error)
	UpdateApplication(context.Context, controlstore.UpdateApplicationParams) (controlstore.ControlApplication, error)
	DeleteApplication(context.Context, pgtype.UUID) (pgtype.UUID, error)
	SetApplicationStatus(context.Context, controlstore.SetApplicationStatusParams) (controlstore.ControlApplication, error)
}

type ApplicationRepository struct {
	queries applicationQueries
}

func NewApplicationRepository(queries applicationQueries) *ApplicationRepository {
	return &ApplicationRepository{queries: queries}
}

func (repository *ApplicationRepository) CreateApplication(
	ctx context.Context,
	userValue string,
	input applications.CreateInput,
) (applications.Application, error) {
	ownerID, err := parseUUID(userValue)
	if err != nil {
		return applications.Application{}, fmt.Errorf("invalid application owner: %w", err)
	}
	row, err := repository.queries.CreateApplication(ctx, controlstore.CreateApplicationParams{
		Name:        input.Name,
		Description: nullableText(input.Description),
		Icon:        nullableText(input.Icon),
		Status:      string(input.Status),
		OwnerID:     ownerID,
	})
	if err != nil {
		return applications.Application{}, mapApplicationError(err)
	}
	return applicationRecord(row)
}

func (repository *ApplicationRepository) ListApplications(
	ctx context.Context,
	userValue string,
) ([]applications.Application, error) {
	userID, err := parseUUID(userValue)
	if err != nil {
		return nil, fmt.Errorf("invalid application user: %w", err)
	}
	rows, err := repository.queries.ListApplicationsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]applications.Application, 0, len(rows))
	for _, row := range rows {
		application, err := applicationFromListRow(row)
		if err != nil {
			return nil, err
		}
		result = append(result, application)
	}
	return result, nil
}

func (repository *ApplicationRepository) GetApplication(
	ctx context.Context,
	value string,
) (applications.Application, error) {
	id, err := parseUUID(value)
	if err != nil {
		return applications.Application{}, applications.ErrApplicationNotFound
	}
	row, err := repository.queries.GetApplicationByID(ctx, id)
	if err != nil {
		return applications.Application{}, mapApplicationError(err)
	}
	return applicationRecord(row)
}

func (repository *ApplicationRepository) UpdateApplication(
	ctx context.Context,
	value string,
	input applications.UpdateInput,
) (applications.Application, error) {
	id, err := parseUUID(value)
	if err != nil {
		return applications.Application{}, applications.ErrApplicationNotFound
	}
	params := controlstore.UpdateApplicationParams{
		ID:             id,
		SetDescription: input.DescriptionSet,
		Description:    nullableText(input.Description),
		SetIcon:        input.IconSet,
		Icon:           nullableText(input.Icon),
	}
	if input.Name != nil {
		params.SetName = true
		params.Name = *input.Name
	}
	if input.Status != nil {
		params.SetStatus = true
		params.Status = string(*input.Status)
	}
	row, err := repository.queries.UpdateApplication(ctx, params)
	if err != nil {
		return applications.Application{}, mapApplicationError(err)
	}
	return applicationRecord(row)
}

func (repository *ApplicationRepository) DeleteApplication(ctx context.Context, value string) error {
	id, err := parseUUID(value)
	if err != nil {
		return applications.ErrApplicationNotFound
	}
	if _, err := repository.queries.DeleteApplication(ctx, id); err != nil {
		return mapApplicationError(err)
	}
	return nil
}

func (repository *ApplicationRepository) SetApplicationStatus(
	ctx context.Context,
	value string,
	status applications.Status,
) (applications.Application, error) {
	id, err := parseUUID(value)
	if err != nil {
		return applications.Application{}, applications.ErrApplicationNotFound
	}
	row, err := repository.queries.SetApplicationStatus(ctx, controlstore.SetApplicationStatusParams{
		ID:     id,
		Status: string(status),
	})
	if err != nil {
		return applications.Application{}, mapApplicationError(err)
	}
	return applicationRecord(row)
}

func applicationRecord(row controlstore.ControlApplication) (applications.Application, error) {
	id, err := uuidString(row.ID)
	if err != nil {
		return applications.Application{}, fmt.Errorf("decode application ID: %w", err)
	}
	ownerID, err := uuidString(row.OwnerID)
	if err != nil {
		return applications.Application{}, fmt.Errorf("decode application owner: %w", err)
	}
	return applications.Application{
		ID:          id,
		Name:        row.Name,
		Description: textPointer(row.Description),
		Icon:        textPointer(row.Icon),
		Status:      applications.Status(row.Status),
		ShareLink:   textPointer(row.ShareLink),
		OwnerID:     ownerID,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func applicationFromListRow(row controlstore.ListApplicationsForUserRow) (applications.Application, error) {
	application, err := applicationRecord(controlstore.ControlApplication{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		Icon:        row.Icon,
		Status:      row.Status,
		ShareLink:   row.ShareLink,
		OwnerID:     row.OwnerID,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	})
	if err != nil {
		return applications.Application{}, err
	}
	accessType, err := databaseText(row.AccessType)
	if err != nil {
		return applications.Application{}, fmt.Errorf("decode application access type: %w", err)
	}
	application.AccessType = accessType
	return application, nil
}

func mapApplicationError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return applications.ErrApplicationNotFound
	}
	return err
}

func nullableText(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}

func textPointer(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func databaseText(value interface{}) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	default:
		return "", fmt.Errorf("unexpected text type %T", value)
	}
}
