package auth

import (
	"context"
	"heyblog-api/internal/platform/httpapi"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type managedUsersOutput struct{ Body ManagedUsersPage }
type managedMailOutput struct{ Body ManagedMailResult }
type managedUserIDInput struct {
	ID string `path:"id" format:"uuid"`
}

func registerManagedAccountRoutes(api huma.API, service *Service, operation func(string, string, string, string) huma.Operation) {
	httpapi.Register(api, operation("get-managed-user", http.MethodGet, "/api/web/management/users/{id}", "Read managed user details"), func(ctx context.Context, input *managedUserIDInput) (*userOutput, error) {
		actor, err := service.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapError(err)
		}
		user, err := service.ManagedUser(ctx, actor, input.ID)
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{Body: userResponseBody{User: user}}, nil
	})
	httpapi.Register(api, operation("create-managed-user", http.MethodPost, "/api/web/management/users", "Create an account and send a password setup link"), func(ctx context.Context, input *bodyInput[ManagedCreate]) (*managedMailOutput, error) {
		actor, err := service.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapError(err)
		}
		result, err := service.CreateManagedUser(ctx, actor, input.Body)
		if err != nil {
			return nil, mapError(err)
		}
		return &managedMailOutput{Body: result}, nil
	})
	httpapi.Register(api, operation("update-managed-profile", http.MethodPatch, "/api/web/management/users/{id}/profile", "Update account display name and email"), func(ctx context.Context, input *userPathInput[ManagedProfile]) (*userOutput, error) {
		actor, err := service.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapError(err)
		}
		user, err := service.UpdateManagedProfile(ctx, actor, input.ID, input.Body)
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{Body: userResponseBody{User: user}}, nil
	})
	httpapi.Register(api, operation("update-managed-status", http.MethodPatch, "/api/web/management/users/{id}/status", "Activate or suspend an account"), func(ctx context.Context, input *userPathInput[ManagedStatus]) (*userOutput, error) {
		actor, err := service.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapError(err)
		}
		user, err := service.UpdateManagedStatus(ctx, actor, input.ID, input.Body)
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{Body: userResponseBody{User: user}}, nil
	})
	for _, action := range []struct{ id, path, summary string }{
		{"revoke-user-sessions", "sessions/revoke", "Revoke all account sessions"},
		{"send-user-password-link", "password-reset", "Send a password setup or reset link"},
		{"send-user-verification", "verification", "Send an email verification code"},
	} {
		httpapi.Register(api, operation(action.id, http.MethodPost, "/api/web/management/users/{id}/"+action.path, action.summary), func(ctx context.Context, input *userPathInput[ManagedRevision]) (*managedMailOutput, error) {
			actor, err := service.Current(ctx, httpapi.Request(ctx))
			if err != nil {
				return nil, mapError(err)
			}
			result, err := service.ManagedAction(ctx, actor, input.ID, input.Body, action.path)
			if err != nil {
				return nil, mapError(err)
			}
			return &managedMailOutput{Body: result}, nil
		})
	}
}
