//go:build integration

package integration_test

import (
	"bytes"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/config"
	"heyblog-api/internal/platform/httpapi"
)

type passwordResetFixture struct {
	credentialFixture
	sender *authMailRecorder
	logs   *bytes.Buffer
}

func newPasswordResetFixture(t *testing.T) passwordResetFixture {
	t.Helper()
	fixture := passwordResetFixture{credentialFixture: newCredentialFixture(t), sender: &authMailRecorder{}, logs: &bytes.Buffer{}}
	// #nosec G101 -- Fixed credentials are used only by this isolated test fixture.
	fixture.auth = auth.NewService(auth.Dependencies{Pool: fixture.pool, Redis: fixture.redis, MailSender: fixture.sender, Config: auth.Config{
		AccessSecret: "credential-access-test-secret", RefreshSecret: "credential-refresh-test-secret", AccessTTL: time.Hour, RefreshTTL: time.Hour,
		PasswordResetTTL: 30 * time.Minute, WebBaseURL: "https://web.example.test", MailFrom: "verify@example.test",
	}})
	router, err := httpapi.NewRouter(httpapi.Options{
		Mode: config.ModeDevelopment, HTTP: config.HTTPConfig{MaxBodyBytes: 1 << 20, TrustedProxies: []string{}},
		Logger: slog.New(slog.NewJSONHandler(fixture.logs, nil)), WebToken: credentialWebToken, HealthcheckToken: "reset-integration-health-token",
	})
	require.NoError(t, err)
	fixture.router = router
	require.NoError(t, auth.RegisterRoutes(router.API, fixture.auth, credentialWebToken))
	return fixture
}

func (fixture passwordResetFixture) resetToken(t *testing.T, email string) string {
	t.Helper()
	outcome, err := fixture.auth.ForgotPassword(t.Context(), email)
	require.NoError(t, err)
	require.Equal(t, auth.PasswordResetSent, outcome)
	require.NotEmpty(t, fixture.sender.messages)
	match := regexp.MustCompile(`token=([^\s]+)`).FindStringSubmatch(fixture.sender.messages[len(fixture.sender.messages)-1].Text)
	require.Len(t, match, 2)
	return match[1]
}
