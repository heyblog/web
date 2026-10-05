package sluggeneration

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/httpapi"
)

type response[T any] struct {
	CacheControl string `header:"Cache-Control"`
	Body         T
}
type generationInput struct{ Body SlugGenerationInput }
type settingsInput struct{ Body SaveSettingsInput }

func RegisterRoutes(api huma.API, service *Service, webToken string) error {
	if service == nil {
		return errors.New("slug generation service is required")
	}
	operation := func(id, method, path string, systemOnly bool) huma.Operation {
		return huma.Operation{OperationID: id, Method: method, Path: path, Tags: []string{"taxonomy"}, MaxBodyBytes: 8192,
			Security: []map[string][]string{{"webToken": {}, "accessCookie": {}}}, Errors: []int{401, 403, 409, 413, 422, 429, 503},
			Middlewares: huma.Middlewares{httpapi.HumaWebAuthorization(webToken), service.authorization(systemOnly)}}
	}
	httpapi.Register(api, operation("generate-tag-slug", http.MethodPost, "/management/taxonomy/slug-generation", false), func(ctx context.Context, input *generationInput) (*response[Result], error) {
		identity, _ := ctx.Value(identityKey{}).(Identity)
		result, err := service.Generate(ctx, identity, input.Body)
		if err != nil {
			setRetryHeader(ctx, err)
			return nil, err
		}
		return &response[Result]{CacheControl: "private, no-store", Body: result}, nil
	})
	httpapi.Register(api, operation("get-system-settings", http.MethodGet, "/management/system-settings", true), func(ctx context.Context, _ *struct{}) (*response[Settings], error) {
		settings, err := service.Settings(ctx)
		if err != nil {
			return nil, err
		}
		return &response[Settings]{CacheControl: "private, no-store", Body: settings}, nil
	})
	httpapi.Register(api, operation("save-system-settings", http.MethodPut, "/management/system-settings", true), func(ctx context.Context, input *settingsInput) (*response[Settings], error) {
		identity, _ := ctx.Value(identityKey{}).(Identity)
		if err := service.guard.Allow(ctx, identity); err != nil {
			setRetryHeader(ctx, err)
			return nil, err
		}
		settings, err := service.SaveSettings(ctx, input.Body)
		if err != nil {
			return nil, err
		}
		return &response[Settings]{CacheControl: "private, no-store", Body: settings}, nil
	})
	httpapi.Register(api, operation("list-slug-models", http.MethodGet, "/management/system-settings/models", true), func(ctx context.Context, _ *struct{}) (*response[Models], error) {
		identity, _ := ctx.Value(identityKey{}).(Identity)
		if err := service.guard.Allow(ctx, identity); err != nil {
			setRetryHeader(ctx, err)
			return nil, err
		}
		models, err := service.Models(ctx)
		if err != nil {
			return nil, err
		}
		return &response[Models]{CacheControl: "private, no-store", Body: models}, nil
	})
	return service.registerJobRoutes(api, webToken)
}

type identityKey struct{}

func (service *Service) authorization(systemOnly bool) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		user, err := service.auth.Current(ctx.Context(), httpapi.Request(ctx.Context()))
		if err != nil {
			var failure *auth.AuthError
			if errors.As(err, &failure) {
				kind := apperror.KindUnauthorized
				if failure.StatusCode == http.StatusForbidden {
					kind = apperror.KindForbidden
				}
				httpapi.RejectHumaRequest(ctx, apperror.New(kind, failure.Code, failure.Message))
			} else {
				httpapi.RejectHumaRequest(ctx, unavailable())
			}
			return
		}
		allowed := user.Role == auth.RoleSysAdmin || (!systemOnly && user.Role == auth.RoleAdmin && slices.Contains(user.Permissions, auth.PermissionTaxonomyManage))
		if !allowed {
			httpapi.RejectHumaRequest(ctx, apperror.New(apperror.KindForbidden, "forbidden", "management permission is required"))
			return
		}
		ip := httpapi.NativeContext(ctx.Context()).ClientIP()
		ctx.SetHeader("Cache-Control", "private, no-store")
		next(huma.WithValue(ctx, identityKey{}, Identity{UserID: user.ID, IP: ip, SystemAdmin: user.Role == auth.RoleSysAdmin}))
	}
}

func setRetryHeader(ctx context.Context, err error) {
	if retry := retryAfter(err); retry != "" {
		httpapi.NativeContext(ctx).Header("Retry-After", retry)
	}
}
