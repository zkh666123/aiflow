package store

import (
	"errors"
	"testing"
	"time"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/rbac"
	controlstore "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/store/sqlc"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/teams"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestTeamRepositoryConvertsListAndDetailRows(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	teamRow := controlstore.ListTeamsForUserRow{
		ID:          mustDatabaseUUID(t, "0b44ed01-327e-412a-a79e-63ef34d981fe"),
		Name:        "Team",
		Description: pgtype.Text{String: "description", Valid: true},
		OwnerID:     mustDatabaseUUID(t, "e9f6332d-da39-44b2-917c-da5ff30aca9d"),
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		MyRole:      "owner",
		MemberCount: 2,
		AppCount:    1,
	}
	team, err := teamFromListRow(teamRow)
	if err != nil || team.ID != "0b44ed01-327e-412a-a79e-63ef34d981fe" || team.MyRole != rbac.TeamRoleOwner || team.MemberCount != 2 || team.AppCount != 1 {
		t.Fatalf("team = %#v, error = %v", team, err)
	}

	members, err := membersFromRows([]controlstore.ListTeamMembersRow{{
		ID:            mustDatabaseUUID(t, "83a55a85-0c52-42f8-ac67-d6be4ba41be1"),
		TeamID:        teamRow.ID,
		UserID:        mustDatabaseUUID(t, "4dc3923c-331f-4a80-9ea6-cf392e991821"),
		Role:          "viewer",
		JoinedAt:      pgtype.Timestamptz{Time: now, Valid: true},
		Username:      "bob",
		Avatar:        pgtype.Text{String: "avatar", Valid: true},
		UserCreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}})
	if err != nil || len(members) != 1 || members[0].User == nil || members[0].User.Username != "bob" || members[0].Role != rbac.TeamRoleViewer {
		t.Fatalf("members = %#v, error = %v", members, err)
	}

	applications, err := teamApplicationsFromRows([]controlstore.ListTeamApplicationsRow{{
		ID:            mustDatabaseUUID(t, "f32384a3-22b1-4913-ab40-e68a980dafd9"),
		TeamID:        teamRow.ID,
		ApplicationID: mustDatabaseUUID(t, "7a611d9a-b555-4469-a289-f1672daefce3"),
		Permission:    "can_edit",
		AddedAt:       pgtype.Timestamptz{Time: now, Valid: true},
		Name:          "App",
		Description:   pgtype.Text{String: "app description", Valid: true},
		Status:        "draft",
	}})
	if err != nil || len(applications) != 1 || applications[0].Application == nil || applications[0].Application.Name != "App" || applications[0].Permission != rbac.TeamAppCanEdit {
		t.Fatalf("applications = %#v, error = %v", applications, err)
	}
}

func TestTeamRepositoryMapsMissingConflictAndForeignKeyErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "missing team", err: pgx.ErrNoRows, want: teams.ErrTeamNotFound},
		{name: "duplicate", err: &pgconn.PgError{Code: "23505", ConstraintName: "team_members_team_id_user_id_key"}, want: teams.ErrTeamConflict},
		{name: "missing user", err: &pgconn.PgError{Code: "23503", ConstraintName: "team_members_user_id_fkey"}, want: teams.ErrUserNotFound},
		{name: "missing application", err: &pgconn.PgError{Code: "23503", ConstraintName: "team_applications_application_id_fkey"}, want: teams.ErrApplicationNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := mapTeamError(test.err, teams.ErrTeamNotFound)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}
