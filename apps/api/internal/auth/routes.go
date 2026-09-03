package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"heyblog-api/internal/httpapi"
	"heyblog-api/internal/ratelimit"
)

func RegisterRoutes(router *gin.Engine, service *Service, webToken string) error {
	if service == nil {
		return errors.New("auth service is required")
	}
	guard := httpapi.WebAuthorization(webToken)
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
	register := func(method, path string, endpoint httpapi.Endpoint) {
		middleware := []httpapi.Middleware{guard}
		if policy, exists := policies[path]; exists {
			middleware = append(middleware, httpapi.RateLimit(limiter, policy))
		}
		handler := httpapi.Adapt(httpapi.Chain(endpoint, middleware...))
		switch method {
		case http.MethodGet:
			router.GET(path, handler)
		case http.MethodPost:
			router.POST(path, handler)
		case http.MethodPatch:
			router.PATCH(path, handler)
		}
	}
	register(http.MethodPost, "/auth/register", func(ctx *httpapi.Context) (httpapi.Response, error) {
		var payload registerRequest
		if err := decodeJSON(ctx.Request, &payload); err != nil {
			return httpapi.Response{}, err
		}
		if err := enforceMailRequest(ctx, limiter, payload.Email); err != nil {
			return httpapi.Response{}, err
		}
		if err := service.Register(ctx.Request.Context(), payload.Username, payload.Email, payload.Password); err != nil {
			return httpapi.Response{}, mapError(err)
		}
		return httpapi.JSON(http.StatusAccepted, map[string]string{"status": "verification_required"})
	})
	register(http.MethodPost, "/auth/login", func(ctx *httpapi.Context) (httpapi.Response, error) {
		var payload loginRequest
		if err := decodeJSON(ctx.Request, &payload); err != nil {
			return httpapi.Response{}, err
		}
		user, cookies, err := service.Login(ctx.Request.Context(), payload.Identifier, payload.Password)
		if err != nil {
			return httpapi.Response{}, mapError(err)
		}
		response, responseErr := httpapi.JSON(http.StatusOK, map[string]User{"user": user})
		if responseErr != nil {
			return httpapi.Response{}, responseErr
		}
		return addCookies(response, cookies, service.config), nil
	})
	register(http.MethodGet, "/auth/me", func(ctx *httpapi.Context) (httpapi.Response, error) {
		user, err := service.Current(ctx.Request.Context(), ctx.Request)
		if err != nil {
			return httpapi.Response{}, mapError(err)
		}
		return httpapi.JSON(http.StatusOK, map[string]User{"user": user})
	})
	register(http.MethodPost, "/auth/refresh", func(ctx *httpapi.Context) (httpapi.Response, error) {
		user, cookies, err := service.Refresh(ctx.Request.Context(), ctx.Request)
		if err != nil {
			return httpapi.Response{}, mapError(err)
		}
		response, responseErr := httpapi.JSON(http.StatusOK, map[string]User{"user": user})
		if responseErr != nil {
			return httpapi.Response{}, responseErr
		}
		return addCookies(response, cookies, service.config), nil
	})
	register(http.MethodPost, "/auth/logout", func(ctx *httpapi.Context) (httpapi.Response, error) {
		if err := service.Logout(ctx.Request.Context(), ctx.Request); err != nil {
			return httpapi.Response{}, mapError(err)
		}
		response := httpapi.NoContent(http.StatusNoContent)
		return clearCookies(response, service.config), nil
	})
	register(http.MethodPost, "/auth/verify-email", func(ctx *httpapi.Context) (httpapi.Response, error) {
		var payload verifyRequest
		if err := decodeJSON(ctx.Request, &payload); err != nil {
			return httpapi.Response{}, err
		}
		if err := service.VerifyEmail(ctx.Request.Context(), payload.Email, payload.Code); err != nil {
			return httpapi.Response{}, mapError(err)
		}
		return httpapi.NoContent(http.StatusNoContent), nil
	})
	register(http.MethodPost, "/auth/verify-email/resend", func(ctx *httpapi.Context) (httpapi.Response, error) {
		var payload emailRequest
		if err := decodeJSON(ctx.Request, &payload); err != nil {
			return httpapi.Response{}, err
		}
		if err := enforceMailRequest(ctx, limiter, payload.Email); err != nil {
			return httpapi.Response{}, err
		}
		if err := service.ResendVerification(ctx.Request.Context(), payload.Email); err != nil {
			return httpapi.Response{}, mapError(err)
		}
		return httpapi.NoContent(http.StatusNoContent), nil
	})
	register(http.MethodPost, "/auth/password/forgot", func(ctx *httpapi.Context) (httpapi.Response, error) {
		var payload emailRequest
		if err := decodeJSON(ctx.Request, &payload); err != nil {
			return httpapi.Response{}, err
		}
		if err := enforceMailRequest(ctx, limiter, payload.Email); err != nil {
			return httpapi.Response{}, err
		}
		if err := service.ForgotPassword(ctx.Request.Context(), payload.Email); err != nil {
			return httpapi.Response{}, mapError(err)
		}
		return httpapi.NoContent(http.StatusNoContent), nil
	})
	register(http.MethodPost, "/auth/password/reset", func(ctx *httpapi.Context) (httpapi.Response, error) {
		var payload resetRequest
		if err := decodeJSON(ctx.Request, &payload); err != nil {
			return httpapi.Response{}, err
		}
		if err := service.ResetPassword(ctx.Request.Context(), payload.Token, payload.Password); err != nil {
			return httpapi.Response{}, mapError(err)
		}
		return httpapi.NoContent(http.StatusNoContent), nil
	})
	register(http.MethodPost, "/auth/password", func(ctx *httpapi.Context) (httpapi.Response, error) {
		var payload setPasswordRequest
		if err := decodeJSON(ctx.Request, &payload); err != nil {
			return httpapi.Response{}, err
		}
		user, cookies, err := service.SetPassword(ctx.Request.Context(), ctx.Request, payload.CurrentPassword, payload.NextPassword)
		if err != nil {
			return httpapi.Response{}, mapError(err)
		}
		response, responseErr := httpapi.JSON(http.StatusOK, map[string]User{"user": user})
		if responseErr != nil {
			return httpapi.Response{}, responseErr
		}
		return addCookies(response, cookies, service.config), nil
	})
	register(http.MethodGet, "/auth/github/start", func(ctx *httpapi.Context) (httpapi.Response, error) {
		next := ctx.Request.URL.Query().Get("next")
		bind := ctx.Request.URL.Query().Get("intent") == "bind"
		target, state, err := service.GithubStart(ctx.Request.Context(), next, bind)
		if err != nil {
			return httpapi.Response{}, mapError(err)
		}
		stateCookie := authCookie(service.config, "heyblog_github_state", state, "/auth/github", 600, time.Now().Add(10*time.Minute))
		response := httpapi.NoContent(http.StatusFound).WithHeader("Location", target).WithHeader("Set-Cookie", stateCookie.String())
		return response.WithHeader("Cache-Control", "no-store"), nil
	})
	register(http.MethodGet, "/auth/github/callback", func(ctx *httpapi.Context) (httpapi.Response, error) {
		stateCookie, _ := ctx.Request.Cookie("heyblog_github_state")
		cookieValue := ""
		if stateCookie != nil {
			cookieValue = stateCookie.Value
		}
		_, cookies, next, err := service.GithubCallback(ctx.Request.Context(), ctx.Request, ctx.Request.URL.Query().Get("code"), ctx.Request.URL.Query().Get("state"), cookieValue)
		if err != nil {
			return httpapi.Response{}, mapError(err)
		}
		target := strings.TrimRight(service.config.WebBaseURL, "/") + next
		clearStateCookie := authCookie(service.config, "heyblog_github_state", "", "/auth/github", -1, time.Unix(1, 0))
		response := httpapi.NoContent(http.StatusFound).WithHeader("Location", target).WithHeader("Set-Cookie", clearStateCookie.String())
		return addCookies(response, cookies, service.config), nil
	})
	register(http.MethodPost, "/auth/github/unbind", func(ctx *httpapi.Context) (httpapi.Response, error) {
		user, cookies, err := service.GithubUnbind(ctx.Request.Context(), ctx.Request)
		if err != nil {
			return httpapi.Response{}, mapError(err)
		}
		response, responseErr := httpapi.JSON(http.StatusOK, map[string]User{"user": user})
		if responseErr != nil {
			return httpapi.Response{}, responseErr
		}
		return addCookies(response, cookies, service.config), nil
	})
	register(http.MethodGet, "/management/users", func(ctx *httpapi.Context) (httpapi.Response, error) {
		actor, err := service.Current(ctx.Request.Context(), ctx.Request)
		if err != nil {
			return httpapi.Response{}, mapError(err)
		}
		users, err := service.ListManagedUsers(ctx.Request.Context(), actor)
		if err != nil {
			return httpapi.Response{}, mapError(err)
		}
		return httpapi.JSON(http.StatusOK, map[string][]User{"users": users})
	})
	register(http.MethodPatch, "/management/users/:id/role", func(ctx *httpapi.Context) (httpapi.Response, error) {
		actor, err := service.Current(ctx.Request.Context(), ctx.Request)
		if err != nil {
			return httpapi.Response{}, mapError(err)
		}
		var payload roleRequest
		if err := decodeJSON(ctx.Request, &payload); err != nil {
			return httpapi.Response{}, err
		}
		user, err := service.UpdateRole(ctx.Request.Context(), actor, ctx.Param("id"), payload.Role)
		if err != nil {
			return httpapi.Response{}, mapError(err)
		}
		return httpapi.JSON(http.StatusOK, map[string]User{"user": user})
	})
	register(http.MethodPatch, "/management/users/:id/permissions", func(ctx *httpapi.Context) (httpapi.Response, error) {
		actor, err := service.Current(ctx.Request.Context(), ctx.Request)
		if err != nil {
			return httpapi.Response{}, mapError(err)
		}
		var payload permissionsRequest
		if err := decodeJSON(ctx.Request, &payload); err != nil {
			return httpapi.Response{}, err
		}
		user, err := service.UpdatePermissions(ctx.Request.Context(), actor, ctx.Param("id"), payload.Permissions)
		if err != nil {
			return httpapi.Response{}, mapError(err)
		}
		return httpapi.JSON(http.StatusOK, map[string]User{"user": user})
	})
	return nil
}
