package teams

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/rbac"
)

type fakeTeamStore struct {
	team             Team
	teams            []Team
	member           Member
	teamApplication  TeamApplication
	applicationOwner string
	err              error
	createInput      CreateInput
	memberInput      AddMemberInput
	memberRole       rbac.TeamRole
	applicationInput AddApplicationInput
	leftUserID       string
	deletedTeamID    string
}

func (store *fakeTeamStore) CreateTeam(_ context.Context, _ string, input CreateInput) (Team, error) {
	store.createInput = input
	return store.team, store.err
}

func (store *fakeTeamStore) ListTeams(context.Context, string) ([]Team, error) {
	return store.teams, store.err
}

func (store *fakeTeamStore) GetTeam(context.Context, string, string) (Team, error) {
	return store.team, store.err
}

func (store *fakeTeamStore) UpdateTeam(_ context.Context, _ string, input UpdateInput) (Team, error) {
	return store.team, store.err
}

func (store *fakeTeamStore) DeleteTeam(_ context.Context, id string) error {
	store.deletedTeamID = id
	return store.err
}

func (store *fakeTeamStore) AddMember(_ context.Context, _ string, input AddMemberInput) (Member, error) {
	store.memberInput = input
	return store.member, store.err
}

func (store *fakeTeamStore) GetMember(context.Context, string, string) (Member, error) {
	return store.member, store.err
}

func (store *fakeTeamStore) UpdateMemberRole(_ context.Context, _, _ string, role rbac.TeamRole) (Member, error) {
	store.memberRole = role
	member := store.member
	member.Role = role
	return member, store.err
}

func (store *fakeTeamStore) RemoveMember(context.Context, string, string) error {
	return store.err
}

func (store *fakeTeamStore) LeaveTeam(_ context.Context, _ string, userID string) error {
	store.leftUserID = userID
	return store.err
}

func (store *fakeTeamStore) ApplicationOwnerID(context.Context, string) (string, error) {
	return store.applicationOwner, store.err
}

func (store *fakeTeamStore) AddApplication(_ context.Context, _ string, input AddApplicationInput) (TeamApplication, error) {
	store.applicationInput = input
	return store.teamApplication, store.err
}

func (store *fakeTeamStore) GetTeamApplication(context.Context, string, string) (TeamApplication, error) {
	return store.teamApplication, store.err
}

func (store *fakeTeamStore) UpdateApplicationPermission(_ context.Context, _, _ string, permission rbac.TeamAppPermission) (TeamApplication, error) {
	application := store.teamApplication
	application.Permission = permission
	return application, store.err
}

func (store *fakeTeamStore) RemoveApplication(context.Context, string, string) error {
	return store.err
}

type fakeTeamAuthorizer struct {
	err         error
	permissions []rbac.Permission
}

func (authorizer *fakeTeamAuthorizer) AuthorizeTeam(
	_ context.Context,
	_ string,
	_ string,
	permissions ...rbac.Permission,
) error {
	authorizer.permissions = append([]rbac.Permission(nil), permissions...)
	return authorizer.err
}

func sampleTeam() Team {
	return Team{
		ID:        "0b44ed01-327e-412a-a79e-63ef34d981fe",
		Name:      "Team",
		OwnerID:   "e9f6332d-da39-44b2-917c-da5ff30aca9d",
		MyRole:    rbac.TeamRoleOwner,
		CreatedAt: time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC),
	}
}

func sampleMember() Member {
	return Member{
		ID:       "83a55a85-0c52-42f8-ac67-d6be4ba41be1",
		TeamID:   "0b44ed01-327e-412a-a79e-63ef34d981fe",
		UserID:   "4dc3923c-331f-4a80-9ea6-cf392e991821",
		Role:     rbac.TeamRoleViewer,
		JoinedAt: time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC),
	}
}

func TestCreateAndListTeamsPreserveOwnerMembershipAndCounts(t *testing.T) {
	team := sampleTeam()
	team.MemberCount = 1
	store := &fakeTeamStore{team: team, teams: []Team{team}}
	service := NewService(store, &fakeTeamAuthorizer{})

	created, err := service.Create(context.Background(), team.OwnerID, CreateInput{Name: "Team"})
	if err != nil || created.OwnerID != team.OwnerID {
		t.Fatalf("created = %#v, error = %v", created, err)
	}
	listed, err := service.List(context.Background(), team.OwnerID)
	if err != nil || len(listed) != 1 || listed[0].MyRole != rbac.TeamRoleOwner || listed[0].MemberCount != 1 {
		t.Fatalf("listed = %#v, error = %v", listed, err)
	}

	for _, input := range []CreateInput{{Name: ""}, {Name: string(make([]byte, 51))}, {Name: "Team", Description: stringPointer(string(make([]byte, 201)))}} {
		_, err := service.Create(context.Background(), team.OwnerID, input)
		assertTeamError(t, err, ErrorInvalidInput)
	}
}

func TestTeamReadUpdateAndDeleteAuthorization(t *testing.T) {
	team := sampleTeam()
	authorizer := &fakeTeamAuthorizer{}
	store := &fakeTeamStore{team: team}
	service := NewService(store, authorizer)

	if _, err := service.Get(context.Background(), team.OwnerID, team.ID); err != nil || authorizer.permissions[0] != rbac.PermissionTeamRead {
		t.Fatalf("get error = %v, permissions = %#v", err, authorizer.permissions)
	}
	name := "Updated"
	if _, err := service.Update(context.Background(), team.OwnerID, team.ID, UpdateInput{Name: &name}); err != nil || authorizer.permissions[0] != rbac.PermissionTeamUpdate {
		t.Fatalf("update error = %v, permissions = %#v", err, authorizer.permissions)
	}
	if err := service.Delete(context.Background(), team.OwnerID, team.ID); err != nil || store.deletedTeamID != team.ID {
		t.Fatalf("delete error = %v, deleted = %q", err, store.deletedTeamID)
	}

	service = NewService(&fakeTeamStore{team: team}, &fakeTeamAuthorizer{})
	if err := service.Delete(context.Background(), "not-owner", team.ID); err == nil {
		t.Fatal("Delete() allowed a non-owner")
	}
}

