package auth

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/platform/httpapi"
)

type oauthStartInput struct {
	Next    string `query:"next"`
	Intent  string `query:"intent" enum:"login,bind,site-claim"`
	Mode    string `query:"mode" enum:"popup,redirect"`
	ClaimID string `query:"claim_id"`
	Accept  string `header:"Accept"`
}

type oauthCallbackInput struct {
	Code   string `query:"code"`
	State  string `query:"state"`
	Error  string `query:"error"`
	Accept string `header:"Accept"`
}

type oauthOutput[T any] struct {
	Status       int
	Location     string   `header:"Location"`
	CacheControl string   `header:"Cache-Control"`
	SetCookie    []string `header:"Set-Cookie"`
	Body         T
}

func oauthStateCookieName(provider OAuthProvider, state string) string {
	return "heyblog_oauth_" + provider.slug() + "_" + state
}

func registerOAuthRoutes(api huma.API, service *Service, build authOperationBuilder) {
	for _, provider := range []OAuthProvider{ProviderGitHub, ProviderBlogsClub} {
		operation := func(id, method, suffix string) huma.Operation {
			return build(huma.Operation{OperationID: id + "-" + provider.slug(), Method: method, Path: "/api/web/auth/" + provider.slug() + "/" + suffix, Summary: id + " " + provider.slug() + " authentication", Tags: []string{"authentication"}, Errors: []int{400, 401, 403, 404, 409, 422, 429, 503}})
		}
		httpapi.Register(api, operation("start-oauth", http.MethodGet, "start"), func(ctx context.Context, input *oauthStartInput) (*oauthOutput[OAuthStartResult], error) {
			result, err := service.OAuthStart(ctx, httpapi.Request(ctx), provider, OAuthStartOptions{Next: input.Next, Intent: input.Intent, Mode: input.Mode, ClaimID: input.ClaimID})
			if err != nil {
				return nil, mapError(err)
			}
			output := &oauthOutput[OAuthStartResult]{Status: 200, CacheControl: "no-store", Body: result, SetCookie: []string{authCookie(service.config, oauthStateCookieName(provider, result.State), result.State, "/", 600, time.Now().Add(oauthStateTTL)).String()}}
			if !strings.Contains(input.Accept, "application/json") {
				output.Status = 302
				output.Location = result.AuthorizationURL
			}
			return output, nil
		})
		httpapi.Register(api, operation("complete-oauth", http.MethodGet, "callback"), func(ctx context.Context, input *oauthCallbackInput) (*oauthOutput[OAuthResult], error) {
			request := httpapi.Request(ctx)
			name := oauthStateCookieName(provider, input.State)
			cookie := ""
			if stateCookie, err := request.Cookie(name); err == nil {
				cookie = stateCookie.Value
			}
			if len(input.State) <= 128 {
				httpapi.NativeContext(ctx).Writer.Header().Add("Set-Cookie", authCookie(service.config, name, "", "/", -1, time.Unix(1, 0)).String())
			}
			result, cookies, err := service.OAuthCallback(ctx, request, provider, input.Code, input.State, cookie, input.Error)
			if err != nil {
				return nil, mapError(err)
			}
			output := &oauthOutput[OAuthResult]{Status: 200, CacheControl: "no-store", Body: result, SetCookie: cookieValues(cookies, service.config)}
			if !strings.Contains(input.Accept, "application/json") {
				next := result.Next
				if result.Outcome == "error" {
					parsed, _ := url.Parse(next)
					query := parsed.Query()
					query.Set("error", result.ErrorCode)
					parsed.RawQuery = query.Encode()
					next = parsed.String()
				}
				output.Status = 302
				output.Location = strings.TrimRight(service.config.WebBaseURL, "/") + next
			}
			return output, nil
		})
		unbind := operation("unbind-oauth", http.MethodPost, "unbind")
		unbind.Security = []map[string][]string{{"webToken": {}, "accessCookie": {}}}
		httpapi.Register(api, unbind, func(ctx context.Context, _ *emptyInput) (*userOutput, error) {
			user, cookies, err := service.OAuthUnbind(ctx, httpapi.Request(ctx), provider)
			if err != nil {
				return nil, mapError(err)
			}
			return &userOutput{CacheControl: "no-store", SetCookie: cookieValues(cookies, service.config), Body: userResponseBody{User: user}}, nil
		})
	}
}
