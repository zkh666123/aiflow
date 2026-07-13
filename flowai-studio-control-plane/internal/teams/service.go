package teams

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/rbac"
)

var (
	ErrTeamNotFound            = errors.New("team not found")
	ErrMemberNotFound          = errors.New("team member not found")
	ErrTeamApplicationNotFound = errors.New("team application not found")
	ErrUserNotFound            = errors.New("user not found")
	ErrApplicationNotFound     = errors.New("application not found")
	ErrTeamConflict            = errors.New("team conflict")
)

type UserSummary struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Avatar    *string   `json:"avatar"`
	CreatedAt time.Time `json:"createdAt"`
}

type Member struct {
	ID       string        `json:"id"`
	TeamID   string        `json:"teamId"`
	UserID   string        `json:"userId"`
	Role     rbac.TeamRole `json:"role"`
	JoinedAt time.Time     `json:"joinedAt"`
	User     *UserSummary  `json:"user,omitempty"`
}

type ApplicationSummary struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Icon        *string `json:"icon"`
	Status      string  `json:"status"`
}

type TeamApplication struct {
	ID            string                 `json:"id"`
	TeamID        string                 `json:"teamId"`
	ApplicationID string                 `json:"applicationId"`
	Permission    rbac.TeamAppPermission `json:"permission"`
	AddedAt       time.Time              `json:"addedAt"`
	Application   *ApplicationSummary    `json:"application,omitempty"`
}

