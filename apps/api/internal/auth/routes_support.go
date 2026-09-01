package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/httpapi"
	"heyblog-api/internal/mail"
)

type loginRequest struct{ Identifier, Password string }
type registerRequest struct{ Username, Email, Password string }
type verifyRequest struct{ Email, Code string }
type emailRequest struct{ Email string }
type resetRequest struct{ Token, Password string }
type setPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NextPassword    string `json:"next_password"`
}
type roleRequest struct{ Role Role }
type permissionsRequest struct{ Permissions []Permission }

func decodeJSON(request *http.Request, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return apperror.New(apperror.KindValidation, apperror.CodeValidationFailed, "request body is invalid")
	}
	return nil
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

func addCookies(response httpapi.Response, cookies []string, config Config) httpapi.Response {
	names := []string{"heyblog_access_token", "heyblog_refresh_token"}
	ttls := []time.Duration{config.AccessTTL, config.RefreshTTL}
	for index, token := range cookies {
		if index >= len(names) {
			break
		}
		ttl := ttls[index]
		response = response.WithHeader("Set-Cookie", authCookie(config, names[index], token, "/", int(ttl.Seconds()), time.Now().Add(ttl)).String())
	}
	return response.WithHeader("Cache-Control", "no-store")
}

func clearCookies(response httpapi.Response, config Config) httpapi.Response {
	for _, name := range []string{"heyblog_access_token", "heyblog_refresh_token"} {
		response = response.WithHeader("Set-Cookie", authCookie(config, name, "", "/", -1, time.Unix(1, 0)).String())
	}
	return response.WithHeader("Cache-Control", "no-store")
}

func authCookie(config Config, name, value, path string, maxAge int, expires time.Time) *http.Cookie {
	//nolint:gosec // Local development uses HTTP; production WebBaseURL validation requires HTTPS.
	return &http.Cookie{Name: name, Value: value, Path: path, Domain: config.CookieDomain, MaxAge: maxAge,
		Expires: expires, HttpOnly: true, Secure: strings.HasPrefix(config.WebBaseURL, "https://"), SameSite: http.SameSiteLaxMode}
}
