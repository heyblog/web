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

func (service *Service) ListManagedUsers(ctx context.Context, requestActor User) ([]User, error) {
	if !canManageUsers(requestActor) {
		return nil, newAuthError("forbidden", 403, "user management permission is required")
	}
	records, err := service.repo.listUsers(ctx)
	if err != nil {
		return nil, err
	}
	users := make([]User, 0, len(records))
	for _, record := range records {
		user, err := service.toUser(ctx, record)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (service *Service) UpdateRole(ctx context.Context, actor User, targetID string, role Role) (User, error) {
	if actor.Role != RoleSysAdmin {
		return User{}, newAuthError("forbidden", 403, "system administrator role is required")
	}
	if actor.ID == targetID {
		return User{}, newAuthError("self_management_forbidden", 409, "you cannot change your own role")
	}
	if role != RoleUser && role != RoleAdmin {
		return User{}, newAuthError("invalid_role", 422, "role is invalid")
	}
	if err := service.repo.setRole(ctx, targetID, string(role)); err != nil {
		return User{}, err
	}
	record, err := service.repo.userByID(ctx, targetID)
	if err != nil {
		return User{}, err
	}
	return service.toUser(ctx, record)
}

func (service *Service) UpdatePermissions(ctx context.Context, actor User, targetID string, permissions []Permission) (User, error) {
	if !canManageUsers(actor) {
		return User{}, newAuthError("forbidden", 403, "user management permission is required")
	}
	if actor.ID == targetID {
		return User{}, newAuthError("self_management_forbidden", 409, "you cannot change your own permissions")
	}
	target, err := service.repo.userByID(ctx, targetID)
	if err != nil {
		return User{}, err
	}
	if Role(target.Role) != RoleAdmin {
		return User{}, newAuthError("invalid_permission_target", 409, "only administrators can receive management permissions")
	}
	seen := make(map[Permission]struct{}, len(permissions))
	for _, permission := range permissions {
		if !slices.Contains(validPermissions, permission) {
			return User{}, newAuthError("invalid_permission", 422, "permission is invalid")
		}
		if _, exists := seen[permission]; exists {
			return User{}, newAuthError("duplicate_permission", 422, "permission is duplicated")
		}
		seen[permission] = struct{}{}
	}
	if actor.Role != RoleSysAdmin {
		for _, permission := range permissions {
			if !slices.Contains(actor.Permissions, permission) {
				return User{}, newAuthError("permission_scope_exceeded", 403, "permission is outside your authorization scope")
			}
		}
	}
	if err := service.repo.replacePermissions(ctx, targetID, actor.ID, permissions); err != nil {
		return User{}, err
	}
	updated, err := service.repo.userByID(ctx, targetID)
	if err != nil {
		return User{}, err
	}
	return service.toUser(ctx, updated)
}

func canManageUsers(user User) bool {
	return user.Role == RoleSysAdmin || (user.Role == RoleAdmin && slices.Contains(user.Permissions, PermissionUserManage))
}
