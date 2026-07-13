package rbac

import "testing"

func TestTeamRolePermissionMatrixMatchesTheLegacyContract(t *testing.T) {
	for _, role := range []TeamRole{TeamRoleOwner, TeamRoleAdmin} {
		for _, permission := range AllPermissions() {
			if !RoleHasPermission(role, permission) {
				t.Fatalf("role %s lacks %s", role, permission)
			}
		}
	}

	tests := []struct {
		role       TeamRole
		permission Permission
		want       bool
	}{
		{role: TeamRoleViewer, permission: PermissionAppRead, want: true},
		{role: TeamRoleViewer, permission: PermissionAppUpdate, want: false},
		{role: TeamRoleEditor, permission: PermissionAppUpdate, want: true},
		{role: TeamRoleEditor, permission: PermissionAppDelete, want: false},
		{role: TeamRoleEditor, permission: PermissionWorkflowDelete, want: true},
		{role: TeamRoleEditor, permission: PermissionTeamManageMembers, want: false},
		{role: TeamRoleViewer, permission: PermissionAPIKeyRead, want: true},
	}
	for _, test := range tests {
		if got := RoleHasPermission(test.role, test.permission); got != test.want {
			t.Fatalf("RoleHasPermission(%q, %q) = %v, want %v", test.role, test.permission, got, test.want)
		}
	}
}

func TestTeamApplicationGrantMatrixMatchesTheLegacyContract(t *testing.T) {
	tests := []struct {
		grant      TeamAppPermission
		permission Permission
		want       bool
	}{
		{grant: TeamAppCanView, permission: PermissionAppRead, want: true},
		{grant: TeamAppCanView, permission: PermissionWorkflowRead, want: true},
		{grant: TeamAppCanView, permission: PermissionWorkflowExecute, want: false},
		{grant: TeamAppCanEdit, permission: PermissionAppUpdate, want: true},
		{grant: TeamAppCanEdit, permission: PermissionWorkflowExecute, want: true},
		{grant: TeamAppCanEdit, permission: PermissionAppPublish, want: false},
		{grant: TeamAppCanEdit, permission: PermissionAppDelete, want: false},
		{grant: TeamAppFullAccess, permission: PermissionAppShare, want: true},
		{grant: TeamAppFullAccess, permission: PermissionWorkflowDelete, want: true},
		{grant: TeamAppFullAccess, permission: PermissionTeamDelete, want: false},
	}
	for _, test := range tests {
		if got := TeamAppHasPermission(test.grant, test.permission); got != test.want {
			t.Fatalf("TeamAppHasPermission(%q, %q) = %v, want %v", test.grant, test.permission, got, test.want)
		}
	}
}

func TestPermissionConstantsRemainCompleteAndUnique(t *testing.T) {
	permissions := AllPermissions()
	if len(permissions) != 32 {
		t.Fatalf("permission count = %d", len(permissions))
	}
	seen := map[Permission]bool{}
	for _, permission := range permissions {
		if seen[permission] {
			t.Fatalf("duplicate permission %q", permission)
		}
		seen[permission] = true
	}
}
