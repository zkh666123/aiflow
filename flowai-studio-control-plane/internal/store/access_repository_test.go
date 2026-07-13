package store

import (
	"context"
	"errors"
	"testing"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/rbac"
	controlstore "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeAccessQueries struct {
	globalRole      string
	applicationRows []controlstore.ListApplicationAccessForUserRow
	team            controlstore.ControlTeam
	membership      controlstore.ControlTeamMember
	globalErr       error
	applicationErr  error
	teamErr         error
	membershipErr   error
	applicationArgs controlstore.ListApplicationAccessForUserParams
	teamMemberArgs  controlstore.GetTeamMembershipParams
}

func (queries *fakeAccessQueries) GetUserGlobalRole(context.Context, pgtype.UUID) (string, error) {
	return queries.globalRole, queries.globalErr
}

func (queries *fakeAccessQueries) ListApplicationAccessForUser(
	_ context.Context,
	params controlstore.ListApplicationAccessForUserParams,
) ([]controlstore.ListApplicationAccessForUserRow, error) {
	queries.applicationArgs = params
	return queries.applicationRows, queries.applicationErr
}

func (queries *fakeAccessQueries) GetTeamByID(context.Context, pgtype.UUID) (controlstore.ControlTeam, error) {
	return queries.team, queries.teamErr
}

func (queries *fakeAccessQueries) GetTeamMembership(
	_ context.Context,
	params controlstore.GetTeamMembershipParams,
) (controlstore.ControlTeamMember, error) {
	queries.teamMemberArgs = params
	return queries.membership, queries.membershipErr
}

func TestAccessRepositoryBuildsApplicationAccessFromRelevantRows(t *testing.T) {
	ownerID := mustDatabaseUUID(t, "4dc3923c-331f-4a80-9ea6-cf392e991821")
	queries := &fakeAccessQueries{applicationRows: []controlstore.ListApplicationAccessForUserRow{
		{
			OwnerID:                   ownerID,
			GlobalRole:                "member",
			TeamRole:                  pgtype.Text{String: "viewer", Valid: true},
			TeamApplicationPermission: pgtype.Text{String: "can_edit", Valid: true},
		},
		{
			OwnerID:                   ownerID,
			GlobalRole:                "member",
			TeamRole:                  pgtype.Text{String: "editor", Valid: true},
			TeamApplicationPermission: pgtype.Text{String: "can_view", Valid: true},
		},
	}}
	repository := NewAccessRepository(queries)

	access, err := repository.ApplicationAccess(
		context.Background(),
		"e9f6332d-da39-44b2-917c-da5ff30aca9d",
		"7a611d9a-b555-4469-a289-f1672daefce3",
	)
	if err != nil {
		t.Fatalf("ApplicationAccess() error = %v", err)
	}
	if access.OwnerID != "4dc3923c-331f-4a80-9ea6-cf392e991821" || access.GlobalRole != rbac.GlobalRoleMember {
		t.Fatalf("access = %#v", access)
	}
	if len(access.Grants) != 2 || access.Grants[0].Permission != rbac.TeamAppCanEdit || access.Grants[1].Role != rbac.TeamRoleEditor {
		t.Fatalf("grants = %#v", access.Grants)
	}
	if !queries.applicationArgs.ApplicationID.Valid || !queries.applicationArgs.UserID.Valid {
		t.Fatalf("params = %#v", queries.applicationArgs)
	}
}

func TestAccessRepositoryDistinguishesMissingApplication(t *testing.T) {
	repository := NewAccessRepository(&fakeAccessQueries{})
	_, err := repository.ApplicationAccess(
		context.Background(),
		"e9f6332d-da39-44b2-917c-da5ff30aca9d",
		"7a611d9a-b555-4469-a289-f1672daefce3",
	)
	if !errors.Is(err, rbac.ErrResourceNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestAccessRepositoryBuildsTeamAccessAndAllowsMissingMembership(t *testing.T) {
	teamID := mustDatabaseUUID(t, "0b44ed01-327e-412a-a79e-63ef34d981fe")
	ownerID := mustDatabaseUUID(t, "4dc3923c-331f-4a80-9ea6-cf392e991821")
	userID := mustDatabaseUUID(t, "e9f6332d-da39-44b2-917c-da5ff30aca9d")
	queries := &fakeAccessQueries{
		globalRole: "member",
		team:       controlstore.ControlTeam{ID: teamID, OwnerID: ownerID},
		membership: controlstore.ControlTeamMember{TeamID: teamID, UserID: userID, Role: "viewer"},
	}
	repository := NewAccessRepository(queries)

	access, err := repository.TeamAccess(
		context.Background(),
		"e9f6332d-da39-44b2-917c-da5ff30aca9d",
		"0b44ed01-327e-412a-a79e-63ef34d981fe",
	)
	if err != nil || !access.Member || access.Role != rbac.TeamRoleViewer || access.OwnerID != "4dc3923c-331f-4a80-9ea6-cf392e991821" {
		t.Fatalf("access = %#v, error = %v", access, err)
	}

	queries.membershipErr = pgx.ErrNoRows
	access, err = repository.TeamAccess(
		context.Background(),
		"e9f6332d-da39-44b2-917c-da5ff30aca9d",
		"0b44ed01-327e-412a-a79e-63ef34d981fe",
	)
	if err != nil || access.Member {
		t.Fatalf("non-member access = %#v, error = %v", access, err)
	}
}

func TestAccessRepositoryMapsMissingTeam(t *testing.T) {
	repository := NewAccessRepository(&fakeAccessQueries{globalRole: "member", teamErr: pgx.ErrNoRows})
	_, err := repository.TeamAccess(
		context.Background(),
		"e9f6332d-da39-44b2-917c-da5ff30aca9d",
		"0b44ed01-327e-412a-a79e-63ef34d981fe",
	)
	if !errors.Is(err, rbac.ErrResourceNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func mustDatabaseUUID(t *testing.T, value string) pgtype.UUID {
	t.Helper()
	var result pgtype.UUID
	if err := result.Scan(value); err != nil {
		t.Fatal(err)
	}
	return result
}