func TestMemberManagementRejectsSelfOwnerAndDuplicateOperations(t *testing.T) {
	team := sampleTeam()
	member := sampleMember()
	store := &fakeTeamStore{team: team, member: member}
	service := NewService(store, &fakeTeamAuthorizer{})

	_, err := service.AddMember(context.Background(), team.OwnerID, team.ID, AddMemberInput{UserID: team.OwnerID, Role: rbac.TeamRoleViewer})
	assertTeamError(t, err, ErrorInvalidInput)
	_, err = service.AddMember(context.Background(), team.OwnerID, team.ID, AddMemberInput{UserID: member.UserID, Role: rbac.TeamRoleOwner})
	assertTeamError(t, err, ErrorInvalidInput)

	store.err = ErrTeamConflict
	_, err = service.AddMember(context.Background(), team.OwnerID, team.ID, AddMemberInput{UserID: member.UserID, Role: rbac.TeamRoleViewer})
	assertTeamError(t, err, ErrorConflict)
	store.err = nil

	ownerMember := member
	ownerMember.UserID = team.OwnerID
	ownerMember.Role = rbac.TeamRoleOwner
	store.member = ownerMember
	_, err = service.UpdateMemberRole(context.Background(), team.OwnerID, team.ID, ownerMember.ID, rbac.TeamRoleAdmin)
	assertTeamError(t, err, ErrorForbidden)
	err = service.RemoveMember(context.Background(), team.OwnerID, team.ID, ownerMember.ID)
	assertTeamError(t, err, ErrorForbidden)
}

func TestLeaveTeamRejectsOwnerAndIsOtherwiseDelegated(t *testing.T) {
	team := sampleTeam()
	store := &fakeTeamStore{team: team}
	service := NewService(store, &fakeTeamAuthorizer{})

	err := service.Leave(context.Background(), team.OwnerID, team.ID)
	assertTeamError(t, err, ErrorInvalidInput)

	memberID := "4dc3923c-331f-4a80-9ea6-cf392e991821"
	err = service.Leave(context.Background(), memberID, team.ID)
	if err != nil || store.leftUserID != memberID {
		t.Fatalf("leave error = %v, left = %q", err, store.leftUserID)
	}
}

func TestTeamApplicationManagementRequiresTeamAdminAndApplicationOwner(t *testing.T) {
	team := sampleTeam()
	application := TeamApplication{ID: "f32384a3-22b1-4913-ab40-e68a980dafd9", TeamID: team.ID, ApplicationID: "7a611d9a-b555-4469-a289-f1672daefce3", Permission: rbac.TeamAppCanView}
	store := &fakeTeamStore{team: team, teamApplication: application, applicationOwner: team.OwnerID}
	authorizer := &fakeTeamAuthorizer{}
	service := NewService(store, authorizer)

	added, err := service.AddApplication(context.Background(), team.OwnerID, team.ID, AddApplicationInput{ApplicationID: application.ApplicationID, Permission: rbac.TeamAppCanEdit})
	if err != nil || added.ID != application.ID || authorizer.permissions[0] != rbac.PermissionTeamUpdate {
		t.Fatalf("added = %#v, error = %v, permissions = %#v", added, err, authorizer.permissions)
	}

	store.applicationOwner = "someone-else"
	_, err = service.AddApplication(context.Background(), team.OwnerID, team.ID, AddApplicationInput{ApplicationID: application.ApplicationID, Permission: rbac.TeamAppCanView})
	assertTeamError(t, err, ErrorForbidden)

	store.applicationOwner = team.OwnerID
	updated, err := service.UpdateApplicationPermission(context.Background(), team.OwnerID, team.ID, application.ID, rbac.TeamAppFullAccess)
	if err != nil || updated.Permission != rbac.TeamAppFullAccess {
		t.Fatalf("updated = %#v, error = %v", updated, err)
	}
	if err := service.RemoveApplication(context.Background(), team.OwnerID, team.ID, application.ID); err != nil {
		t.Fatalf("RemoveApplication() error = %v", err)
	}
}

func TestTeamServiceMapsAuthorizationAndStoreErrors(t *testing.T) {
	service := NewService(&fakeTeamStore{team: sampleTeam()}, &fakeTeamAuthorizer{err: rbac.ErrForbidden})
	_, err := service.Get(context.Background(), "user", "team")
	assertTeamError(t, err, ErrorForbidden)

	service = NewService(&fakeTeamStore{err: ErrTeamNotFound}, &fakeTeamAuthorizer{})
	_, err = service.Get(context.Background(), "user", "team")
	assertTeamError(t, err, ErrorNotFound)
}

func assertTeamError(t *testing.T, err error, kind ErrorKind) *ServiceError {
	t.Helper()
	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) {
		t.Fatalf("error = %v, want ServiceError", err)
	}
	if serviceErr.Kind != kind {
		t.Fatalf("kind = %q, want %q", serviceErr.Kind, kind)
	}
	return serviceErr
}

func stringPointer(value string) *string {
	return &value
}
