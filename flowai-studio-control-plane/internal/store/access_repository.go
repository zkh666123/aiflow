package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/rbac"
	controlstore "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type accessQueries interface {
	GetUserGlobalRole(context.Context, pgtype.UUID) (string, error)
	ListApplicationAccessForUser(
		context.Context,
		controlstore.ListApplicationAccessForUserParams,
	) ([]controlstore.ListApplicationAccessForUserRow, error)
	GetTeamByID(context.Context, pgtype.UUID) (controlstore.ControlTeam, error)
	GetTeamMembership(
		context.Context,
		controlstore.GetTeamMembershipParams,
	) (controlstore.ControlTeamMember, error)
}

type AccessRepository struct {
	queries accessQueries
}

func NewAccessRepository(queries accessQueries) *AccessRepository {
	return &AccessRepository{queries: queries}
}

func (repository *AccessRepository) ApplicationAccess(
	ctx context.Context,
	userValue string,
	applicationValue string,
) (rbac.ApplicationAccess, error) {
	userID, err := parseUUID(userValue)
	if err != nil {
		return rbac.ApplicationAccess{}, rbac.ErrResourceNotFound
	}
	applicationID, err := parseUUID(applicationValue)
	if err != nil {
		return rbac.ApplicationAccess{}, rbac.ErrResourceNotFound
	}
	rows, err := repository.queries.ListApplicationAccessForUser(
		ctx,
		controlstore.ListApplicationAccessForUserParams{
			ApplicationID: applicationID,
			UserID:        userID,
		},
	)
	if err != nil {
		return rbac.ApplicationAccess{}, fmt.Errorf("query application access: %w", err)
	}
	if len(rows) == 0 {
		return rbac.ApplicationAccess{}, rbac.ErrResourceNotFound
	}
	ownerID, err := uuidString(rows[0].OwnerID)
	if err != nil {
		return rbac.ApplicationAccess{}, fmt.Errorf("decode application owner: %w", err)
	}
	globalRole, err := parseGlobalRole(rows[0].GlobalRole)
	if err != nil {
		return rbac.ApplicationAccess{}, err
	}
	access := rbac.ApplicationAccess{OwnerID: ownerID, GlobalRole: globalRole}
	for _, row := range rows {
		if !row.TeamRole.Valid || !row.TeamApplicationPermission.Valid {
			continue
		}
		role, err := parseTeamRole(row.TeamRole.String)
		if err != nil {
			return rbac.ApplicationAccess{}, err
		}
		permission, err := parseTeamAppPermission(row.TeamApplicationPermission.String)
		if err != nil {
			return rbac.ApplicationAccess{}, err
		}
		access.Grants = append(access.Grants, rbac.ApplicationGrant{
			Role:       role,
			Permission: permission,
		})
	}
	return access, nil
}

func (repository *AccessRepository) TeamAccess(
	ctx context.Context,
	userValue string,
	teamValue string,
) (rbac.TeamAccess, error) {
	userID, err := parseUUID(userValue)
	if err != nil {
		return rbac.TeamAccess{}, rbac.ErrResourceNotFound
	}
	teamID, err := parseUUID(teamValue)
	if err != nil {
		return rbac.TeamAccess{}, rbac.ErrResourceNotFound
	}
	globalRoleValue, err := repository.queries.GetUserGlobalRole(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return rbac.TeamAccess{}, rbac.ErrResourceNotFound
	}
	if err != nil {
		return rbac.TeamAccess{}, fmt.Errorf("query global role: %w", err)
	}
	globalRole, err := parseGlobalRole(globalRoleValue)
	if err != nil {
		return rbac.TeamAccess{}, err
	}
	team, err := repository.queries.GetTeamByID(ctx, teamID)
	if errors.Is(err, pgx.ErrNoRows) {
		return rbac.TeamAccess{}, rbac.ErrResourceNotFound
	}
	if err != nil {
		return rbac.TeamAccess{}, fmt.Errorf("query team: %w", err)
	}
	ownerID, err := uuidString(team.OwnerID)
	if err != nil {
		return rbac.TeamAccess{}, fmt.Errorf("decode team owner: %w", err)
	}
	access := rbac.TeamAccess{GlobalRole: globalRole, OwnerID: ownerID}
	membership, err := repository.queries.GetTeamMembership(ctx, controlstore.GetTeamMembershipParams{
		TeamID: teamID,
		UserID: userID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return access, nil
	}
	if err != nil {
		return rbac.TeamAccess{}, fmt.Errorf("query team membership: %w", err)
	}
	role, err := parseTeamRole(membership.Role)
	if err != nil {
		return rbac.TeamAccess{}, err
	}
	access.Member = true
	access.Role = role
	return access, nil
}

func parseGlobalRole(value string) (rbac.GlobalRole, error) {
	switch rbac.GlobalRole(value) {
	case rbac.GlobalRoleAdmin, rbac.GlobalRoleMember:
		return rbac.GlobalRole(value), nil
	default:
		return "", fmt.Errorf("unknown global role %q", value)
	}
}

func parseTeamRole(value string) (rbac.TeamRole, error) {
	switch rbac.TeamRole(value) {
	case rbac.TeamRoleOwner, rbac.TeamRoleAdmin, rbac.TeamRoleEditor, rbac.TeamRoleViewer:
		return rbac.TeamRole(value), nil
	default:
		return "", fmt.Errorf("unknown team role %q", value)
	}
}

func parseTeamAppPermission(value string) (rbac.TeamAppPermission, error) {
	switch rbac.TeamAppPermission(value) {
	case rbac.TeamAppFullAccess, rbac.TeamAppCanEdit, rbac.TeamAppCanView:
		return rbac.TeamAppPermission(value), nil
	default:
		return "", fmt.Errorf("unknown team application permission %q", value)
	}
}