type Team struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  *string           `json:"description"`
	Avatar       *string           `json:"avatar"`
	OwnerID      string            `json:"ownerId"`
	MyRole       rbac.TeamRole     `json:"myRole,omitempty"`
	MemberCount  int64             `json:"memberCount,omitempty"`
	AppCount     int64             `json:"appCount,omitempty"`
	Members      []Member          `json:"members,omitempty"`
	Applications []TeamApplication `json:"applications,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
}

type CreateInput struct {
	Name        string
	Description *string
	Avatar      *string
}

type UpdateInput struct {
	Name           *string
	DescriptionSet bool
	Description    *string
	AvatarSet      bool
	Avatar         *string
}

type AddMemberInput struct {
	UserID string
	Role   rbac.TeamRole
}

type AddApplicationInput struct {
	ApplicationID string
	Permission    rbac.TeamAppPermission
}

type ErrorKind string

const (
	ErrorInvalidInput ErrorKind = "invalid_input"
	ErrorForbidden    ErrorKind = "forbidden"
	ErrorNotFound     ErrorKind = "not_found"
	ErrorConflict     ErrorKind = "conflict"
	ErrorInternal     ErrorKind = "internal"
)

type ServiceError struct {
	Kind    ErrorKind
	Message string
	Cause   error
}

func (err *ServiceError) Error() string {
	return err.Message
}

func (err *ServiceError) Unwrap() error {
	return err.Cause
}

type Store interface {
	CreateTeam(context.Context, string, CreateInput) (Team, error)
	ListTeams(context.Context, string) ([]Team, error)
	GetTeam(context.Context, string, string) (Team, error)
	UpdateTeam(context.Context, string, UpdateInput) (Team, error)
	DeleteTeam(context.Context, string) error
	AddMember(context.Context, string, AddMemberInput) (Member, error)
	GetMember(context.Context, string, string) (Member, error)
	UpdateMemberRole(context.Context, string, string, rbac.TeamRole) (Member, error)
	RemoveMember(context.Context, string, string) error
	LeaveTeam(context.Context, string, string) error
	ApplicationOwnerID(context.Context, string) (string, error)
	AddApplication(context.Context, string, AddApplicationInput) (TeamApplication, error)
	GetTeamApplication(context.Context, string, string) (TeamApplication, error)
	UpdateApplicationPermission(context.Context, string, string, rbac.TeamAppPermission) (TeamApplication, error)
	RemoveApplication(context.Context, string, string) error
}

type Authorizer interface {
	AuthorizeTeam(context.Context, string, string, ...rbac.Permission) error
}

type Service struct {
	store      Store
	authorizer Authorizer
}

func NewService(store Store, authorizer Authorizer) *Service {
	return &Service{store: store, authorizer: authorizer}
}

func (service *Service) Create(ctx context.Context, userID string, input CreateInput) (Team, error) {
	if err := validateCreateInput(input); err != nil {
		return Team{}, err
	}
	team, err := service.store.CreateTeam(ctx, userID, input)
	if err != nil {
		return Team{}, mapStoreError("Failed to create team", err)
	}
	return team, nil
}

func (service *Service) List(ctx context.Context, userID string) ([]Team, error) {
	teams, err := service.store.ListTeams(ctx, userID)
	if err != nil {
		return nil, mapStoreError("Failed to list teams", err)
	}
	return teams, nil
}

func (service *Service) Get(ctx context.Context, userID, teamID string) (Team, error) {
	if err := service.authorize(ctx, userID, teamID, rbac.PermissionTeamRead); err != nil {
		return Team{}, err
	}
	team, err := service.store.GetTeam(ctx, teamID, userID)
	if err != nil {
		return Team{}, mapStoreError("Failed to get team", err)
	}
	return team, nil
}

func (service *Service) Update(ctx context.Context, userID, teamID string, input UpdateInput) (Team, error) {
	if err := validateUpdateInput(input); err != nil {
		return Team{}, err
	}
	if err := service.authorize(ctx, userID, teamID, rbac.PermissionTeamUpdate); err != nil {
		return Team{}, err
	}
	team, err := service.store.UpdateTeam(ctx, teamID, input)
	if err != nil {
		return Team{}, mapStoreError("Failed to update team", err)
	}
	return team, nil
}

func (service *Service) Delete(ctx context.Context, userID, teamID string) error {
	if err := service.authorize(ctx, userID, teamID, rbac.PermissionTeamDelete); err != nil {
		return err
	}
	team, err := service.store.GetTeam(ctx, teamID, userID)
	if err != nil {
		return mapStoreError("Failed to get team", err)
	}
	if team.OwnerID != userID {
		return &ServiceError{Kind: ErrorForbidden, Message: "Only the team owner can delete the team"}
	}
	if err := service.store.DeleteTeam(ctx, teamID); err != nil {
		return mapStoreError("Failed to delete team", err)
	}
	return nil
}

func (service *Service) AddMember(ctx context.Context, userID, teamID string, input AddMemberInput) (Member, error) {
	if input.UserID == userID {
		return Member{}, &ServiceError{Kind: ErrorInvalidInput, Message: "You cannot add yourself"}
	}
	if !validAssignableRole(input.Role) {
		return Member{}, &ServiceError{Kind: ErrorInvalidInput, Message: "Role must be admin, editor, or viewer"}
	}
	if err := service.authorize(ctx, userID, teamID, rbac.PermissionTeamManageMembers); err != nil {
		return Member{}, err
	}
	member, err := service.store.AddMember(ctx, teamID, input)
	if err != nil {
		return Member{}, mapStoreError("Failed to add team member", err)
	}
	return member, nil
}

func (service *Service) UpdateMemberRole(
	ctx context.Context,
	userID string,
	teamID string,
	memberID string,
	role rbac.TeamRole,
) (Member, error) {
	if !validAssignableRole(role) {
		return Member{}, &ServiceError{Kind: ErrorInvalidInput, Message: "Role must be admin, editor, or viewer"}
	}
	if err := service.authorize(ctx, userID, teamID, rbac.PermissionTeamManageMembers); err != nil {
		return Member{}, err
	}
	team, member, err := service.teamAndMember(ctx, userID, teamID, memberID)
	if err != nil {
		return Member{}, err
	}
	if member.Role == rbac.TeamRoleOwner || member.UserID == team.OwnerID {
		return Member{}, &ServiceError{Kind: ErrorForbidden, Message: "The team owner role cannot be changed"}
	}
	updated, err := service.store.UpdateMemberRole(ctx, teamID, memberID, role)
	if err != nil {
		return Member{}, mapStoreError("Failed to update member role", err)
	}
	return updated, nil
}

func (service *Service) RemoveMember(ctx context.Context, userID, teamID, memberID string) error {
	if err := service.authorize(ctx, userID, teamID, rbac.PermissionTeamManageMembers); err != nil {
		return err
	}
	team, member, err := service.teamAndMember(ctx, userID, teamID, memberID)
	if err != nil {
		return err
	}
	if member.UserID == team.OwnerID || member.Role == rbac.TeamRoleOwner {
		return &ServiceError{Kind: ErrorForbidden, Message: "The team owner cannot be removed"}
	}
	if err := service.store.RemoveMember(ctx, teamID, memberID); err != nil {
		return mapStoreError("Failed to remove member", err)
	}
	return nil
}

func (service *Service) Leave(ctx context.Context, userID, teamID string) error {
	team, err := service.store.GetTeam(ctx, teamID, userID)
	if err != nil {
		return mapStoreError("Failed to get team", err)
	}
	if team.OwnerID == userID {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "The team owner cannot leave the team"}
	}
	if err := service.store.LeaveTeam(ctx, teamID, userID); err != nil {
		return mapStoreError("Failed to leave team", err)
	}
	return nil
}

func (service *Service) AddApplication(
	ctx context.Context,
	userID string,
	teamID string,
	input AddApplicationInput,
) (TeamApplication, error) {
	if !validTeamAppPermission(input.Permission) {
		return TeamApplication{}, &ServiceError{Kind: ErrorInvalidInput, Message: "Permission must be full_access, can_edit, or can_view"}
	}
	if err := service.authorize(ctx, userID, teamID, rbac.PermissionTeamUpdate); err != nil {
		return TeamApplication{}, err
	}
	ownerID, err := service.store.ApplicationOwnerID(ctx, input.ApplicationID)
	if err != nil {
		return TeamApplication{}, mapStoreError("Failed to get application", err)
	}
	if ownerID != userID {
		return TeamApplication{}, &ServiceError{Kind: ErrorForbidden, Message: "Only the application owner can share it with a team"}
	}
	application, err := service.store.AddApplication(ctx, teamID, input)
	if err != nil {
		return TeamApplication{}, mapStoreError("Failed to add application to team", err)
	}
	return application, nil
}

func (service *Service) UpdateApplicationPermission(
	ctx context.Context,
	userID string,
	teamID string,
	teamApplicationID string,
	permission rbac.TeamAppPermission,
) (TeamApplication, error) {
	if !validTeamAppPermission(permission) {
		return TeamApplication{}, &ServiceError{Kind: ErrorInvalidInput, Message: "Permission must be full_access, can_edit, or can_view"}
	}
	if err := service.authorize(ctx, userID, teamID, rbac.PermissionTeamUpdate); err != nil {
		return TeamApplication{}, err
	}
	if _, err := service.store.GetTeamApplication(ctx, teamID, teamApplicationID); err != nil {
		return TeamApplication{}, mapStoreError("Failed to get team application", err)
	}
	application, err := service.store.UpdateApplicationPermission(ctx, teamID, teamApplicationID, permission)
	if err != nil {
		return TeamApplication{}, mapStoreError("Failed to update team application", err)
	}
	return application, nil
}

func (service *Service) RemoveApplication(ctx context.Context, userID, teamID, teamApplicationID string) error {
	if err := service.authorize(ctx, userID, teamID, rbac.PermissionTeamUpdate); err != nil {
		return err
	}
	if _, err := service.store.GetTeamApplication(ctx, teamID, teamApplicationID); err != nil {
		return mapStoreError("Failed to get team application", err)
	}
	if err := service.store.RemoveApplication(ctx, teamID, teamApplicationID); err != nil {
		return mapStoreError("Failed to remove team application", err)
	}
	return nil
}

func (service *Service) teamAndMember(
	ctx context.Context,
	userID string,
	teamID string,
	memberID string,
) (Team, Member, error) {
	team, err := service.store.GetTeam(ctx, teamID, userID)
	if err != nil {
		return Team{}, Member{}, mapStoreError("Failed to get team", err)
	}
	member, err := service.store.GetMember(ctx, teamID, memberID)
	if err != nil {
		return Team{}, Member{}, mapStoreError("Failed to get team member", err)
	}
	return team, member, nil
}

func (service *Service) authorize(ctx context.Context, userID, teamID string, permissions ...rbac.Permission) error {
	err := service.authorizer.AuthorizeTeam(ctx, userID, teamID, permissions...)
	if errors.Is(err, rbac.ErrResourceNotFound) {
		return &ServiceError{Kind: ErrorNotFound, Message: "Team not found", Cause: err}
	}
	if errors.Is(err, rbac.ErrForbidden) {
		return &ServiceError{Kind: ErrorForbidden, Message: "You do not have permission to manage this team", Cause: err}
	}
	if err != nil {
		return &ServiceError{Kind: ErrorInternal, Message: "Failed to authorize team", Cause: err}
	}
	return nil
}

func validateCreateInput(input CreateInput) error {
	if strings.TrimSpace(input.Name) == "" || utf8.RuneCountInString(input.Name) > 50 {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Team name must contain 1-50 characters"}
	}
	if input.Description != nil && utf8.RuneCountInString(*input.Description) > 200 {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Team description must not exceed 200 characters"}
	}
	if input.Avatar != nil && len(*input.Avatar) > 2048 {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Team avatar must not exceed 2048 characters"}
	}
	return nil
}

func validateUpdateInput(input UpdateInput) error {
	return validateCreateInput(CreateInput{Name: valueOrDefault(input.Name, "valid"), Description: input.Description, Avatar: input.Avatar})
}

func valueOrDefault(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}

func validAssignableRole(role rbac.TeamRole) bool {
	switch role {
	case rbac.TeamRoleAdmin, rbac.TeamRoleEditor, rbac.TeamRoleViewer:
		return true
	default:
		return false
	}
}

func validTeamAppPermission(permission rbac.TeamAppPermission) bool {
	switch permission {
	case rbac.TeamAppFullAccess, rbac.TeamAppCanEdit, rbac.TeamAppCanView:
		return true
	default:
		return false
	}
}

func mapStoreError(message string, err error) *ServiceError {
	switch {
	case errors.Is(err, ErrTeamNotFound),
		errors.Is(err, ErrMemberNotFound),
		errors.Is(err, ErrTeamApplicationNotFound),
		errors.Is(err, ErrUserNotFound),
		errors.Is(err, ErrApplicationNotFound):
		return &ServiceError{Kind: ErrorNotFound, Message: message, Cause: err}
	case errors.Is(err, ErrTeamConflict):
		return &ServiceError{Kind: ErrorConflict, Message: "The resource already exists", Cause: err}
	default:
		return &ServiceError{Kind: ErrorInternal, Message: message, Cause: err}
	}
}
