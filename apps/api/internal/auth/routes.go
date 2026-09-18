package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/httpapi"
	"heyblog-api/internal/ratelimit"
)

type authOperationBuilder func(huma.Operation) huma.Operation

func RegisterRoutes(api huma.API, service *Service, webToken string) error {
	if service == nil {
		return errors.New("auth service is required")
	}
	limiter := ratelimit.New(service.redis)
	policies := map[string]ratelimit.Policy{
		"/auth/login":               {Name: "auth-login", Capacity: 10, RefillTokens: 10, RefillInterval: time.Minute},
		"/auth/register":            {Name: "auth-register", Capacity: 5, RefillTokens: 5, RefillInterval: 10 * time.Minute},
		"/auth/verify-email":        {Name: "auth-verify", Capacity: 10, RefillTokens: 10, RefillInterval: 10 * time.Minute},
		"/auth/verify-email/resend": {Name: "auth-resend", Capacity: 3, RefillTokens: 3, RefillInterval: 10 * time.Minute},
		"/auth/password/forgot":     {Name: "auth-forgot", Capacity: 3, RefillTokens: 3, RefillInterval: 10 * time.Minute},
		"/auth/password/reset":      {Name: "auth-reset", Capacity: 5, RefillTokens: 5, RefillInterval: 10 * time.Minute},
		"/auth/refresh":             {Name: "auth-refresh", Capacity: 30, RefillTokens: 30, RefillInterval: time.Minute},
		"/auth/github/start":        {Name: "auth-github", Capacity: 10, RefillTokens: 10, RefillInterval: time.Minute},
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
	registerGithubRoutes(api, service, build)
	registerManagementRoutes(api, service, build)
	return nil
}
