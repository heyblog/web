package announcement

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/apperror"
)

type Authenticator interface {
	Current(context.Context, *http.Request) (auth.User, error)
}
type Service struct {
	repository *Repository
	auth       Authenticator
}

func NewService(repository *Repository, authentication Authenticator) *Service {
	return &Service{repository: repository, auth: authentication}
}

func canManage(user auth.User) bool {
	return user.Role == auth.RoleSysAdmin || user.Role == auth.RoleAdmin && slices.Contains(user.Permissions, auth.PermissionAnnouncementManage)
}

func (service *Service) authorize(ctx context.Context, request *http.Request) (auth.User, error) {
	user, err := service.auth.Current(ctx, request)
	if err != nil {
		var failure *auth.AuthError
		if errors.As(err, &failure) {
			kind := apperror.KindUnauthorized
			if failure.StatusCode == http.StatusForbidden {
				kind = apperror.KindForbidden
			}
			return auth.User{}, apperror.New(kind, failure.Code, failure.Message)
		}
		return auth.User{}, apperror.Wrap(err, apperror.KindInternal, "internal_error", "authentication service is unavailable", "authorize announcement management")
	}
	if !canManage(user) {
		return auth.User{}, apperror.New(apperror.KindForbidden, "forbidden", "announcement management permission is required")
	}
	return user, nil
}
