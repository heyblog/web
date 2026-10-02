package siteaudit

import (
	"context"
	"net/http"

	"heyblog-api/internal/features/auth"
)

func canReview(user auth.User) bool {
	return user.Role == auth.RoleSysAdmin || user.Role == auth.RoleAdmin && slicesContains(user.Permissions, auth.PermissionSiteAuditReview)
}

func canManageTaxonomy(user auth.User) bool {
	return user.Role == auth.RoleSysAdmin || user.Role == auth.RoleAdmin && slicesContains(user.Permissions, auth.PermissionTaxonomyManage)
}

func slicesContains(values []auth.Permission, expected auth.Permission) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func (service *Service) CurrentReviewer(ctx context.Context, request *http.Request) (auth.User, error) {
	user, err := service.auth.Current(ctx, request)
	if err != nil {
		return auth.User{}, err
	}
	if !canReview(user) {
		return auth.User{}, newServiceError("forbidden", http.StatusForbidden, "site audit review permission is required")
	}
	return user, nil
}
