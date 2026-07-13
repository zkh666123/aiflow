package rbac

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrResourceNotFound = errors.New("resource not found")
	ErrForbidden        = errors.New("forbidden")
)

type ApplicationGrant struct {
	Role       TeamRole
	Permission TeamAppPermission
}

type ApplicationAccess struct {
	GlobalRole GlobalRole
	OwnerID    string
	Grants     []ApplicationGrant
}

type TeamAccess struct {
	GlobalRole GlobalRole
	OwnerID    string
	Role       TeamRole
	Member     bool
}

type AccessStore interface {
	ApplicationAccess(context.Context, string, string) (ApplicationAccess, error)
	TeamAccess(context.Context, string, string) (TeamAccess, error)
}

type Authorizer struct {
	store AccessStore
}

func NewAuthorizer(store AccessStore) *Authorizer {
	return &Authorizer{store: store}
}

func (authorizer *Authorizer) AuthorizeApplication(
	ctx context.Context,
	userID string,
	applicationID string,
	permissions ...Permission,
) error {
	access, err := authorizer.store.ApplicationAccess(ctx, userID, applicationID)
	if err != nil {
		return fmt.Errorf("resolve application access: %w", err)
	}
	if access.GlobalRole == GlobalRoleAdmin || access.OwnerID == userID {
		return nil
	}
	for _, permission := range permissions {
		allowed := false
		for _, grant := range access.Grants {
			if RoleHasPermission(grant.Role, permission) || TeamAppHasPermission(grant.Permission, permission) {
				allowed = true
				break
			}
		}
		if !allowed {
			return ErrForbidden
		}
	}
	return nil
}

func (authorizer *Authorizer) AuthorizeTeam(
	ctx context.Context,
	userID string,
	teamID string,
	permissions ...Permission,
) error {
	access, err := authorizer.store.TeamAccess(ctx, userID, teamID)
	if err != nil {
		return fmt.Errorf("resolve team access: %w", err)
	}
	if access.GlobalRole == GlobalRoleAdmin || access.OwnerID == userID {
		return nil
	}
	if !access.Member {
		return ErrForbidden
	}
	for _, permission := range permissions {
		if !RoleHasPermission(access.Role, permission) {
			return ErrForbidden
		}
	}
	return nil
}
