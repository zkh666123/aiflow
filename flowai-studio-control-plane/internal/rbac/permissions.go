package rbac

type Permission string

const (
	PermissionAppCreate  Permission = "app:create"
	PermissionAppRead    Permission = "app:read"
	PermissionAppUpdate  Permission = "app:update"
	PermissionAppDelete  Permission = "app:delete"
	PermissionAppPublish Permission = "app:publish"
	PermissionAppShare   Permission = "app:share"

	PermissionWorkflowCreate  Permission = "workflow:create"
	PermissionWorkflowRead    Permission = "workflow:read"
	PermissionWorkflowUpdate  Permission = "workflow:update"
	PermissionWorkflowDelete  Permission = "workflow:delete"
	PermissionWorkflowExecute Permission = "workflow:execute"

	PermissionKBCreate Permission = "kb:create"
	PermissionKBRead   Permission = "kb:read"
	PermissionKBUpdate Permission = "kb:update"
	PermissionKBDelete Permission = "kb:delete"

	PermissionSkillCreate Permission = "skill:create"
	PermissionSkillRead   Permission = "skill:read"
	PermissionSkillUpdate Permission = "skill:update"
	PermissionSkillDelete Permission = "skill:delete"

	PermissionTeamCreate        Permission = "team:create"
	PermissionTeamRead          Permission = "team:read"
	PermissionTeamUpdate        Permission = "team:update"
	PermissionTeamDelete        Permission = "team:delete"
	PermissionTeamManageMembers Permission = "team:manage-members"

	PermissionAPIKeyCreate Permission = "api-key:create"
	PermissionAPIKeyRead   Permission = "api-key:read"
	PermissionAPIKeyDelete Permission = "api-key:delete"

	PermissionTemplateCreate  Permission = "template:create"
	PermissionTemplateRead    Permission = "template:read"
	PermissionTemplateUpdate  Permission = "template:update"
	PermissionTemplateDelete  Permission = "template:delete"
	PermissionTemplatePublish Permission = "template:publish"
)

var allPermissions = []Permission{
	PermissionAppCreate,
	PermissionAppRead,
	PermissionAppUpdate,
	PermissionAppDelete,
	PermissionAppPublish,
	PermissionAppShare,
	PermissionWorkflowCreate,
	PermissionWorkflowRead,
	PermissionWorkflowUpdate,
	PermissionWorkflowDelete,
	PermissionWorkflowExecute,
	PermissionKBCreate,
	PermissionKBRead,
	PermissionKBUpdate,
	PermissionKBDelete,
	PermissionSkillCreate,
	PermissionSkillRead,
	PermissionSkillUpdate,
	PermissionSkillDelete,
	PermissionTeamCreate,
	PermissionTeamRead,
	PermissionTeamUpdate,
	PermissionTeamDelete,
	PermissionTeamManageMembers,
	PermissionAPIKeyCreate,
	PermissionAPIKeyRead,
	PermissionAPIKeyDelete,
	PermissionTemplateCreate,
	PermissionTemplateRead,
	PermissionTemplateUpdate,
	PermissionTemplateDelete,
	PermissionTemplatePublish,
}

type GlobalRole string

const (
	GlobalRoleAdmin  GlobalRole = "admin"
	GlobalRoleMember GlobalRole = "member"
)

type TeamRole string

const (
	TeamRoleOwner  TeamRole = "owner"
	TeamRoleAdmin  TeamRole = "admin"
	TeamRoleEditor TeamRole = "editor"
	TeamRoleViewer TeamRole = "viewer"
)

type TeamAppPermission string

const (
	TeamAppFullAccess TeamAppPermission = "full_access"
	TeamAppCanEdit    TeamAppPermission = "can_edit"
	TeamAppCanView    TeamAppPermission = "can_view"
)

var editorPermissions = permissionSet(
	PermissionAppRead,
	PermissionAppUpdate,
	PermissionWorkflowCreate,
	PermissionWorkflowRead,
	PermissionWorkflowUpdate,
	PermissionWorkflowDelete,
	PermissionWorkflowExecute,
	PermissionKBCreate,
	PermissionKBRead,
	PermissionKBUpdate,
	PermissionSkillCreate,
	PermissionSkillRead,
	PermissionSkillUpdate,
	PermissionTemplateCreate,
	PermissionTemplateRead,
	PermissionTemplateUpdate,
	PermissionAPIKeyRead,
	PermissionTeamRead,
)

var viewerPermissions = permissionSet(
	PermissionAppRead,
	PermissionWorkflowRead,
	PermissionKBRead,
	PermissionSkillRead,
	PermissionTemplateRead,
	PermissionAPIKeyRead,
	PermissionTeamRead,
)

var fullAccessPermissions = permissionSet(
	PermissionAppRead,
	PermissionAppUpdate,
	PermissionAppDelete,
	PermissionAppPublish,
	PermissionAppShare,
	PermissionWorkflowCreate,
	PermissionWorkflowRead,
	PermissionWorkflowUpdate,
	PermissionWorkflowDelete,
	PermissionWorkflowExecute,
)

var canEditPermissions = permissionSet(
	PermissionAppRead,
	PermissionAppUpdate,
	PermissionWorkflowCreate,
	PermissionWorkflowRead,
	PermissionWorkflowUpdate,
	PermissionWorkflowExecute,
)

var canViewPermissions = permissionSet(
	PermissionAppRead,
	PermissionWorkflowRead,
)

func AllPermissions() []Permission {
	return append([]Permission(nil), allPermissions...)
}

func RoleHasPermission(role TeamRole, permission Permission) bool {
	switch role {
	case TeamRoleOwner, TeamRoleAdmin:
		return true
	case TeamRoleEditor:
		return editorPermissions[permission]
	case TeamRoleViewer:
		return viewerPermissions[permission]
	default:
		return false
	}
}

func TeamAppHasPermission(grant TeamAppPermission, permission Permission) bool {
	switch grant {
	case TeamAppFullAccess:
		return fullAccessPermissions[permission]
	case TeamAppCanEdit:
		return canEditPermissions[permission]
	case TeamAppCanView:
		return canViewPermissions[permission]
	default:
		return false
	}
}

func permissionSet(values ...Permission) map[Permission]bool {
	result := make(map[Permission]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
