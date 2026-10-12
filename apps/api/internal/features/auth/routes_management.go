package auth

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/platform/httpapi"
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

	httpapi.Register(api, operation("list-managed-users", http.MethodGet, "/api/web/management/users", "List managed users"), func(ctx context.Context, input *ManagedUsersFilter) (*managedUsersOutput, error) {
		actor, err := service.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapError(err)
		}
		users, err := service.ManagedUsers(ctx, actor, *input)
		if err != nil {
			return nil, mapError(err)
		}
		return &managedUsersOutput{Body: users}, nil
	})

	httpapi.Register(api, operation("update-user-role", http.MethodPatch, "/api/web/management/users/{id}/role", "Update a user's role"), func(ctx context.Context, input *userPathInput[roleRequest]) (*userOutput, error) {
		actor, err := service.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapError(err)
		}
		user, err := service.updateManagedAuthorization(ctx, managedMutation{Actor: actor, ID: input.ID, Kind: "role", Role: input.Body.Role, UpdatedAt: input.Body.UpdatedAt})
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{Body: userResponseBody{User: user}}, nil
	})

	httpapi.Register(api, operation("update-user-permissions", http.MethodPatch, "/api/web/management/users/{id}/permissions", "Update a user's permissions"), func(ctx context.Context, input *userPathInput[permissionsRequest]) (*userOutput, error) {
		actor, err := service.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapError(err)
		}
		user, err := service.updateManagedAuthorization(ctx, managedMutation{Actor: actor, ID: input.ID, Kind: "permissions", Permissions: input.Body.Permissions, UpdatedAt: input.Body.UpdatedAt})
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{Body: userResponseBody{User: user}}, nil
	})
	registerManagedAccountRoutes(api, service, operation)
}
