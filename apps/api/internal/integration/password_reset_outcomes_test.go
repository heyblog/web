//go:build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/infrastructure/mail"
	"heyblog-api/internal/platform/httpapi"
)

type passwordResetLog struct {
	Event     string                    `json:"event"`
	RequestID string                    `json:"request_id"`
	Outcome   auth.PasswordResetOutcome `json:"outcome"`
	Level     string                    `json:"level"`
	Code      string                    `json:"code"`
}

func TestPasswordResetOutcomesPreserveAnonymousResponseAndPrivateLogs(t *testing.T) {
	fixture := newPasswordResetFixture(t)
	tests := []struct {
		name, status string
		verified     bool
		deleting     bool
		outcome      auth.PasswordResetOutcome
	}{
		{name: "missing", outcome: auth.PasswordResetNotFound},
		{name: "unverified", status: "ACTIVE", outcome: auth.PasswordResetUnverified},
		{name: "suspended", status: "SUSPENDED", verified: true, outcome: auth.PasswordResetUnavailable},
		{name: "deleting", status: "SUSPENDED", verified: true, deleting: true, outcome: auth.PasswordResetUnavailable},
		{name: "sent", status: "ACTIVE", verified: true, outcome: auth.PasswordResetSent},
		{name: "failed", status: "ACTIVE", verified: true, outcome: auth.PasswordResetFailed},
	}
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			email := test.name + "-reset@example.test"
			if test.status != "" {
				_, err := fixture.pool.Exec(t.Context(), `INSERT INTO identity.users
					(email, username, display_name, password_hash, email_verified_at, access_status, deletion_requested_at, deletion_scheduled_for)
					SELECT $1, $2, 'Reset User', password_hash,
					CASE WHEN $3 THEN clock_timestamp() END, $4,
					CASE WHEN $5 THEN statement_timestamp() END,
					CASE WHEN $5 THEN statement_timestamp() + interval '30 days' END
					FROM identity.users WHERE id = $6::uuid`, email, "reset_"+test.name, test.verified, test.status, test.deleting, fixture.actor)
				require.NoError(t, err)
			}
			fixture.sender.failure = nil
			if test.outcome == auth.PasswordResetFailed {
				fixture.sender.failure = mail.ErrDeliveryUnavailable
			}
			messageCount := len(fixture.sender.messages)
			outcome, err := fixture.auth.ForgotPassword(t.Context(), email)
			require.Equal(t, test.outcome, outcome)
			if test.outcome == auth.PasswordResetFailed {
				require.ErrorIs(t, err, mail.ErrDeliveryUnavailable)
			} else {
				require.NoError(t, err)
			}

			fixture.logs.Reset()
			request := httptest.NewRequest(http.MethodPost, "/api/web/auth/password/forgot", strings.NewReader(fmt.Sprintf(`{"email":%q}`, email)))
			request.RemoteAddr = fmt.Sprintf("192.0.2.%d:12345", i+1)
			request.Header.Set(httpapi.WebTokenHeader, credentialWebToken)
			request.Header.Set("Content-Type", "application/json")
			requestID := "reset-request-" + test.name
			request.Header.Set(httpapi.RequestIDHeader, requestID)
			response := httptest.NewRecorder()
			fixture.router.ServeHTTP(response, request)
			wantStatus, wantErrors, wantMessages := http.StatusNoContent, 0, messageCount
			if test.outcome == auth.PasswordResetFailed {
				wantStatus, wantErrors = http.StatusServiceUnavailable, 1
				require.Contains(t, response.Body.String(), `"code":"mail_unavailable"`)
			}
			if test.outcome == auth.PasswordResetSent {
				wantMessages += 2
			}
			require.Equal(t, wantStatus, response.Code, response.Body.String())
			require.Len(t, fixture.sender.messages, wantMessages)
			if wantStatus == http.StatusNoContent {
				require.Empty(t, response.Body.String())
			}
			rawLogs := fixture.logs.String()
			require.NotContains(t, rawLogs, email)
			require.NotContains(t, rawLogs, fixture.actor)
			require.NotContains(t, rawLogs, credentialWebToken)
			if test.outcome == auth.PasswordResetSent {
				match := regexp.MustCompile(`token=([^\s]+)`).FindStringSubmatch(fixture.sender.messages[len(fixture.sender.messages)-1].Text)
				require.Len(t, match, 2)
				require.NotContains(t, rawLogs, match[1])
			}
			decoder := json.NewDecoder(strings.NewReader(rawLogs))
			events, requestErrors := 0, 0
			for {
				var event passwordResetLog
				decodeErr := decoder.Decode(&event)
				if decodeErr == io.EOF {
					break
				}
				require.NoError(t, decodeErr)
				switch event.Event {
				case "password_reset_request":
					events++
					require.Equal(t, requestID, event.RequestID)
					require.Equal(t, test.outcome, event.Outcome)
					wantLevel := "INFO"
					if test.outcome == auth.PasswordResetFailed {
						wantLevel = "WARN"
					}
					require.Equal(t, wantLevel, event.Level)
				case "request_error":
					requestErrors++
					require.Equal(t, "mail_unavailable", event.Code)
				}
			}
			require.Equal(t, 1, events)
			require.Equal(t, wantErrors, requestErrors, "delivery failure must be recorded by the error boundary exactly once")
		})
	}
}
