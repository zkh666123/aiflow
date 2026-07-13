package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/rbac"
	controlstore "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/store/sqlc"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/teams"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TeamRepository struct {
	pool    *pgxpool.Pool
	queries *controlstore.Queries
}

func NewTeamRepository(pool *pgxpool.Pool) *TeamRepository {
	return &TeamRepository{pool: pool, queries: controlstore.New(pool)}
}

func (repository *TeamRepository) CreateTeam(
	ctx context.Context,
	userValue string,
	input teams.CreateInput,
) (teams.Team, error) {
	ownerID, err := parseUUID(userValue)
	if err != nil {
		return teams.Team{}, teams.ErrUserNotFound
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return teams.Team{}, fmt.Errorf("begin team transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := repository.queries.WithTx(tx)
	teamRow, err := queries.CreateTeam(ctx, controlstore.CreateTeamParams{
		Name:        input.Name,
		Description: nullableText(input.Description),
		Avatar:      nullableText(input.Avatar),
		OwnerID:     ownerID,
	})
	if err != nil {
		return teams.Team{}, mapTeamError(err, teams.ErrTeamNotFound)
	}
	memberRow, err := queries.CreateTeamMember(ctx, controlstore.CreateTeamMemberParams{
		TeamID: teamRow.ID,
		UserID: ownerID,
		Role:   string(rbac.TeamRoleOwner),
	})
	if err != nil {
		return teams.Team{}, mapTeamError(err, teams.ErrMemberNotFound)
	}
	if err := tx.Commit(ctx); err != nil {
		return teams.Team{}, fmt.Errorf("commit team transaction: %w", err)
	}

	team, err := teamRecord(teamRow)
	if err != nil {
		return teams.Team{}, err
	}
	member, err := memberRecord(memberRow)
	if err != nil {
		return teams.Team{}, err
	}
	team.MyRole = rbac.TeamRoleOwner
	team.MemberCount = 1
	team.Members = []teams.Member{member}
	team.Applications = []teams.TeamApplication{}
	return team, nil
}

func (repository *TeamRepository) ListTeams(ctx context.Context, userValue string) ([]teams.Team, error) {
	userID, err := parseUUID(userValue)
	if err != nil {
		return nil, teams.ErrUserNotFound
	}
	rows, err := repository.queries.ListTeamsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]teams.Team, 0, len(rows))
	for _, row := range rows {
		team, err := teamFromListRow(row)
		if err != nil {
			return nil, err
		}
		result = append(result, team)
	}
	return result, nil
}

func (repository *TeamRepository) GetTeam(
	ctx context.Context,
	teamValue string,
	userValue string,
) (teams.Team, error) {
	teamID, err := parseUUID(teamValue)
	if err != nil {
		return teams.Team{}, teams.ErrTeamNotFound
	}
	userID, err := parseUUID(userValue)
	if err != nil {
		return teams.Team{}, teams.ErrUserNotFound
	}
	row, err := repository.queries.GetTeamByID(ctx, teamID)
	if err != nil {
		return teams.Team{}, mapTeamError(err, teams.ErrTeamNotFound)
	}
	team, err := teamRecord(row)
	if err != nil {
		return teams.Team{}, err
	}
	membership, err := repository.queries.GetTeamMembership(ctx, controlstore.GetTeamMembershipParams{TeamID: teamID, UserID: userID})
	if err == nil {
		team.MyRole = rbac.TeamRole(membership.Role)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return teams.Team{}, err
	}
	members, err := repository.queries.ListTeamMembers(ctx, teamID)
	if err != nil {
		return teams.Team{}, err
	}
	team.Members, err = membersFromRows(members)
	if err != nil {
		return teams.Team{}, err
	}
	applications, err := repository.queries.ListTeamApplications(ctx, teamID)
	if err != nil {
		return teams.Team{}, err
	}
	team.Applications, err = teamApplicationsFromRows(applications)
	if err != nil {
		return teams.Team{}, err
	}
	team.MemberCount = int64(len(team.Members))
	team.AppCount = int64(len(team.Applications))
	return team, nil
}

func (repository *TeamRepository) UpdateTeam(
	ctx context.Context,
	teamValue string,
	input teams.UpdateInput,
) (teams.Team, error) {
	teamID, err := parseUUID(teamValue)
	if err != nil {
		return teams.Team{}, teams.ErrTeamNotFound
	}
	params := controlstore.UpdateTeamParams{
		ID:             teamID,
		SetDescription: input.DescriptionSet,
		Description:    nullableText(input.Description),
		SetAvatar:      input.AvatarSet,
		Avatar:         nullableText(input.Avatar),
	}
	if input.Name != nil {
		params.SetName = true
		params.Name = *input.Name
	}
	row, err := repository.queries.UpdateTeam(ctx, params)
	if err != nil {
		return teams.Team{}, mapTeamError(err, teams.ErrTeamNotFound)
	}
	return teamRecord(row)
}

