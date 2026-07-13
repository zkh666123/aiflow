package store

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/auth"
	controlstore "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type userQueries interface {
	CreateUser(context.Context, controlstore.CreateUserParams) (controlstore.ControlUser, error)
	GetUserByUsername(context.Context, string) (controlstore.ControlUser, error)
	GetUserByID(context.Context, pgtype.UUID) (controlstore.ControlUser, error)
	UpdateUserProfile(context.Context, controlstore.UpdateUserProfileParams) (controlstore.ControlUser, error)
}

type UserRepository struct {
	queries userQueries
}

func NewUserRepository(queries userQueries) *UserRepository {
	return &UserRepository{queries: queries}
}

func (repository *UserRepository) CreateUser(ctx context.Context, username, passwordHash string) (auth.UserRecord, error) {
	row, err := repository.queries.CreateUser(ctx, controlstore.CreateUserParams{
		Username:     username,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return auth.UserRecord{}, mapUserError(err)
	}
	return userRecord(row)
}

func (repository *UserRepository) GetUserByUsername(ctx context.Context, username string) (auth.UserRecord, error) {
	row, err := repository.queries.GetUserByUsername(ctx, username)
	if err != nil {
		return auth.UserRecord{}, mapUserError(err)
	}
	return userRecord(row)
}

func (repository *UserRepository) GetUserByID(ctx context.Context, value string) (auth.UserRecord, error) {
	id, err := parseUUID(value)
	if err != nil {
		return auth.UserRecord{}, auth.ErrUserNotFound
	}
	row, err := repository.queries.GetUserByID(ctx, id)
	if err != nil {
		return auth.UserRecord{}, mapUserError(err)
	}
	return userRecord(row)
}

func (repository *UserRepository) UpdateUserProfile(
	ctx context.Context,
	value string,
	input auth.UpdateProfileInput,
) (auth.UserRecord, error) {
	id, err := parseUUID(value)
	if err != nil {
		return auth.UserRecord{}, auth.ErrUserNotFound
	}
	params := controlstore.UpdateUserProfileParams{
		ID:        id,
		SetAvatar: input.AvatarSet,
	}
	if input.Username != nil {
		params.SetUsername = true
		params.Username = *input.Username
	}
	if input.Avatar != nil {
		params.Avatar = pgtype.Text{String: *input.Avatar, Valid: true}
	}
	row, err := repository.queries.UpdateUserProfile(ctx, params)
	if err != nil {
		return auth.UserRecord{}, mapUserError(err)
	}
	return userRecord(row)
}

func mapUserError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrUserNotFound
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" && strings.Contains(databaseError.ConstraintName, "username") {
		return auth.ErrUsernameExists
	}
	return err
}

func userRecord(row controlstore.ControlUser) (auth.UserRecord, error) {
	id, err := uuidString(row.ID)
	if err != nil {
		return auth.UserRecord{}, err
	}
	var avatar *string
	if row.Avatar.Valid {
		value := row.Avatar.String
		avatar = &value
	}
	return auth.UserRecord{
		ID:           id,
		Username:     row.Username,
		PasswordHash: row.PasswordHash,
		Avatar:       avatar,
		GlobalRole:   row.GlobalRole,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}, nil
}

func parseUUID(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil || !id.Valid {
		return pgtype.UUID{}, fmt.Errorf("invalid UUID")
	}
	return id, nil
}

func uuidString(value pgtype.UUID) (string, error) {
	if !value.Valid {
		return "", errors.New("database UUID is null")
	}
	encoded := hex.EncodeToString(value.Bytes[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}
