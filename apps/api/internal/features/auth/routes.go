package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/platform/httpapi"
	"heyblog-api/internal/platform/ratelimit"
)

type authOperationBuilder func(huma.Operation) huma.Operation

func RegisterRoutes(api huma.API, service *Service, webToken string) error {
	if service == nil {
		return errors.New("auth service is required")
	}
	limiter := ratelimit.New(service.redis)
	policies := map[string]ratelimit.Policy{
		"/api/web/management/users/{id}/password-reset": {Name: "management-password-mail", Capacity: 3, RefillTokens: 3, RefillInterval: 10 * time.Minute},
		"/api/web/management/users/{id}/verification":   {Name: "management-verification-mail", Capacity: 3, RefillTokens: 3, RefillInterval: 10 * time.Minute},
		"/api/web/auth/login":                           {Name: "auth-login", Capacity: 10, RefillTokens: 10, RefillInterval: time.Minute},
		"/api/web/auth/register":                        {Name: "auth-register", Capacity: 5, RefillTokens: 5, RefillInterval: 10 * time.Minute},
		"/api/web/auth/verify-email":                    {Name: "auth-verify", Capacity: 10, RefillTokens: 10, RefillInterval: 10 * time.Minute},
		"/api/web/auth/verify-email/resend":             {Name: "auth-resend", Capacity: 3, RefillTokens: 3, RefillInterval: 10 * time.Minute},
		"/api/web/auth/password/forgot":                 {Name: "auth-forgot", Capacity: 3, RefillTokens: 3, RefillInterval: 10 * time.Minute},
		"/api/web/auth/password/reset":                  {Name: "auth-reset", Capacity: 5, RefillTokens: 5, RefillInterval: 10 * time.Minute},
		"/api/web/auth/refresh":                         {Name: "auth-refresh", Capacity: 30, RefillTokens: 30, RefillInterval: time.Minute},
		"/api/web/auth/blogsclub/start":                 {Name: "auth-blogsclub", Capacity: 10, RefillTokens: 10, RefillInterval: time.Minute},
		"/api/web/auth/github/start":                    {Name: "auth-github", Capacity: 10, RefillTokens: 10, RefillInterval: time.Minute},
	}
	build := func(operation huma.Operation) huma.Operation {
		if operation.Security == nil {
			operation.Security = []map[string][]string{{"webToken": {}}}
		}
		operation.Middlewares = huma.Middlewares{httpapi.HumaWebAuthorization(webToken)}
		if policy, exists := policies[operation.Path]; exists {
			operation.Middlewares = append(operation.Middlewares, httpapi.HumaRateLimit(limiter, policy))
		}
		if operation.MaxBodyBytes == 0 && operation.Method != http.MethodGet {
			operation.MaxBodyBytes = 64 << 10
		}
		return operation
	}
	registerSessionRoutes(api, service, limiter, build)
	registerOAuthRoutes(api, service, build)
	registerManagementRoutes(api, service, build)
	registerProfileRoutes(api, service, build)
	return nil
}