func (repository *TeamRepository) DeleteTeam(ctx context.Context, teamValue string) error {
	teamID, err := parseUUID(teamValue)
	if err != nil {
		return teams.ErrTeamNotFound
	}
	if _, err := repository.queries.DeleteTeam(ctx, teamID); err != nil {
		return mapTeamError(err, teams.ErrTeamNotFound)
	}
	return nil
}

func (repository *TeamRepository) AddMember(
	ctx context.Context,
	teamValue string,
	input teams.AddMemberInput,
) (teams.Member, error) {
	teamID, err := parseUUID(teamValue)
	if err != nil {
		return teams.Member{}, teams.ErrTeamNotFound
	}
	userID, err := parseUUID(input.UserID)
	if err != nil {
		return teams.Member{}, teams.ErrUserNotFound
	}
	row, err := repository.queries.CreateTeamMember(ctx, controlstore.CreateTeamMemberParams{
		TeamID: teamID,
		UserID: userID,
		Role:   string(input.Role),
	})
	if err != nil {
		return teams.Member{}, mapTeamError(err, teams.ErrMemberNotFound)
	}
	member, err := memberRecord(row)
	if err != nil {
		return teams.Member{}, err
	}
	user, err := repository.queries.GetUserByID(ctx, userID)
	if err != nil {
		return teams.Member{}, mapTeamError(err, teams.ErrUserNotFound)
	}
	member.User = userSummary(user)
	return member, nil
}

func (repository *TeamRepository) GetMember(ctx context.Context, teamValue, memberValue string) (teams.Member, error) {
	teamID, err := parseUUID(teamValue)
	if err != nil {
		return teams.Member{}, teams.ErrTeamNotFound
	}
	memberID, err := parseUUID(memberValue)
	if err != nil {
		return teams.Member{}, teams.ErrMemberNotFound
	}
	row, err := repository.queries.GetTeamMemberByID(ctx, memberID)
	if err != nil {
		return teams.Member{}, mapTeamError(err, teams.ErrMemberNotFound)
	}
	if !uuidEqual(row.TeamID, teamID) {
		return teams.Member{}, teams.ErrMemberNotFound
	}
	return memberRecord(row)
}

func (repository *TeamRepository) UpdateMemberRole(
	ctx context.Context,
	teamValue string,
	memberValue string,
	role rbac.TeamRole,
) (teams.Member, error) {
	teamID, err := parseUUID(teamValue)
	if err != nil {
		return teams.Member{}, teams.ErrTeamNotFound
	}
	memberID, err := parseUUID(memberValue)
	if err != nil {
		return teams.Member{}, teams.ErrMemberNotFound
	}
	row, err := repository.queries.UpdateTeamMemberRole(ctx, controlstore.UpdateTeamMemberRoleParams{
		TeamID: teamID,
		ID:     memberID,
		Role:   string(role),
	})
	if err != nil {
		return teams.Member{}, mapTeamError(err, teams.ErrMemberNotFound)
	}
	member, err := memberRecord(row)
	if err != nil {
		return teams.Member{}, err
	}
	user, err := repository.queries.GetUserByID(ctx, row.UserID)
	if err == nil {
		member.User = userSummary(user)
	}
	return member, nil
}

func (repository *TeamRepository) RemoveMember(ctx context.Context, teamValue, memberValue string) error {
	teamID, err := parseUUID(teamValue)
	if err != nil {
		return teams.ErrTeamNotFound
	}
	memberID, err := parseUUID(memberValue)
	if err != nil {
		return teams.ErrMemberNotFound
	}
	if _, err := repository.queries.DeleteTeamMember(ctx, controlstore.DeleteTeamMemberParams{TeamID: teamID, ID: memberID}); err != nil {
		return mapTeamError(err, teams.ErrMemberNotFound)
	}
	return nil
}

