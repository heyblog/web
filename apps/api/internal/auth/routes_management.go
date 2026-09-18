package auth

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/httpapi"
)

func registerManagementRoutes(api huma.API, service *Service, build authOperationBuilder) {
	operation := func(id, method, path, summary string) huma.Operation {
		return build(huma.Operation{
			OperationID: id,
			Method:      method,
			Path:        path,
			Summary:     summary,
			Tags:        []string{"user management"},
			Errors: []int{
				http.StatusBadRequest,
				http.StatusUnauthorized,
				http.StatusForbidden,
				http.StatusNotFound,
				http.StatusConflict,
				http.StatusUnprocessableEntity,
				http.StatusServiceUnavailable,
			},
			Security: []map[string][]string{{"webToken": {}, "accessCookie": {}}},
		})
	}

	httpapi.Register(api, operation("list-managed-users", http.MethodGet, "/management/users", "List managed users"), func(ctx context.Context, _ *emptyInput) (*usersOutput, error) {
		actor, err := service.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapError(err)
		}
		users, err := service.ListManagedUsers(ctx, actor)
		if err != nil {
			return nil, mapError(err)
		}
		return &usersOutput{Body: usersResponseBody{Users: users}}, nil
	})

	httpapi.Register(api, operation("update-user-role", http.MethodPatch, "/management/users/{id}/role", "Update a user's role"), func(ctx context.Context, input *userPathInput[roleRequest]) (*userOutput, error) {
		actor, err := service.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapError(err)
		}
		user, err := service.UpdateRole(ctx, actor, input.ID, input.Body.Role)
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{Body: userResponseBody{User: user}}, nil
	})

	httpapi.Register(api, operation("update-user-permissions", http.MethodPatch, "/management/users/{id}/permissions", "Update a user's permissions"), func(ctx context.Context, input *userPathInput[permissionsRequest]) (*userOutput, error) {
		actor, err := service.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapError(err)
		}
		user, err := service.UpdatePermissions(ctx, actor, input.ID, input.Body.Permissions)
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{Body: userResponseBody{User: user}}, nil
	})
}
