package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/mail"
)

const legacyGithubStateCookieName = "heyblog_github_state"

func githubStateCookieName(stateToken string) string {
	return legacyGithubStateCookieName + "_" + stateToken
}

func readGithubStateCookie(request *http.Request, stateToken string) (string, bool) {
	if stateCookie, err := request.Cookie(githubStateCookieName(stateToken)); err == nil {
		return stateCookie.Value, false
	}
	if stateCookie, err := request.Cookie(legacyGithubStateCookieName); err == nil {
		return stateCookie.Value, true
	}
	return "", false
}

type loginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}
type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email" format:"email"`
	Password string `json:"password"`
}
type verifyRequest struct {
	Email string `json:"email" format:"email"`
	Code  string `json:"code"`
}
type emailRequest struct {
	Email string `json:"email" format:"email"`
}
type resetRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}
type setPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NextPassword    string `json:"next_password"`
}
type roleRequest struct {
	Role Role `json:"role"`
}
type permissionsRequest struct {
	Permissions []Permission `json:"permissions"`
}

func cookieValues(cookies []string, config Config) []string {
	values := make([]string, 0, len(cookies))
	names := []string{"heyblog_access_token", "heyblog_refresh_token"}
	ttls := []time.Duration{config.AccessTTL, config.RefreshTTL}
	for index, token := range cookies {
		if index >= len(names) {
			break
		}
		ttl := ttls[index]
		values = append(values, authCookie(config, names[index], token, "/", int(ttl.Seconds()), time.Now().Add(ttl)).String())
	}
	return values
}

func clearedCookieValues(config Config) []string {
	values := make([]string, 0, 2)
	for _, name := range []string{"heyblog_access_token", "heyblog_refresh_token"} {
		values = append(values, authCookie(config, name, "", "/", -1, time.Unix(1, 0)).String())
	}
	return values
}

func mapError(err error) error {
	if errors.Is(err, mail.ErrDeliveryUnavailable) {
		return apperror.Wrap(err, apperror.KindUnavailable, "mail_unavailable", "email delivery is temporarily unavailable", "send authentication email")
	}
	var authErr *AuthError
	if errors.As(err, &authErr) {
		kind := apperror.KindBadRequest
		switch statusKind(authErr.StatusCode) {
		case "unauthorized":
			kind = apperror.KindUnauthorized
		case "forbidden":
			kind = apperror.KindForbidden
		case "conflict":
			kind = apperror.KindConflict
		case "rate_limited":
			kind = apperror.KindRateLimited
		case "validation":
			kind = apperror.KindValidation
		case "unavailable":
			kind = apperror.KindUnavailable
		}
		return apperror.New(kind, authErr.Code, authErr.Message)
	}
	return apperror.Wrap(err, apperror.KindInternal, apperror.CodeInternal, "authentication service is unavailable", "authentication request")
}

func authCookie(config Config, name, value, path string, maxAge int, expires time.Time) *http.Cookie {
	//nolint:gosec // Local development uses HTTP; production WebBaseURL validation requires HTTPS.
	return &http.Cookie{Name: name, Value: value, Path: path, Domain: config.CookieDomain, MaxAge: maxAge,
		Expires: expires, HttpOnly: true, Secure: strings.HasPrefix(config.WebBaseURL, "https://"), SameSite: http.SameSiteLaxMode}
}
