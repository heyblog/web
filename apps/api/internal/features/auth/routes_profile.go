package auth

import (
	"context"
	"net/http"

	"heyblog-api/internal/platform/httpapi"

	"github.com/danielgtaylor/huma/v2"
)

func registerProfileRoutes(api huma.API, service *Service, build authOperationBuilder) {
	operation := func(id, method, path, summary string) huma.Operation {
		return build(huma.Operation{OperationID: id, Method: method, Path: path, Summary: summary, Tags: []string{"account profile"}, Security: []map[string][]string{{"webToken": {}, "accessCookie": {}}}, Errors: []int{400, 401, 403, 409, 422, 503}})
	}
	httpapi.Register(api, operation("confirm-account-profile", http.MethodPost, "/api/web/account/profile/confirm", "Confirm the initial account profile"), func(ctx context.Context, input *bodyInput[ProfileConfirmation]) (*userOutput, error) {
		user, cookies, err := service.ConfirmProfile(ctx, httpapi.Request(ctx), input.Body)
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{CacheControl: "no-store", SetCookie: cookieValues(cookies, service.config), Body: userResponseBody{User: user}}, nil
	})
	httpapi.Register(api, operation("update-account-profile", http.MethodPatch, "/api/web/account/profile", "Update the display name"), func(ctx context.Context, input *bodyInput[DisplayProfile]) (*userOutput, error) {
		user, err := service.ChangeDisplayName(ctx, httpapi.Request(ctx), input.Body.DisplayName)
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{CacheControl: "no-store", Body: userResponseBody{User: user}}, nil
	})
	httpapi.Register(api, operation("change-account-username", http.MethodPost, "/api/web/account/username", "Use the optional username change"), func(ctx context.Context, input *bodyInput[UsernameChange]) (*userOutput, error) {
		user, cookies, err := service.ChangeUsername(ctx, httpapi.Request(ctx), input.Body.Username)
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{CacheControl: "no-store", SetCookie: cookieValues(cookies, service.config), Body: userResponseBody{User: user}}, nil
	})
}
