//go:build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/httpapi"
)

func TestEmailChangeCredentials(t *testing.T) {
	fixture := newPasswordResetFixture(t)
	const (
		path     = "/api/web/account/email-change"
		oldEmail = "credential_admin@example.test"
		newEmail = "updated@example.test"
		password = "correct-password"
	)
	resetToken := fixture.resetToken(t, oldEmail)
	_, otherSession, err := fixture.auth.Login(t.Context(), oldEmail, password)
	require.NoError(t, err)
	request := func(method, suffix, id, body string) *httptest.ResponseRecorder {
		return emailChangeRequest(fixture, method, path+suffix, id, body, &http.Cookie{Name: "heyblog_access_token", Value: fixture.access})
	}
	start := request(http.MethodPost, "", "", `{"email":"UPDATED@example.test"}`)
	var status auth.EmailChangeStatus
	require.Equal(t, http.StatusOK, start.Code, start.Body.String())
	require.NoError(t, json.Unmarshal(start.Body.Bytes(), &status))
	require.NotNil(t, status.Request)
	require.Equal(t, newEmail, status.Request.NewEmail)
	require.Len(t, fixture.sender.messages, 1, "starting a change should not send verification codes")
	id := status.Request.ID
	requireEmailChangeProblem(t, request(http.MethodPost, "/complete", id, `{}`), http.StatusConflict, "email_change_verification_required")
	requireEmailChangeProblem(t, request(http.MethodPost, "/send-code", "019ded7f-4be3-7168-8953-b64109326083", `{"side":"old"}`), http.StatusNotFound, "email_change_missing")
	require.Len(t, fixture.sender.messages, 1)

	for _, address := range []struct{ side, email string }{{"old", oldEmail}, {"new", newEmail}} {
		sent := request(http.MethodPost, "/send-code", id, fmt.Sprintf(`{"side":%q}`, address.side))
		require.Equal(t, http.StatusOK, sent.Code, sent.Body.String())
		message := fixture.sender.messages[len(fixture.sender.messages)-1]
		require.Equal(t, address.email, message.To)
		code := regexp.MustCompile(`[0-9]{6}`).FindString(message.Text)
		require.Len(t, code, 6)
		body := fmt.Sprintf(`{"side":%q,"code":%q}`, address.side, code)
		verified := request(http.MethodPost, "/verify-code", id, body)
		require.Equal(t, http.StatusOK, verified.Code, verified.Body.String())
		requireEmailChangeProblem(t, request(http.MethodPost, "/verify-code", id, body), http.StatusBadRequest, "invalid_verification_code")
	}

	completed := request(http.MethodPost, "/complete", id, `{}`)
	require.Equal(t, http.StatusOK, completed.Code, completed.Body.String())
	require.Len(t, completed.Result().Cookies(), 2)
	current := emailChangeRequest(fixture, http.MethodGet, "/api/web/auth/me", "", "", completed.Result().Cookies()...)
	require.Equal(t, http.StatusOK, current.Code, current.Body.String())
	var account struct {
		User auth.User `json:"user"`
	}
	require.NoError(t, json.Unmarshal(current.Body.Bytes(), &account))
	require.NotNil(t, account.User.Email)
	require.Equal(t, newEmail, *account.User.Email)
	require.True(t, account.User.EmailVerified)
	require.Equal(t, oldEmail, fixture.sender.messages[len(fixture.sender.messages)-1].To)
	oldAccess := emailChangeRequest(fixture, http.MethodGet, "/api/web/auth/me", "", "", &http.Cookie{Name: "heyblog_access_token", Value: otherSession[0]})
	requireEmailChangeProblem(t, oldAccess, http.StatusUnauthorized, "unauthenticated")
	oldRefresh := emailChangeRequest(fixture, http.MethodPost, "/api/web/auth/refresh", "", "", &http.Cookie{Name: "heyblog_refresh_token", Value: otherSession[1]})
	requireEmailChangeProblem(t, oldRefresh, http.StatusUnauthorized, "session_expired")
	reset := emailChangeRequest(fixture, http.MethodPost, "/api/web/auth/password/reset", "", fmt.Sprintf(`{"token":%q,"password":"replacement-password"}`, resetToken))
	requireEmailChangeProblem(t, reset, http.StatusBadRequest, "invalid_password_reset_token")
	_, _, err = fixture.auth.Login(t.Context(), oldEmail, password)
	require.Error(t, err)
	_, newSession, err := fixture.auth.Login(t.Context(), newEmail, password)
	require.NoError(t, err)
	fixture.access = newSession[0]
	start = request(http.MethodPost, "", "", `{"email":"another@example.test"}`)
	require.Equal(t, http.StatusOK, start.Code, start.Body.String())
	require.NoError(t, json.Unmarshal(start.Body.Bytes(), &status))
	require.NotNil(t, status.Request)
	const passwordPath = "/verify-password"
	requireEmailChangeProblem(t, request(http.MethodPost, passwordPath, status.Request.ID, `{"password":"incorrect-password"}`), http.StatusForbidden, "invalid_current_password")
	verified := request(http.MethodPost, passwordPath, status.Request.ID, fmt.Sprintf(`{"password":%q}`, password))
	require.Equal(t, http.StatusOK, verified.Code, verified.Body.String())
	require.NoError(t, json.Unmarshal(verified.Body.Bytes(), &status))
	require.NotNil(t, status.Request)
	require.True(t, status.Request.OldVerified)
	require.False(t, status.Request.NewVerified)
}

func emailChangeRequest(fixture passwordResetFixture, method, path, id, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set(httpapi.WebTokenHeader, credentialWebToken)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if id != "" {
		request.Header.Set("X-HeyBlog-Email-Change-ID", id)
	}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	fixture.router.ServeHTTP(response, request)
	return response
}

func requireEmailChangeProblem(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	require.Equal(t, status, response.Code, response.Body.String())
	var problem httpapi.Problem
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &problem))
	require.Equal(t, code, problem.Code)
}
