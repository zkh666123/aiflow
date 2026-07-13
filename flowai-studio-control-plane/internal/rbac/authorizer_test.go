package rbac

import (
	"context"
	"errors"
	"testing"
)

type fakeAccessStore struct {
	application ApplicationAccess
	team        TeamAccess
	err         error
}

func (store *fakeAccessStore) ApplicationAccess(context.Context, string, string) (ApplicationAccess, error) {
	return store.application, store.err
}

func (store *fakeAccessStore) TeamAccess(context.Context, string, string) (TeamAccess, error) {
	return store.team, store.err
}

func TestApplicationAuthorizationUsesGlobalOwnerRoleAndGrantOrder(t *testing.T) {
	userID := "user-1"
	tests := []struct {
		name        string
		access      ApplicationAccess
		permissions []Permission
		wantErr     error
	}{
		{
			name:        "global admin",
			access:      ApplicationAccess{GlobalRole: GlobalRoleAdmin, OwnerID: "someone-else"},
			permissions: []Permission{PermissionAppDelete, PermissionAppShare},
		},
		{
			name:        "application owner",
			access:      ApplicationAccess{GlobalRole: GlobalRoleMember, OwnerID: userID},
			permissions: []Permission{PermissionAppDelete, PermissionAppShare},
		},
		{
			name:        "team editor role",
			access:      ApplicationAccess{GlobalRole: GlobalRoleMember, OwnerID: "owner", Grants: []ApplicationGrant{{Role: TeamRoleEditor, Permission: TeamAppCanView}}},
			permissions: []Permission{PermissionAppUpdate},
		},
		{
			name:        "team app can edit",
			access:      ApplicationAccess{GlobalRole: GlobalRoleMember, OwnerID: "owner", Grants: []ApplicationGrant{{Role: TeamRoleViewer, Permission: TeamAppCanEdit}}},
			permissions: []Permission{PermissionWorkflowExecute},
		},
		{
			name:        "all requested permissions required",
			access:      ApplicationAccess{GlobalRole: GlobalRoleMember, OwnerID: "owner", Grants: []ApplicationGrant{{Role: TeamRoleViewer, Permission: TeamAppCanEdit}}},
			permissions: []Permission{PermissionAppUpdate, PermissionAppDelete},
			wantErr:     ErrForbidden,
		},
		{
			name:        "unrelated grant cannot authorize",
			access:      ApplicationAccess{GlobalRole: GlobalRoleMember, OwnerID: "owner"},
			permissions: []Permission{PermissionAppRead},
			wantErr:     ErrForbidden,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorizer := NewAuthorizer(&fakeAccessStore{application: test.access})
			err := authorizer.AuthorizeApplication(context.Background(), userID, "app-1", test.permissions...)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestApplicationAuthorizationKeepsNotFoundSeparateFromForbidden(t *testing.T) {
	authorizer := NewAuthorizer(&fakeAccessStore{err: ErrResourceNotFound})
	err := authorizer.AuthorizeApplication(context.Background(), "user", "missing", PermissionAppRead)
	if !errors.Is(err, ErrResourceNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestTeamAuthorizationUsesGlobalOwnerAndMembershipRoles(t *testing.T) {
	userID := "user-1"
	tests := []struct {
		name       string
		access     TeamAccess
		permission Permission
		wantErr    error
	}{
		{name: "global admin", access: TeamAccess{GlobalRole: GlobalRoleAdmin}, permission: PermissionTeamDelete},
		{name: "owner", access: TeamAccess{GlobalRole: GlobalRoleMember, OwnerID: userID}, permission: PermissionTeamManageMembers},
		{name: "team admin", access: TeamAccess{GlobalRole: GlobalRoleMember, OwnerID: "owner", Role: TeamRoleAdmin, Member: true}, permission: PermissionTeamUpdate},
		{name: "viewer reads", access: TeamAccess{GlobalRole: GlobalRoleMember, OwnerID: "owner", Role: TeamRoleViewer, Member: true}, permission: PermissionTeamRead},
		{name: "viewer cannot update", access: TeamAccess{GlobalRole: GlobalRoleMember, OwnerID: "owner", Role: TeamRoleViewer, Member: true}, permission: PermissionTeamUpdate, wantErr: ErrForbidden},
		{name: "non member", access: TeamAccess{GlobalRole: GlobalRoleMember, OwnerID: "owner"}, permission: PermissionTeamRead, wantErr: ErrForbidden},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorizer := NewAuthorizer(&fakeAccessStore{team: test.access})
			err := authorizer.AuthorizeTeam(context.Background(), userID, "team-1", test.permission)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
		})
	}
}
