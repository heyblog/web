package auth

import (
	"context"
	"slices"
)

var validPermissions = []Permission{
	PermissionUserManage, PermissionSiteAuditReview, PermissionFeedbackReview,
	PermissionAnnouncementManage, PermissionTaxonomyManage, PermissionSiteManage,
	PermissionTaskManage, PermissionLogRead,
}

func (service *Service) UpdateRole(ctx context.Context, actor User, targetID string, role Role) (User, error) {
	return service.updateManagedAuthorization(ctx, managedMutation{Actor: actor, ID: targetID, Kind: "role", Role: role})
}

func (service *Service) UpdatePermissions(ctx context.Context, actor User, targetID string, permissions []Permission) (User, error) {
	return service.updateManagedAuthorization(ctx, managedMutation{Actor: actor, ID: targetID, Kind: "permissions", Permissions: permissions})
}

func (service *Service) updateManagedAuthorization(ctx context.Context, change managedMutation) (User, error) {
	if !canManageUsers(change.Actor) {
		return User{}, newAuthError("forbidden", 403, "user management permission is required")
	}
	if change.Kind == "role" && change.Actor.Role != RoleSysAdmin {
		return User{}, newAuthError("forbidden", 403, "system administrator role is required")
	}
	record, err := service.repo.mutateManagedUser(ctx, change)
	if err != nil {
		return User{}, err
	}
	return service.toUser(ctx, record)
}

func validatePermissions(actor User, target dbUser, permissions []Permission) error {
	if Role(target.Role) != RoleAdmin {
		return newAuthError("invalid_permission_target", 409, "only administrators can receive management permissions")
	}
	seen := make(map[Permission]struct{}, len(permissions))
	for _, permission := range permissions {
		if !slices.Contains(validPermissions, permission) {
			return newAuthError("invalid_permission", 422, "permission is invalid")
		}
		if _, ok := seen[permission]; ok {
			return newAuthError("duplicate_permission", 422, "permission is duplicated")
		}
		seen[permission] = struct{}{}
		if actor.Role != RoleSysAdmin && !slices.Contains(actor.Permissions, permission) {
			return newAuthError("permission_scope_exceeded", 403, "permission is outside your authorization scope")
		}
	}
	return nil
}

func canManageUsers(user User) bool {
	return user.Role == RoleSysAdmin || (user.Role == RoleAdmin && slices.Contains(user.Permissions, PermissionUserManage))
}
