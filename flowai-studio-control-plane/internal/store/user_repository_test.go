package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/auth"
	controlstore "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeUserQueries struct {
	user         controlstore.ControlUser
	err          error
	createParams controlstore.CreateUserParams
	updateParams controlstore.UpdateUserProfileParams
}

func (queries *fakeUserQueries) CreateUser(_ context.Context, params controlstore.CreateUserParams) (controlstore.ControlUser, error) {
	queries.createParams = params
	return queries.user, queries.err
}

func (queries *fakeUserQueries) GetUserByUsername(context.Context, string) (controlstore.ControlUser, error) {
	return queries.user, queries.err
}

func (queries *fakeUserQueries) GetUserByID(context.Context, pgtype.UUID) (controlstore.ControlUser, error) {
	return queries.user, queries.err
}

func (queries *fakeUserQueries) UpdateUserProfile(_ context.Context, params controlstore.UpdateUserProfileParams) (controlstore.ControlUser, error) {
	queries.updateParams = params
	return queries.user, queries.err
}

func TestUserRepositoryConvertsGeneratedDatabaseTypes(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	avatar := "https://example.test/avatar.png"
	queries := &fakeUserQueries{user: databaseUser(t, now, &avatar)}
	repository := NewUserRepository(queries)

	created, err := repository.CreateUser(context.Background(), "alice", "bcrypt-hash")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if created.ID != "e9f6332d-da39-44b2-917c-da5ff30aca9d" || created.Avatar == nil || *created.Avatar != avatar {
		t.Fatalf("created = %#v", created)
	}
	if created.CreatedAt != now || queries.createParams.Username != "alice" || queries.createParams.PasswordHash != "bcrypt-hash" {
		t.Fatalf("created = %#v, params = %#v", created, queries.createParams)
	}

	newUsername := "alice_2"
	updated, err := repository.UpdateUserProfile(context.Background(), created.ID, auth.UpdateProfileInput{
		Username:  &newUsername,
		AvatarSet: true,
		Avatar:    nil,
	})
	if err != nil {
		t.Fatalf("UpdateUserProfile() error = %v", err)
	}
	if updated.ID != created.ID || !queries.updateParams.SetUsername || queries.updateParams.Username != newUsername {
		t.Fatalf("updated = %#v, params = %#v", updated, queries.updateParams)
	}
	if !queries.updateParams.SetAvatar || queries.updateParams.Avatar.Valid {
		t.Fatalf("avatar params = %#v", queries.updateParams.Avatar)
	}
}

func TestUserRepositoryMapsDatabaseErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "not found", err: pgx.ErrNoRows, want: auth.ErrUserNotFound},
		{name: "duplicate username", err: &pgconn.PgError{Code: "23505", ConstraintName: "users_username_key"}, want: auth.ErrUsernameExists},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := NewUserRepository(&fakeUserQueries{err: test.err})
			_, err := repository.GetUserByUsername(context.Background(), "alice")
			if test.want == auth.ErrUsernameExists {
				_, err = repository.CreateUser(context.Background(), "alice", "hash")
			}
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func databaseUser(t *testing.T, now time.Time, avatar *string) controlstore.ControlUser {
	t.Helper()
	var id pgtype.UUID
	if err := id.Scan("e9f6332d-da39-44b2-917c-da5ff30aca9d"); err != nil {
		t.Fatal(err)
	}
	databaseAvatar := pgtype.Text{}
	if avatar != nil {
		databaseAvatar = pgtype.Text{String: *avatar, Valid: true}
	}
	return controlstore.ControlUser{
		ID:           id,
		Username:     "alice",
		PasswordHash: "bcrypt-hash",
		Avatar:       databaseAvatar,
		GlobalRole:   "member",
		CreatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
	}
}
