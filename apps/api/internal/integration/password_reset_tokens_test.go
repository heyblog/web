//go:build integration

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPasswordResetTokenLifecycle(t *testing.T) {
	fixture := newPasswordResetFixture(t)
	t.Run("existing_password", func(t *testing.T) {
		const email = "credential_admin@example.test"
		token := fixture.resetToken(t, email)
		require.NoError(t, fixture.auth.ResetPassword(t.Context(), token, "updated-local-password"))
		_, _, err := fixture.auth.Login(t.Context(), email, "correct-password")
		require.Error(t, err, "password reset must replace the existing password")
		user, _, err := fixture.auth.Login(t.Context(), email, "updated-local-password")
		require.NoError(t, err)
		require.True(t, user.HasPassword)
	})
	for _, scenario := range []string{"replacement", "expiry", "email_change", "suspension", "deletion"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := t.Context()
			email := "token-" + scenario + "@example.test"
			var userID string
			err := fixture.pool.QueryRow(ctx, `INSERT INTO identity.users
				(email, username, display_name, email_verified_at)
				VALUES ($1, $2, 'Token User', clock_timestamp()) RETURNING id::text`, email, "token_"+scenario).Scan(&userID)
			require.NoError(t, err)
			token := fixture.resetToken(t, email)
			switch scenario {
			case "replacement":
				replacement := fixture.resetToken(t, email)
				require.NotEqual(t, token, replacement)
				require.Error(t, fixture.auth.ResetPassword(ctx, token, "old-reset-password"))
				require.NoError(t, fixture.auth.ResetPassword(ctx, replacement, "replacement-password"))
				require.Error(t, fixture.auth.ResetPassword(ctx, replacement, "reused-password"))
				_, _, err := fixture.auth.Login(ctx, email, "replacement-password")
				require.NoError(t, err)
				return
			case "expiry":
				_, err = fixture.pool.Exec(ctx, `UPDATE identity.password_reset_tokens SET created_at = clock_timestamp() - interval '2 seconds', expires_at = clock_timestamp() - interval '1 second' WHERE user_id = $1::uuid`, userID)
			case "email_change":
				_, err = fixture.pool.Exec(ctx, `UPDATE identity.users SET email = 'changed-token@example.test' WHERE id = $1::uuid`, userID)
			case "suspension":
				_, err = fixture.pool.Exec(ctx, `UPDATE identity.users SET access_status = 'SUSPENDED' WHERE id = $1::uuid`, userID)
			case "deletion":
				_, err = fixture.pool.Exec(ctx, `UPDATE identity.users SET access_status = 'SUSPENDED', deletion_requested_at = statement_timestamp(), deletion_scheduled_for = statement_timestamp() + interval '30 days' WHERE id = $1::uuid`, userID)
			}
			require.NoError(t, err)
			require.Error(t, fixture.auth.ResetPassword(ctx, token, "unavailable-password"))
			var hasPassword bool
			require.NoError(t, fixture.pool.QueryRow(ctx, `SELECT password_hash IS NOT NULL FROM identity.users WHERE id = $1::uuid`, userID).Scan(&hasPassword))
			require.False(t, hasPassword, "invalid reset tokens must not set the first password")
		})
	}
}