func (repository *TeamRepository) LeaveTeam(ctx context.Context, teamValue, userValue string) error {
	teamID, err := parseUUID(teamValue)
	if err != nil {
		return teams.ErrTeamNotFound
	}
	userID, err := parseUUID(userValue)
	if err != nil {
		return teams.ErrUserNotFound
	}
	_, err = repository.queries.DeleteTeamMembershipByUser(ctx, controlstore.DeleteTeamMembershipByUserParams{TeamID: teamID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func (repository *TeamRepository) ApplicationOwnerID(ctx context.Context, applicationValue string) (string, error) {
	applicationID, err := parseUUID(applicationValue)
	if err != nil {
		return "", teams.ErrApplicationNotFound
	}
	ownerID, err := repository.queries.GetApplicationOwnerID(ctx, applicationID)
	if err != nil {
		return "", mapTeamError(err, teams.ErrApplicationNotFound)
	}
	return uuidString(ownerID)
}

func (repository *TeamRepository) AddApplication(
	ctx context.Context,
	teamValue string,
	input teams.AddApplicationInput,
) (teams.TeamApplication, error) {
	teamID, err := parseUUID(teamValue)
	if err != nil {
		return teams.TeamApplication{}, teams.ErrTeamNotFound
	}
	applicationID, err := parseUUID(input.ApplicationID)
	if err != nil {
		return teams.TeamApplication{}, teams.ErrApplicationNotFound
	}
	row, err := repository.queries.CreateTeamApplication(ctx, controlstore.CreateTeamApplicationParams{
		TeamID:        teamID,
		ApplicationID: applicationID,
		Permission:    string(input.Permission),
	})
	if err != nil {
		return teams.TeamApplication{}, mapTeamError(err, teams.ErrTeamApplicationNotFound)
	}
	result, err := teamApplicationRecord(row)
	if err != nil {
		return teams.TeamApplication{}, err
	}
	application, err := repository.queries.GetApplicationByID(ctx, applicationID)
	if err == nil {
		result.Application = applicationSummary(application)
	}
	return result, nil
}

func (repository *TeamRepository) GetTeamApplication(
	ctx context.Context,
	teamValue string,
	teamApplicationValue string,
) (teams.TeamApplication, error) {
	teamID, err := parseUUID(teamValue)
	if err != nil {
		return teams.TeamApplication{}, teams.ErrTeamNotFound
	}
	teamApplicationID, err := parseUUID(teamApplicationValue)
	if err != nil {
		return teams.TeamApplication{}, teams.ErrTeamApplicationNotFound
	}
	row, err := repository.queries.GetTeamApplicationByID(ctx, teamApplicationID)
	if err != nil {
		return teams.TeamApplication{}, mapTeamError(err, teams.ErrTeamApplicationNotFound)
	}
	if !uuidEqual(row.TeamID, teamID) {
		return teams.TeamApplication{}, teams.ErrTeamApplicationNotFound
	}
	return teamApplicationRecord(row)
}

func (repository *TeamRepository) UpdateApplicationPermission(
	ctx context.Context,
	teamValue string,
	teamApplicationValue string,
	permission rbac.TeamAppPermission,
) (teams.TeamApplication, error) {
	teamID, err := parseUUID(teamValue)
	if err != nil {
		return teams.TeamApplication{}, teams.ErrTeamNotFound
	}
	teamApplicationID, err := parseUUID(teamApplicationValue)
	if err != nil {
		return teams.TeamApplication{}, teams.ErrTeamApplicationNotFound
	}
	row, err := repository.queries.UpdateTeamApplicationPermission(ctx, controlstore.UpdateTeamApplicationPermissionParams{
		TeamID:     teamID,
		ID:         teamApplicationID,
		Permission: string(permission),
	})
	if err != nil {
		return teams.TeamApplication{}, mapTeamError(err, teams.ErrTeamApplicationNotFound)
	}
	return teamApplicationRecord(row)
}

func (repository *TeamRepository) RemoveApplication(ctx context.Context, teamValue, teamApplicationValue string) error {
	teamID, err := parseUUID(teamValue)
	if err != nil {
		return teams.ErrTeamNotFound
	}
	teamApplicationID, err := parseUUID(teamApplicationValue)
	if err != nil {
		return teams.ErrTeamApplicationNotFound
	}
	if _, err := repository.queries.DeleteTeamApplication(ctx, controlstore.DeleteTeamApplicationParams{TeamID: teamID, ID: teamApplicationID}); err != nil {
		return mapTeamError(err, teams.ErrTeamApplicationNotFound)
	}
	return nil
}

func teamRecord(row controlstore.ControlTeam) (teams.Team, error) {
	id, err := uuidString(row.ID)
	if err != nil {
		return teams.Team{}, err
	}
	ownerID, err := uuidString(row.OwnerID)
	if err != nil {
		return teams.Team{}, err
	}
	return teams.Team{
		ID:          id,
		Name:        row.Name,
		Description: textPointer(row.Description),
		Avatar:      textPointer(row.Avatar),
		OwnerID:     ownerID,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func teamFromListRow(row controlstore.ListTeamsForUserRow) (teams.Team, error) {
	team, err := teamRecord(controlstore.ControlTeam{
		ID: row.ID, Name: row.Name, Description: row.Description, Avatar: row.Avatar,
		OwnerID: row.OwnerID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	if err != nil {
		return teams.Team{}, err
	}
	team.MyRole = rbac.TeamRole(row.MyRole)
	team.MemberCount = row.MemberCount
	team.AppCount = row.AppCount
	return team, nil
}

func memberRecord(row controlstore.ControlTeamMember) (teams.Member, error) {
	id, err := uuidString(row.ID)
	if err != nil {
		return teams.Member{}, err
	}
	teamID, err := uuidString(row.TeamID)
	if err != nil {
		return teams.Member{}, err
	}
	userID, err := uuidString(row.UserID)
	if err != nil {
		return teams.Member{}, err
	}
	return teams.Member{ID: id, TeamID: teamID, UserID: userID, Role: rbac.TeamRole(row.Role), JoinedAt: row.JoinedAt.Time}, nil
}

func membersFromRows(rows []controlstore.ListTeamMembersRow) ([]teams.Member, error) {
	result := make([]teams.Member, 0, len(rows))
	for _, row := range rows {
		member, err := memberRecord(controlstore.ControlTeamMember{
			ID: row.ID, TeamID: row.TeamID, UserID: row.UserID, Role: row.Role, JoinedAt: row.JoinedAt,
		})
		if err != nil {
			return nil, err
		}
		userID, err := uuidString(row.UserID)
		if err != nil {
			return nil, err
		}
		member.User = &teams.UserSummary{
			ID: userID, Username: row.Username, Avatar: textPointer(row.Avatar), CreatedAt: row.UserCreatedAt.Time,
		}
		result = append(result, member)
	}
	return result, nil
}

func teamApplicationRecord(row controlstore.ControlTeamApplication) (teams.TeamApplication, error) {
	id, err := uuidString(row.ID)
	if err != nil {
		return teams.TeamApplication{}, err
	}
	teamID, err := uuidString(row.TeamID)
	if err != nil {
		return teams.TeamApplication{}, err
	}
	applicationID, err := uuidString(row.ApplicationID)
	if err != nil {
		return teams.TeamApplication{}, err
	}
	return teams.TeamApplication{
		ID: id, TeamID: teamID, ApplicationID: applicationID,
		Permission: rbac.TeamAppPermission(row.Permission), AddedAt: row.AddedAt.Time,
	}, nil
}

func teamApplicationsFromRows(rows []controlstore.ListTeamApplicationsRow) ([]teams.TeamApplication, error) {
	result := make([]teams.TeamApplication, 0, len(rows))
	for _, row := range rows {
		application, err := teamApplicationRecord(controlstore.ControlTeamApplication{
			ID: row.ID, TeamID: row.TeamID, ApplicationID: row.ApplicationID,
			Permission: row.Permission, AddedAt: row.AddedAt,
		})
		if err != nil {
			return nil, err
		}
		applicationID, err := uuidString(row.ApplicationID)
		if err != nil {
			return nil, err
		}
		application.Application = &teams.ApplicationSummary{
			ID: applicationID, Name: row.Name, Description: textPointer(row.Description),
			Icon: textPointer(row.Icon), Status: row.Status,
		}
		result = append(result, application)
	}
	return result, nil
}

func userSummary(row controlstore.ControlUser) *teams.UserSummary {
	id, err := uuidString(row.ID)
	if err != nil {
		return nil
	}
	return &teams.UserSummary{ID: id, Username: row.Username, Avatar: textPointer(row.Avatar), CreatedAt: row.CreatedAt.Time}
}

func applicationSummary(row controlstore.ControlApplication) *teams.ApplicationSummary {
	id, err := uuidString(row.ID)
	if err != nil {
		return nil
	}
	return &teams.ApplicationSummary{
		ID: id, Name: row.Name, Description: textPointer(row.Description), Icon: textPointer(row.Icon), Status: row.Status,
	}
}

func uuidEqual(left, right pgtype.UUID) bool {
	return left.Valid && right.Valid && bytes.Equal(left.Bytes[:], right.Bytes[:])
}

func mapTeamError(err error, missing error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return missing
	}
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) {
		return err
	}
	switch databaseError.Code {
	case "23505":
		return teams.ErrTeamConflict
	case "23503":
		switch {
		case strings.Contains(databaseError.ConstraintName, "user_id"):
			return teams.ErrUserNotFound
		case strings.Contains(databaseError.ConstraintName, "application_id"):
			return teams.ErrApplicationNotFound
		case strings.Contains(databaseError.ConstraintName, "team_id"):
			return teams.ErrTeamNotFound
		}
	}
	return err
}
