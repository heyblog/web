package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/httpapi"
)

func registerGithubRoutes(api huma.API, service *Service, build authOperationBuilder) {
	operation := func(id, method, path, summary string) huma.Operation {
		return build(huma.Operation{
			OperationID: id,
			Method:      method,
			Path:        path,
			Summary:     summary,
			Tags:        []string{"authentication"},
			Errors: []int{
				http.StatusBadRequest,
				http.StatusUnauthorized,
				http.StatusForbidden,
				http.StatusConflict,
				http.StatusUnprocessableEntity,
				http.StatusTooManyRequests,
				http.StatusServiceUnavailable,
			},
		})
	}

	startOperation := operation("start-github-auth", http.MethodGet, "/auth/github/start", "Start GitHub authentication")
	startOperation.DefaultStatus = http.StatusFound
	httpapi.Register(api, startOperation, func(ctx context.Context, input *githubStartInput) (*redirectOutput, error) {
		target, state, err := service.GithubStart(ctx, input.Next, input.Intent == "bind")
		if err != nil {
			return nil, mapError(err)
		}
		stateCookie := authCookie(service.config, githubStateCookieName(state), state, "/auth/github", 600, time.Now().Add(10*time.Minute))
		return &redirectOutput{
			Status:       http.StatusFound,
			Location:     target,
			CacheControl: "no-store",
			SetCookie:    []string{stateCookie.String()},
		}, nil
	})

	callbackOperation := operation("complete-github-auth", http.MethodGet, "/auth/github/callback", "Complete GitHub authentication")
	callbackOperation.DefaultStatus = http.StatusFound
	httpapi.Register(api, callbackOperation, func(ctx context.Context, input *githubCallbackInput) (*redirectOutput, error) {
		request := httpapi.Request(ctx)
		cookieValue, legacyCookie := readGithubStateCookie(request, input.State)
		_, cookies, next, err := service.GithubCallback(ctx, request, input.Code, input.State, cookieValue)
		if err != nil {
			return nil, mapError(err)
		}
		setCookies := []string{authCookie(
			service.config,
			githubStateCookieName(input.State),
			"",
			"/auth/github",
			-1,
			time.Unix(1, 0),
		).String()}
		if legacyCookie {
			setCookies = append(setCookies, authCookie(
				service.config,
				legacyGithubStateCookieName,
				"",
				"/auth/github",
				-1,
				time.Unix(1, 0),
			).String())
		}
		setCookies = append(setCookies, cookieValues(cookies, service.config)...)
		return &redirectOutput{
			Status:       http.StatusFound,
			Location:     strings.TrimRight(service.config.WebBaseURL, "/") + next,
			CacheControl: "no-store",
			SetCookie:    setCookies,
		}, nil
	})

	unbindOperation := operation("unbind-github", http.MethodPost, "/auth/github/unbind", "Unbind GitHub authentication")
	unbindOperation.Security = []map[string][]string{{"webToken": {}, "accessCookie": {}}}
	httpapi.Register(api, unbindOperation, func(ctx context.Context, _ *emptyInput) (*userOutput, error) {
		user, cookies, err := service.GithubUnbind(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{
			CacheControl: "no-store",
			SetCookie:    cookieValues(cookies, service.config),
			Body:         userResponseBody{User: user},
		}, nil
	})
}
