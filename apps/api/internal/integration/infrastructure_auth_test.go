//go:build integration

package integration_test

import (
	"context"
	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/infrastructure/cache"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/infrastructure/mail"
	"heyblog-api/internal/platform/config"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

type authMailRecorder struct{ messages []mail.Message }

func (sender *authMailRecorder) Send(_ context.Context, message mail.Message) error {
	sender.messages = append(sender.messages, message)
	return nil
}

type githubRoundTripFunc func(*http.Request) (*http.Response, error)

func (function githubRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func verifyAuthenticationFlows(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	container, err := tcredis.Run(ctx, "redis:8.4-alpine")
	if err != nil {
		t.Fatalf("start auth Redis container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Errorf("terminate auth Redis container: %v", err)
		}
	})
	redisURL, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("get auth Redis connection string: %v", err)
	}
	redisClient, err := cache.OpenRedis(ctx, config.RedisConfig{URL: redisURL, DialTimeout: 3 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("open auth Redis client: %v", err)
	}
	t.Cleanup(func() { _ = redisClient.Close() })

	recorder := &authMailRecorder{}
	githubClient := &http.Client{Transport: githubRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"access_token":"github-integration-token"}`
		switch request.URL.Path {
		case "/user":
			body = `{"id":4242,"login":"integration-oauth","name":"Integration OAuth","avatar_url":"https://avatars.example.test/4242"}`
		case "/user/emails":
			body = `[{"email":"github-integration@example.test","primary":true,"verified":true}]`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}
	service := auth.NewService(auth.Dependencies{Pool: pool, Redis: redisClient, MailSender: recorder,
		VerificationMailer: mail.NewVerificationMailer(recorder, "verify@example.test", 10*time.Minute), GithubHTTPClient: githubClient, Config: auth.Config{
			AccessSecret: "integration-access-secret", RefreshSecret: "integration-refresh-secret",
			AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour, VerificationTTL: 10 * time.Minute,
			PasswordResetTTL: 30 * time.Minute, WebBaseURL: "http://web.example.test", MailFrom: "verify@example.test",
			GithubClientID: "github-client-id", GithubClientSecret: "github-client-secret", GithubScope: "read:user,user:email",
		}})
	const email = "auth-integration@example.test"
	if err := service.Register(ctx, "auth_integration", email, "correct-password"); err != nil {
		t.Fatalf("register auth user: %v", err)
	}
	code := regexp.MustCompile(`[0-9]{6}`).FindString(recorder.messages[len(recorder.messages)-1].Text)
	if code == "" {
		t.Fatal("verification email did not contain a six-digit code")
	}
	if _, _, err := service.Login(ctx, email, "correct-password"); err == nil {
		t.Fatal("unverified user unexpectedly logged in")
	}
	if err := service.VerifyEmail(ctx, email, code); err != nil {
		t.Fatalf("verify auth email: %v", err)
	}
	if err := service.VerifyEmail(ctx, email, code); err == nil {
		t.Fatal("verification code was accepted twice")
	}

	user, tokens, err := service.Login(ctx, email, "correct-password")
	if err != nil {
		t.Fatalf("login verified user: %v", err)
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://api.example.test/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: "heyblog_access_token", Value: tokens[0]})
	request.AddCookie(&http.Cookie{Name: "heyblog_refresh_token", Value: tokens[1]})
	if current, err := service.Current(ctx, request); err != nil || current.ID != user.ID {
		t.Fatalf("current user = (%q, %v), want %q", current.ID, err, user.ID)
	}
	_, rotated, err := service.Refresh(ctx, request)
	if err != nil {
		t.Fatalf("refresh session: %v", err)
	}
	if _, _, err := service.Refresh(ctx, request); err == nil {
		t.Fatal("refresh token was accepted after rotation")
	}
	logoutRequest, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://api.example.test/auth/logout", nil)
	logoutRequest.AddCookie(&http.Cookie{Name: "heyblog_refresh_token", Value: rotated[1]})
	if err := service.Logout(ctx, logoutRequest); err != nil {
		t.Fatalf("logout session: %v", err)
	}

	if err := service.ForgotPassword(ctx, email); err != nil {
		t.Fatalf("request password reset: %v", err)
	}
	resetText := recorder.messages[len(recorder.messages)-1].Text
	resetMatch := regexp.MustCompile(`token=([^\s]+)`).FindStringSubmatch(resetText)
	resetToken := ""
	if len(resetMatch) == 2 {
		resetToken = resetMatch[1]
	}
	if resetToken == "" {
		t.Fatal("password reset email did not contain a token")
	}
	if err := service.ResetPassword(ctx, resetToken, "new-correct-password"); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	if err := service.ResetPassword(ctx, resetToken, "another-password"); err == nil {
		t.Fatal("password reset token was accepted twice")
	}
	if _, _, err := service.Login(ctx, email, "correct-password"); err == nil {
		t.Fatal("old password remained valid after reset")
	}
	user, _, err = service.Login(ctx, email, "new-correct-password")
	if err != nil {
		t.Fatalf("login with reset password: %v", err)
	}

	_, firstStateToken, err := service.GithubStart(ctx, "/dashboard", false)
	if err != nil {
		t.Fatalf("start first GitHub login: %v", err)
	}
	_, secondStateToken, err := service.GithubStart(ctx, "/dashboard", false)
	if err != nil {
		t.Fatalf("start second GitHub login: %v", err)
	}
	firstGithubRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://api.example.test/auth/github/callback", nil)
	firstGithubRequest.AddCookie(&http.Cookie{Name: "heyblog_github_state_" + firstStateToken, Value: firstStateToken})
	githubUser, githubTokens, _, err := service.GithubCallback(ctx, firstGithubRequest, "oauth-code", firstStateToken, firstStateToken)
	if err != nil {
		t.Fatalf("complete first GitHub login: %v", err)
	}
	if !githubUser.EmailVerified {
		t.Fatal("new GitHub user email was not marked verified")
	}
	secondGithubRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://api.example.test/auth/github/callback", nil)
	secondGithubRequest.AddCookie(&http.Cookie{Name: "heyblog_github_state_" + secondStateToken, Value: secondStateToken})
	if _, _, _, err := service.GithubCallback(ctx, secondGithubRequest, "oauth-code", secondStateToken, secondStateToken); err != nil {
		t.Fatalf("complete second GitHub login: %v", err)
	}
	setPasswordRequest, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://api.example.test/auth/password", nil)
	setPasswordRequest.AddCookie(&http.Cookie{Name: "heyblog_access_token", Value: githubTokens[0]})
	if _, _, err := service.SetPassword(ctx, setPasswordRequest, "", "github-local-password"); err != nil {
		t.Fatalf("set GitHub user password: %v", err)
	}
	messageCount := len(recorder.messages)
	if err := service.ForgotPassword(ctx, "github-integration@example.test"); err != nil {
		t.Fatalf("request GitHub user password reset: %v", err)
	}
	if len(recorder.messages) != messageCount+1 || !strings.Contains(recorder.messages[len(recorder.messages)-1].Text, "30 分钟") {
		t.Fatalf("GitHub password reset messages = %#v, want one reset email with configured validity", recorder.messages[messageCount:])
	}

	admin, err := dbgen.New(pool).CreateUser(ctx, dbgen.CreateUserParams{Email: "sysadmin@example.test", Username: "auth_sysadmin", DisplayName: "Auth Sysadmin"})
	if err != nil {
		t.Fatalf("create auth sysadmin: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE identity.users SET role = 'SYS_ADMIN', email_verified_at = clock_timestamp() WHERE id = $1`, admin.ID); err != nil {
		t.Fatalf("promote auth sysadmin: %v", err)
	}
	actor := auth.User{ID: admin.ID.String(), Role: auth.RoleSysAdmin}
	managed, err := service.UpdateRole(ctx, actor, user.ID, auth.RoleAdmin)
	if err != nil || managed.Role != auth.RoleAdmin {
		t.Fatalf("update auth role = (%q, %v)", managed.Role, err)
	}
	managed, err = service.UpdatePermissions(ctx, actor, user.ID, []auth.Permission{auth.PermissionUserManage})
	if err != nil || !slices.Contains(managed.Permissions, auth.PermissionUserManage) {
		t.Fatalf("update auth permissions = (%v, %v)", managed.Permissions, err)
	}
}

func verifyUserDeletionSemantics(
	ctx context.Context,
	t *testing.T,
	runtimeConnection *pgxpool.Pool,
	migrationURL string,
) {
	t.Helper()
	queries := dbgen.New(runtimeConnection)
	user, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       "deletion@example.com",
		Username:    "deletion_user",
		DisplayName: "\u6635\u79f0 @ # []",
	})
	if err != nil {
		t.Fatalf("create deletion semantics user: %v", err)
	}
	if _, err := runtimeConnection.Exec(ctx, `
		INSERT INTO identity.users (email, username, display_name)
		VALUES ('trimmed-name@example.com', 'trimmed_name', ' padded ')
	`); err == nil {
		t.Fatal("display name with surrounding whitespace unexpectedly succeeded")
	}
	if _, err := runtimeConnection.Exec(ctx, `
		INSERT INTO identity.users (email, username, display_name)
		VALUES ('long-name@example.com', 'long_name', repeat('x', 129))
	`); err == nil {
		t.Fatal("display name longer than 128 characters unexpectedly succeeded")
	}
	if _, err := runtimeConnection.Exec(ctx, `
		INSERT INTO identity.users (email, username, display_name, access_status)
		VALUES ('removed-state@example.com', 'removed_state', 'Removed State', 'REMOVED')
	`); err == nil {
		t.Fatal("REMOVED access status unexpectedly succeeded")
	}

	if _, err := queries.UpsertGitHubIdentity(ctx, dbgen.UpsertGitHubIdentityParams{
		UserID:         user.ID,
		ProviderUserID: "deletion-provider-user",
		Profile:        []byte(`{}`),
	}); err != nil {
		t.Fatalf("create deletion semantics OAuth identity: %v", err)
	}

	suspended, err := queries.SuspendUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("suspend user: %v", err)
	}
	if suspended.AccessStatus != "SUSPENDED" || suspended.AuthVersion != user.AuthVersion+1 {
		t.Fatalf(
			"suspended user state = (%q, %d), want (SUSPENDED, %d)",
			suspended.AccessStatus,
			suspended.AuthVersion,
			user.AuthVersion+1,
		)
	}
	if _, err := queries.RecordUserLogin(ctx, user.ID); err == nil {
		t.Fatal("record login for suspended user unexpectedly succeeded")
	}

	active, err := queries.ActivateUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("activate user: %v", err)
	}
	if active.AccessStatus != "ACTIVE" || active.AuthVersion != suspended.AuthVersion+1 {
		t.Fatalf(
			"activated user state = (%q, %d), want (ACTIVE, %d)",
			active.AccessStatus,
			active.AuthVersion,
			suspended.AuthVersion+1,
		)
	}

	var verifiedVersion int32
	if err := runtimeConnection.QueryRow(ctx, `
		UPDATE identity.users
		   SET email_verified_at = clock_timestamp()
		 WHERE id = $1
		 RETURNING auth_version
	`, user.ID).Scan(&verifiedVersion); err != nil {
		t.Fatalf("verify user email: %v", err)
	}
	if verifiedVersion != active.AuthVersion+1 {
		t.Fatalf("verified auth version = %d, want %d", verifiedVersion, active.AuthVersion+1)
	}

	changedEmail := "deletion-updated@example.com"
	var changedEmailVerifiedAt pgtype.Timestamptz
	var changedEmailVersion int32
	if err := runtimeConnection.QueryRow(ctx, `
		UPDATE identity.users
		   SET email = $2
		 WHERE id = $1
		 RETURNING email_verified_at, auth_version
	`, user.ID, changedEmail).Scan(&changedEmailVerifiedAt, &changedEmailVersion); err != nil {
		t.Fatalf("change user email: %v", err)
	}
	if changedEmailVerifiedAt.Valid || changedEmailVersion != verifiedVersion+1 {
		t.Fatalf(
			"changed email state = (verified:%t, version:%d), want (false, %d)",
			changedEmailVerifiedAt.Valid,
			changedEmailVersion,
			verifiedVersion+1,
		)
	}

	var roleVersion int32
	if err := runtimeConnection.QueryRow(ctx, `
		UPDATE identity.users SET role = 'ADMIN' WHERE id = $1 RETURNING auth_version
	`, user.ID).Scan(&roleVersion); err != nil {
		t.Fatalf("change user role: %v", err)
	}
	if roleVersion != changedEmailVersion+1 {
		t.Fatalf("role auth version = %d, want %d", roleVersion, changedEmailVersion+1)
	}
	if _, err := runtimeConnection.Exec(ctx, `
		UPDATE identity.users SET auth_version = auth_version - 1 WHERE id = $1
	`, user.ID); err == nil {
		t.Fatal("decreasing auth version unexpectedly succeeded")
	}

	requested, err := queries.RequestUserDeletion(ctx, user.ID)
	if err != nil {
		t.Fatalf("request user deletion: %v", err)
	}
	if requested.AccessStatus != "SUSPENDED" ||
		!requested.DeletionRequestedAt.Valid ||
		!requested.DeletionScheduledFor.Valid ||
		requested.DeletionScheduledFor.Time.Sub(requested.DeletionRequestedAt.Time) != 30*24*time.Hour ||
		requested.AuthVersion != roleVersion+1 {
		t.Fatalf(
			"requested deletion state = (status:%q, request:%t, schedule:%t, version:%d)",
			requested.AccessStatus,
			requested.DeletionRequestedAt.Valid,
			requested.DeletionScheduledFor.Valid,
			requested.AuthVersion,
		)
	}
	if _, err := queries.RecordUserLogin(ctx, user.ID); err == nil {
		t.Fatal("record login for user pending deletion unexpectedly succeeded")
	}
	if _, err := queries.UpdateUserProfile(ctx, dbgen.UpdateUserProfileParams{
		ID:          user.ID,
		DisplayName: "Blocked Update",
		Profile:     []byte(`{}`),
		Settings:    []byte(`{}`),
	}); err == nil {
		t.Fatal("profile update for user pending deletion unexpectedly succeeded")
	}

	var oauthIdentityCount int
	if err := runtimeConnection.QueryRow(ctx, `
		SELECT count(*) FROM identity.oauth_identities WHERE user_id = $1
	`, user.ID).Scan(&oauthIdentityCount); err != nil {
		t.Fatalf("count OAuth identities while deletion is pending: %v", err)
	}
	if oauthIdentityCount != 1 {
		t.Fatalf("OAuth identity count while deletion is pending = %d, want 1", oauthIdentityCount)
	}

	cancelled, err := queries.CancelUserDeletion(ctx, user.ID)
	if err != nil {
		t.Fatalf("cancel user deletion: %v", err)
	}
	if cancelled.AccessStatus != "ACTIVE" ||
		cancelled.DeletionRequestedAt.Valid ||
		cancelled.DeletionScheduledFor.Valid ||
		cancelled.AuthVersion != requested.AuthVersion+1 {
		t.Fatalf(
			"cancelled deletion state = (status:%q, request:%t, schedule:%t, version:%d)",
			cancelled.AccessStatus,
			cancelled.DeletionRequestedAt.Valid,
			cancelled.DeletionScheduledFor.Valid,
			cancelled.AuthVersion,
		)
	}
	loggedIn, err := queries.RecordUserLogin(ctx, user.ID)
	if err != nil {
		t.Fatalf("record login after cancellation: %v", err)
	}
	if loggedIn.AuthVersion != cancelled.AuthVersion {
		t.Fatalf("login auth version = %d, want %d", loggedIn.AuthVersion, cancelled.AuthVersion)
	}

	deletedEmail := "due-deletion@example.com"
	var dueUserID pgtype.UUID
	if err := runtimeConnection.QueryRow(ctx, `
		INSERT INTO identity.users (
			email, username, display_name, password_hash, role, access_status,
			email_verified_at, profile, settings, last_login_at,
			deletion_requested_at, deletion_scheduled_for, created_at
		) VALUES (
			$1, 'due_deletion_user', 'Due Deletion User', 'password-hash', 'ADMIN', 'SUSPENDED',
			statement_timestamp() - interval '31 days', '{"bio":"private"}', '{"theme":"private"}',
			statement_timestamp() - interval '31 days',
			statement_timestamp() - interval '31 days', statement_timestamp() - interval '1 day',
			statement_timestamp() - interval '32 days'
		)
		RETURNING id
	`, deletedEmail).Scan(&dueUserID); err != nil {
		t.Fatalf("create user with due deletion: %v", err)
	}
	if _, err := runtimeConnection.Exec(ctx, `
		INSERT INTO identity.oauth_identities (user_id, provider, provider_user_id, profile)
		VALUES ($1, 'GITHUB', 'due-deletion-provider-user', '{"login":"private"}')
	`, dueUserID); err != nil {
		t.Fatalf("create OAuth identity for due deletion: %v", err)
	}
	if _, err := queries.CancelUserDeletion(ctx, dueUserID); err == nil {
		t.Fatal("cancelling deletion after its deadline unexpectedly succeeded")
	}
	if _, err := runtimeConnection.Exec(ctx, `
		UPDATE identity.users
		   SET username = 'released_username', deleted_at = clock_timestamp()
		 WHERE id = $1
	`, dueUserID); err == nil {
		t.Fatal("changing a username while completing deletion unexpectedly succeeded")
	}

	deleted, err := queries.CompleteUserDeletion(ctx, dueUserID)
	if err != nil {
		t.Fatalf("complete user deletion: %v", err)
	}
	if deleted.Email != nil ||
		deleted.DisplayName != deleted.Username ||
		deleted.PasswordHash != nil ||
		deleted.Role != "USER" ||
		deleted.AccessStatus != "SUSPENDED" ||
		deleted.EmailVerifiedAt.Valid ||
		string(deleted.Profile) != `{}` ||
		string(deleted.Settings) != `{}` ||
		deleted.LastLoginAt.Valid ||
		!deleted.DeletedAt.Valid ||
		deleted.AuthVersion != 2 {
		t.Fatalf(
			"completed deletion was not anonymized: email:%v display:%q role:%q status:%q version:%d",
			deleted.Email,
			deleted.DisplayName,
			deleted.Role,
			deleted.AccessStatus,
			deleted.AuthVersion,
		)
	}
	if err := runtimeConnection.QueryRow(ctx, `
		SELECT count(*) FROM identity.oauth_identities WHERE user_id = $1
	`, dueUserID).Scan(&oauthIdentityCount); err != nil {
		t.Fatalf("count OAuth identities after completed deletion: %v", err)
	}
	if oauthIdentityCount != 0 {
		t.Fatalf("OAuth identity count after completed deletion = %d, want 0", oauthIdentityCount)
	}
	if _, err := runtimeConnection.Exec(ctx, `
		UPDATE identity.users SET display_name = 'Restored User' WHERE id = $1
	`, dueUserID); err == nil {
		t.Fatal("updating a deleted user unexpectedly succeeded")
	}

	if _, err := runtimeConnection.Exec(ctx, "DELETE FROM identity.users WHERE id = $1", dueUserID); err == nil {
		t.Fatal("runtime user hard deletion unexpectedly succeeded")
	}
	if _, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       "replacement@example.com",
		Username:    "due_deletion_user",
		DisplayName: "Replacement User",
	}); err == nil {
		t.Fatal("reusing a deleted username unexpectedly succeeded")
	}
	replacement, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       deletedEmail,
		Username:    "replacement_user",
		DisplayName: "Replacement @ User",
	})
	if err != nil {
		t.Fatalf("reuse deleted user email: %v", err)
	}
	if _, err := queries.UpsertGitHubIdentity(ctx, dbgen.UpsertGitHubIdentityParams{
		UserID:         replacement.ID,
		ProviderUserID: "due-deletion-provider-user",
		Profile:        []byte(`{}`),
	}); err != nil {
		t.Fatalf("reuse deleted user OAuth identity: %v", err)
	}

	migrationConnection, err := pgx.Connect(ctx, migrationURL)
	if err != nil {
		t.Fatalf("connect as migrator for user deletion: %v", err)
	}
	defer func() { _ = migrationConnection.Close(context.Background()) }()
	if _, err := migrationConnection.Exec(ctx, "DELETE FROM identity.users WHERE id = $1", replacement.ID); err != nil {
		t.Fatalf("hard delete user as migrator: %v", err)
	}
	if err := migrationConnection.QueryRow(ctx, `
		SELECT count(*) FROM identity.oauth_identities WHERE user_id = $1
	`, replacement.ID).Scan(&oauthIdentityCount); err != nil {
		t.Fatalf("count OAuth identities after hard deletion: %v", err)
	}
	if oauthIdentityCount != 0 {
		t.Fatalf("OAuth identity count after hard deletion = %d, want 0", oauthIdentityCount)
	}
}
