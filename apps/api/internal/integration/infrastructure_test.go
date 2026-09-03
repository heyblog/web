//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"heyblog-api/internal/auth"
	"heyblog-api/internal/cache"
	"heyblog-api/internal/config"
	"heyblog-api/internal/database"
	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/database/migrations"
	"heyblog-api/internal/domain/content"
	"heyblog-api/internal/mail"
	"heyblog-api/internal/ratelimit"
)

func TestPostgresAGEInfrastructure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	container, err := postgres.Run(
		ctx,
		"apache/age:release_PG18_1.7.0",
		postgres.WithDatabase("heyblog"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres-secret"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL/AGE container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Errorf("terminate PostgreSQL/AGE container: %v", err)
		}
	})

	adminURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get PostgreSQL connection string: %v", err)
	}

	adminConnection, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect to migration database: %v", err)
	}
	t.Cleanup(func() { _ = adminConnection.Close(context.Background()) })

	bootstrapDatabaseRoles(ctx, t, adminConnection)
	migrationURL := databaseURLForRole(t, adminURL, "migrator", "migrator-secret")
	if err := database.Migrate(ctx, migrationURL); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if err := database.Migrate(ctx, migrationURL); err != nil {
		t.Fatalf("reapply migrations: %v", err)
	}

	verifyDatabaseCatalog(ctx, t, adminConnection)
	verifyRoleBoundaries(ctx, t, adminConnection)

	runtimeURL := databaseURLForRole(t, adminURL, "api_runtime", "runtime-secret")
	pool, err := database.OpenPool(ctx, config.DatabaseConfig{
		URL:                   runtimeURL,
		MaxConnections:        4,
		MinConnections:        0,
		MaxConnectionLifetime: time.Minute,
		MaxConnectionIdleTime: time.Minute,
		HealthCheckPeriod:     30 * time.Second,
	})
	if err != nil {
		t.Fatalf("open runtime pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)

	if result, err := dbgen.New(pool).Ping(ctx); err != nil || result != 1 {
		t.Fatalf("sqlc Ping() = (%d, %v), want (1, nil)", result, err)
	}

	verifyDirectorySiteTimestampSchema(ctx, t, pool)
	verifyDirectoryConstraints(ctx, t, pool)
	verifyPublicViewQueries(ctx, t, pool)
	verifyDirectoryQueries(ctx, t, pool)
	verifyTagAndIconConstraints(ctx, t, pool)
	verifyAnnouncementQueries(ctx, t, pool)
	verifyAnnouncementConstraints(ctx, t, pool, migrationURL)
	verifySoftwareComponentDependencies(ctx, t, pool)
	verifySiteAuditReviewDraftQueries(ctx, t, pool)
	verifySiteAuditShortIDMaintenance(ctx, t, pool)
	verifyFriendLinkGraph(ctx, t, pool, adminConnection)
	verifyRuntimePermissions(ctx, t, pool)
	verifyAnnouncementActorDeletionSemantics(ctx, t, pool, migrationURL)
	verifyUserDeletionSemantics(ctx, t, pool, migrationURL)
	verifyAuthenticationFlows(ctx, t, pool)
	verifyMigrationRollback(ctx, t, adminConnection, migrationURL)
}

func verifyDirectoryQueries(ctx context.Context, t *testing.T, connection *pgxpool.Pool) {
	t.Helper()
	queries := dbgen.New(connection)
	for index := range 26 {
		insertSite(
			ctx,
			t,
			connection,
			fmt.Sprintf("D%08d", index),
			fmt.Sprintf("Directory Fixture %02d", index),
			fmt.Sprintf("directory-%02d.example.com", index),
		)
	}

	filters := dbgen.CountDirectorySitesByStatusParams{
		QueryText: "directory fixture", PrimaryTagSlugs: []string{}, SecondaryTagSlugs: []string{},
		WarningSlugs: []string{}, TechnologyNames: []string{}, AccessScopes: []string{"ALL"},
		FeedMode: "without",
	}
	counts, err := queries.CountDirectorySitesByStatus(ctx, filters)
	if err != nil {
		t.Fatalf("count directory fixtures: %v", err)
	}
	if counts.NormalCount != 26 || counts.AbnormalCount != 0 {
		t.Fatalf("directory fixture counts = %#v, want normal=26 abnormal=0", counts)
	}

	base := dbgen.ListDirectorySitesParams{
		SiteVisibility: "VISIBLE", QueryText: filters.QueryText,
		PrimaryTagSlugs: filters.PrimaryTagSlugs, SecondaryTagSlugs: filters.SecondaryTagSlugs,
		WarningSlugs: filters.WarningSlugs, TechnologyNames: filters.TechnologyNames,
		AccessScopes: filters.AccessScopes, FeedMode: filters.FeedMode,
		SortMode: "random", Seed: "site-directory:integration", SortOrder: "desc", PageLimit: 24,
	}
	firstPage, err := queries.ListDirectorySites(ctx, base)
	if err != nil {
		t.Fatalf("list first stable directory page: %v", err)
	}
	repeatedPage, err := queries.ListDirectorySites(ctx, base)
	if err != nil {
		t.Fatalf("repeat first stable directory page: %v", err)
	}
	if len(firstPage) != 24 || !slices.EqualFunc(firstPage, repeatedPage, func(left, right dbgen.DirectorySite) bool {
		return left.ID == right.ID
	}) {
		t.Fatalf("stable directory page changed between identical queries")
	}
	base.PageOffset = 24
	secondPage, err := queries.ListDirectorySites(ctx, base)
	if err != nil {
		t.Fatalf("list second stable directory page: %v", err)
	}
	if len(secondPage) != 2 {
		t.Fatalf("second directory page length = %d, want 2", len(secondPage))
	}
	seen := make(map[pgtype.UUID]struct{}, len(firstPage))
	for _, row := range firstPage {
		seen[row.ID] = struct{}{}
	}
	for _, row := range secondPage {
		if _, exists := seen[row.ID]; exists {
			t.Fatalf("stable directory pages repeated site %s", row.ShortID)
		}
	}

	primaryOne, err := queries.CreateTag(ctx, dbgen.CreateTagParams{
		Name: "Directory Primary One", NormalizedName: "directory primary one",
		Slug: "directory-primary-one", Description: "integration fixture",
	})
	if err != nil {
		t.Fatalf("create first directory primary tag: %v", err)
	}
	primaryTwo, err := queries.CreateTag(ctx, dbgen.CreateTagParams{
		Name: "Directory Primary Two", NormalizedName: "directory primary two",
		Slug: "directory-primary-two", Description: "integration fixture",
	})
	if err != nil {
		t.Fatalf("create second directory primary tag: %v", err)
	}
	secondaryOne, err := queries.CreateTag(ctx, dbgen.CreateTagParams{
		Name: "Directory Secondary One", NormalizedName: "directory secondary one",
		Slug: "directory-secondary-one", Description: "integration fixture",
	})
	if err != nil {
		t.Fatalf("create first directory secondary tag: %v", err)
	}
	secondaryTwo, err := queries.CreateTag(ctx, dbgen.CreateTagParams{
		Name: "Directory Secondary Two", NormalizedName: "directory secondary two",
		Slug: "directory-secondary-two", Description: "integration fixture",
	})
	if err != nil {
		t.Fatalf("create second directory secondary tag: %v", err)
	}
	visibleBoth := insertSite(ctx, t, connection, "F00000001", "Role Fixture Visible Both", "role-visible-both.example.com")
	visiblePartial := insertSite(ctx, t, connection, "F00000002", "Role Fixture Visible Partial", "role-visible-partial.example.com")
	hiddenBoth := insertSite(ctx, t, connection, "F00000003", "Role Fixture Hidden Both", "role-hidden-both.example.com")
	removedBoth := insertSite(ctx, t, connection, "F00000004", "Role Fixture Removed Both", "role-removed-both.example.com")
	if _, err := connection.Exec(ctx, `UPDATE directory.sites SET visibility = 'HIDDEN', visibility_reason = 'fixture' WHERE id = $1`, hiddenBoth); err != nil {
		t.Fatalf("hide directory role fixture: %v", err)
	}
	if _, err := connection.Exec(ctx, `UPDATE directory.sites SET visibility = 'REMOVED', visibility_reason = 'fixture' WHERE id = $1`, removedBoth); err != nil {
		t.Fatalf("remove directory role fixture: %v", err)
	}
	assign := func(siteID, tagID pgtype.UUID, role string) {
		t.Helper()
		if _, assignErr := queries.AssignSiteTag(ctx, dbgen.AssignSiteTagParams{
			SiteID: siteID, TagID: tagID, Role: role, AssignmentSource: "SYSTEM",
		}); assignErr != nil {
			t.Fatalf("assign %s directory role fixture: %v", role, assignErr)
		}
	}
	for _, siteID := range []pgtype.UUID{visibleBoth, hiddenBoth, removedBoth} {
		assign(siteID, primaryOne.ID, "PRIMARY")
		assign(siteID, secondaryOne.ID, "SECONDARY")
		assign(siteID, secondaryTwo.ID, "SECONDARY")
	}
	assign(visiblePartial, primaryTwo.ID, "PRIMARY")
	assign(visiblePartial, secondaryOne.ID, "SECONDARY")

	roleFilters := dbgen.CountDirectorySitesByStatusParams{
		QueryText: "role fixture", PrimaryTagSlugs: []string{primaryOne.Slug, primaryTwo.Slug},
		SecondaryTagSlugs: []string{secondaryOne.Slug}, WarningSlugs: []string{},
		TechnologyNames: []string{}, AccessScopes: []string{}, FeedMode: "any",
	}
	roleCounts, err := queries.CountDirectorySitesByStatus(ctx, roleFilters)
	if err != nil {
		t.Fatalf("count primary OR directory fixtures: %v", err)
	}
	if roleCounts.NormalCount != 2 || roleCounts.AbnormalCount != 1 {
		t.Fatalf("primary OR status counts = %#v, want normal=2 abnormal=1", roleCounts)
	}
	roleFilters.SecondaryTagSlugs = []string{secondaryOne.Slug, secondaryTwo.Slug}
	roleCounts, err = queries.CountDirectorySitesByStatus(ctx, roleFilters)
	if err != nil {
		t.Fatalf("count secondary AND directory fixtures: %v", err)
	}
	if roleCounts.NormalCount != 1 || roleCounts.AbnormalCount != 1 {
		t.Fatalf("secondary AND status counts = %#v, want normal=1 abnormal=1", roleCounts)
	}
	hiddenRows, err := queries.ListDirectorySites(ctx, dbgen.ListDirectorySitesParams{
		SiteVisibility: "HIDDEN", QueryText: roleFilters.QueryText,
		PrimaryTagSlugs: roleFilters.PrimaryTagSlugs, SecondaryTagSlugs: roleFilters.SecondaryTagSlugs,
		WarningSlugs: []string{}, TechnologyNames: []string{}, AccessScopes: []string{},
		FeedMode: "any", SortMode: "joined", Seed: "integration", SortOrder: "desc", PageLimit: 24,
	})
	if err != nil {
		t.Fatalf("list hidden directory fixtures: %v", err)
	}
	if len(hiddenRows) != 1 || hiddenRows[0].ID != hiddenBoth {
		t.Fatalf("hidden directory rows = %#v, want only hidden fixture", hiddenRows)
	}
	optionRows, err := queries.ListDirectoryTagOptions(ctx)
	if err != nil {
		t.Fatalf("list directory tag options: %v", err)
	}
	var primaryOption, secondaryOption *dbgen.ListDirectoryTagOptionsRow
	for index := range optionRows {
		row := &optionRows[index]
		if row.Slug == primaryOne.Slug && row.Role == "PRIMARY" {
			primaryOption = row
		}
		if row.Slug == secondaryTwo.Slug && row.Role == "SECONDARY" {
			secondaryOption = row
		}
	}
	if primaryOption == nil || primaryOption.NormalCount != 1 || primaryOption.AbnormalCount != 1 {
		t.Fatalf("primary directory option = %#v, want normal=1 abnormal=1", primaryOption)
	}
	if secondaryOption == nil || secondaryOption.NormalCount != 1 || secondaryOption.AbnormalCount != 1 {
		t.Fatalf("secondary directory option = %#v, want normal=1 abnormal=1", secondaryOption)
	}
}

func verifySiteAuditReviewDraftQueries(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	queries := dbgen.New(pool)
	reviewer, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email: "audit-reviewer@example.test", Username: "audit_reviewer", DisplayName: "Audit Reviewer",
	})
	if err != nil {
		t.Fatalf("create audit reviewer: %v", err)
	}
	audit, err := queries.CreateSiteAudit(ctx, dbgen.CreateSiteAuditParams{
		LookupSecretHash: make([]byte, 32), Action: "CREATE",
		ProposedSnapshot: []byte(`{"name":"Submitted"}`), RequestReason: "",
	})
	if err != nil {
		t.Fatalf("create review draft audit: %v", err)
	}
	saved, err := queries.SaveSiteAuditReviewDraft(ctx, dbgen.SaveSiteAuditReviewDraftParams{
		ReviewDraftSnapshot: []byte(`{"name":"Corrected"}`), ReviewDraftUpdatedBy: reviewer.ID,
		ID: audit.ID, ExpectedReviewDraftRevision: 0,
	})
	if err != nil {
		t.Fatalf("save review draft: %v", err)
	}
	var savedDraft map[string]string
	if err := json.Unmarshal(saved.ReviewDraftSnapshot, &savedDraft); err != nil {
		t.Fatalf("decode saved review draft: %v", err)
	}
	if saved.ReviewDraftRevision != 1 || savedDraft["name"] != "Corrected" {
		t.Fatalf("saved review draft = (revision:%d snapshot:%s)", saved.ReviewDraftRevision, saved.ReviewDraftSnapshot)
	}
	if _, err := queries.SaveSiteAuditReviewDraft(ctx, dbgen.SaveSiteAuditReviewDraftParams{
		ReviewDraftSnapshot: []byte(`{"name":"Stale"}`), ReviewDraftUpdatedBy: reviewer.ID,
		ID: audit.ID, ExpectedReviewDraftRevision: 0,
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("stale review draft error = %v, want pgx.ErrNoRows", err)
	}
	discarded, err := queries.DiscardSiteAuditReviewDraft(ctx, dbgen.DiscardSiteAuditReviewDraftParams{
		ReviewDraftUpdatedBy: reviewer.ID, ID: audit.ID, ExpectedReviewDraftRevision: 1,
	})
	if err != nil {
		t.Fatalf("discard review draft: %v", err)
	}
	if discarded.ReviewDraftSnapshot != nil || discarded.ReviewDraftRevision != 2 {
		t.Fatalf("discarded review draft = (revision:%d snapshot:%s)", discarded.ReviewDraftRevision, discarded.ReviewDraftSnapshot)
	}
}

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

	_, stateToken, err := service.GithubStart(ctx, "/dashboard", false)
	if err != nil {
		t.Fatalf("start GitHub login: %v", err)
	}
	githubRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://api.example.test/auth/github/callback", nil)
	githubUser, githubTokens, _, err := service.GithubCallback(ctx, githubRequest, "oauth-code", stateToken, stateToken)
	if err != nil {
		t.Fatalf("complete GitHub login: %v", err)
	}
	if !githubUser.EmailVerified {
		t.Fatal("new GitHub user email was not marked verified")
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

func bootstrapDatabaseRoles(ctx context.Context, t *testing.T, connection *pgx.Conn) {
	t.Helper()
	if _, err := connection.Exec(ctx, `
		CREATE EXTENSION IF NOT EXISTS age;
		COMMENT ON EXTENSION age IS
			'Apache AGE provides the authoritative directed site friend-link graph.';
		CREATE ROLE migrator
			LOGIN PASSWORD 'migrator-secret'
			NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
		CREATE ROLE api_runtime
			LOGIN PASSWORD 'runtime-secret'
			NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
		ALTER ROLE migrator SET session_preload_libraries = 'age';
		ALTER ROLE api_runtime SET session_preload_libraries = 'age';
		REVOKE ALL ON DATABASE heyblog FROM PUBLIC;
		GRANT CONNECT, CREATE ON DATABASE heyblog TO migrator;
		GRANT CONNECT ON DATABASE heyblog TO api_runtime;
		GRANT USAGE ON SCHEMA ag_catalog TO migrator;
		CREATE SCHEMA migration AUTHORIZATION migrator;
		COMMENT ON SCHEMA migration IS 'Goose migration history owned by the migration role.';
	`); err != nil {
		t.Fatalf("bootstrap database roles: %v", err)
	}
}

func verifyRoleBoundaries(ctx context.Context, t *testing.T, connection *pgx.Conn) {
	t.Helper()
	for _, role := range []string{"migrator", "api_runtime"} {
		var canLogin, isSuperuser, canCreateDatabase, canCreateRole, canReplicate, canBypassRLS bool
		if err := connection.QueryRow(ctx, `
			SELECT rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolbypassrls
			  FROM pg_roles
			 WHERE rolname = $1
		`, role).Scan(
			&canLogin,
			&isSuperuser,
			&canCreateDatabase,
			&canCreateRole,
			&canReplicate,
			&canBypassRLS,
		); err != nil {
			t.Fatalf("query %s role: %v", role, err)
		}
		if !canLogin || isSuperuser || canCreateDatabase || canCreateRole || canReplicate || canBypassRLS {
			t.Fatalf(
				"role %s attributes = login:%t super:%t createdb:%t createrole:%t replication:%t bypassrls:%t",
				role,
				canLogin,
				isSuperuser,
				canCreateDatabase,
				canCreateRole,
				canReplicate,
				canBypassRLS,
			)
		}
	}

	var migratorCanCreate, runtimeCanCreate, runtimeCanConnect, runtimeCanUseAGE bool
	if err := connection.QueryRow(ctx, `
		SELECT has_database_privilege('migrator', 'heyblog', 'CREATE'),
		       has_database_privilege('api_runtime', 'heyblog', 'CREATE'),
		       has_database_privilege('api_runtime', 'heyblog', 'CONNECT'),
		       has_schema_privilege('api_runtime', 'ag_catalog', 'USAGE')
	`).Scan(&migratorCanCreate, &runtimeCanCreate, &runtimeCanConnect, &runtimeCanUseAGE); err != nil {
		t.Fatalf("query database role privileges: %v", err)
	}
	if !migratorCanCreate || runtimeCanCreate || !runtimeCanConnect || runtimeCanUseAGE {
		t.Fatalf(
			"database privileges = migrator create:%t, runtime create:%t connect:%t AGE usage:%t",
			migratorCanCreate,
			runtimeCanCreate,
			runtimeCanConnect,
			runtimeCanUseAGE,
		)
	}
}

func verifyDatabaseCatalog(ctx context.Context, t *testing.T, connection *pgx.Conn) {
	t.Helper()

	var extensionVersion string
	if err := connection.QueryRow(ctx, "SELECT extversion FROM pg_extension WHERE extname = 'age'").Scan(&extensionVersion); err != nil {
		t.Fatalf("query AGE extension: %v", err)
	}

	var businessSchemaCount int
	if err := connection.QueryRow(ctx, `
		SELECT count(*) FROM pg_namespace
		WHERE nspname = ANY($1::text[])
	`, []string{"identity", "directory", "content"}).Scan(&businessSchemaCount); err != nil {
		t.Fatalf("query business schemas: %v", err)
	}
	if businessSchemaCount != 3 {
		t.Fatalf("business schema count = %d, want 3", businessSchemaCount)
	}

	var tableCount int
	if err := connection.QueryRow(ctx, `
		SELECT count(*)
		  FROM pg_class AS relation
		  JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
		 WHERE namespace.nspname = ANY($1::text[])
		   AND relation.relkind IN ('r', 'p')
	`, []string{"identity", "directory", "content"}).Scan(&tableCount); err != nil {
		t.Fatalf("query business tables: %v", err)
	}
	if tableCount != 19 {
		t.Fatalf("business table count = %d, want 19", tableCount)
	}

	var graphExists bool
	if err := connection.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM ag_catalog.ag_graph WHERE name = 'directory_graph')
	`).Scan(&graphExists); err != nil {
		t.Fatalf("query AGE graph: %v", err)
	}
	if !graphExists {
		t.Fatal("directory_graph AGE graph does not exist")
	}

	verifyCatalogComments(ctx, t, connection)
	verifyIdentitySchema(ctx, t, connection)
	verifyContentSchema(ctx, t, connection)
}

func verifyContentSchema(ctx context.Context, t *testing.T, connection *pgx.Conn) {
	t.Helper()

	var revisionStatusColumns int
	if err := connection.QueryRow(ctx, `
		SELECT count(*)
		  FROM information_schema.columns
		 WHERE table_schema = 'content'
		   AND table_name = 'announcement_revisions'
		   AND column_name = 'status'
	`).Scan(&revisionStatusColumns); err != nil {
		t.Fatalf("query announcement revision status column: %v", err)
	}
	if revisionStatusColumns != 0 {
		t.Fatalf("announcement revision status column count = %d, want 0", revisionStatusColumns)
	}

	wantConstraints := []string{
		"announcement_revisions_action_external_url_check",
		"announcement_revisions_action_label_check",
		"announcement_revisions_action_path_check",
	}
	rows, err := connection.Query(ctx, `
		SELECT constraint_name
		  FROM information_schema.table_constraints
		 WHERE table_schema = 'content'
		   AND table_name = 'announcement_revisions'
		   AND constraint_name = ANY($1::text[])
		 ORDER BY constraint_name
	`, wantConstraints)
	if err != nil {
		t.Fatalf("query announcement revision action constraints: %v", err)
	}
	constraints, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect announcement revision action constraints: %v", err)
	}
	if !slices.Equal(constraints, wantConstraints) {
		t.Fatalf("announcement revision action constraints = %v, want %v", constraints, wantConstraints)
	}
}

func verifyIdentitySchema(ctx context.Context, t *testing.T, connection *pgx.Conn) {
	t.Helper()

	rows, err := connection.Query(ctx, `
		SELECT column_name || ':' || data_type || ':' || is_nullable
		  FROM information_schema.columns
		 WHERE table_schema = 'identity'
		   AND table_name = 'users'
		 ORDER BY ordinal_position
	`)
	if err != nil {
		t.Fatalf("query identity user columns: %v", err)
	}
	columns, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect identity user columns: %v", err)
	}

	want := []string{
		"id:uuid:NO",
		"email:text:YES",
		"username:text:NO",
		"display_name:text:NO",
		"password_hash:text:YES",
		"role:text:NO",
		"access_status:text:NO",
		"email_verified_at:timestamp with time zone:YES",
		"auth_version:integer:NO",
		"profile:jsonb:NO",
		"settings:jsonb:NO",
		"last_login_at:timestamp with time zone:YES",
		"deletion_requested_at:timestamp with time zone:YES",
		"deletion_scheduled_for:timestamp with time zone:YES",
		"deleted_at:timestamp with time zone:YES",
		"created_at:timestamp with time zone:NO",
		"updated_at:timestamp with time zone:NO",
	}
	if !slices.Equal(columns, want) {
		t.Fatalf("identity user columns = %v, want %v", columns, want)
	}
}

func verifyCatalogComments(ctx context.Context, t *testing.T, connection *pgx.Conn) {
	t.Helper()
	schemas := []string{"identity", "directory", "content"}

	var undocumentedRelations int
	if err := connection.QueryRow(ctx, `
		SELECT count(*)
		  FROM pg_class AS relation
		  JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
		  LEFT JOIN pg_description AS description
		    ON description.objoid = relation.oid AND description.objsubid = 0
		 WHERE namespace.nspname = ANY($1::text[])
		   AND relation.relkind IN ('r', 'v', 'p')
		   AND description.description IS NULL
	`, schemas).Scan(&undocumentedRelations); err != nil {
		t.Fatalf("query undocumented business relations: %v", err)
	}
	if undocumentedRelations != 0 {
		t.Fatalf("undocumented business relation count = %d, want 0", undocumentedRelations)
	}

	var undocumentedColumns int
	if err := connection.QueryRow(ctx, `
		SELECT count(*)
		  FROM pg_class AS relation
		  JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
		  JOIN pg_attribute AS attribute
		    ON attribute.attrelid = relation.oid
		   AND attribute.attnum > 0
		   AND NOT attribute.attisdropped
		  LEFT JOIN pg_description AS description
		    ON description.objoid = relation.oid AND description.objsubid = attribute.attnum
		 WHERE namespace.nspname = ANY($1::text[])
		   AND relation.relkind IN ('r', 'v', 'p')
		   AND description.description IS NULL
	`, schemas).Scan(&undocumentedColumns); err != nil {
		t.Fatalf("query undocumented business columns: %v", err)
	}
	if undocumentedColumns != 0 {
		t.Fatalf("undocumented business column count = %d, want 0", undocumentedColumns)
	}

	var undocumentedFunctions int
	if err := connection.QueryRow(ctx, `
		SELECT count(*)
		  FROM pg_proc AS routine
		  JOIN pg_namespace AS namespace ON namespace.oid = routine.pronamespace
		 WHERE namespace.nspname = ANY($1::text[])
		   AND obj_description(routine.oid, 'pg_proc') IS NULL
	`, schemas).Scan(&undocumentedFunctions); err != nil {
		t.Fatalf("query undocumented business functions: %v", err)
	}
	if undocumentedFunctions != 0 {
		t.Fatalf("undocumented business function count = %d, want 0", undocumentedFunctions)
	}

	var undocumentedTriggers int
	if err := connection.QueryRow(ctx, `
		SELECT count(*)
		  FROM pg_trigger AS trigger
		  JOIN pg_class AS relation ON relation.oid = trigger.tgrelid
		  JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
		 WHERE namespace.nspname = ANY($1::text[])
		   AND NOT trigger.tgisinternal
		   AND obj_description(trigger.oid, 'pg_trigger') IS NULL
	`, schemas).Scan(&undocumentedTriggers); err != nil {
		t.Fatalf("query undocumented business triggers: %v", err)
	}
	if undocumentedTriggers != 0 {
		t.Fatalf("undocumented business trigger count = %d, want 0", undocumentedTriggers)
	}
}

func verifyDirectoryConstraints(ctx context.Context, t *testing.T, connection *pgxpool.Pool) {
	t.Helper()

	siteID := insertSite(ctx, t, connection, "0Aa1Bb2Cc", "Example Blog", "example.com")
	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_feeds (
			site_id, name, location_type, url_ref, url_key, is_default
		) VALUES ($1, 'Default', 'RELATIVE', '/feed.xml', '/feed.xml', true)
	`, siteID); err != nil {
		t.Fatalf("insert default feed: %v", err)
	}

	feedlessSiteID := insertSite(ctx, t, connection, "1Aa2Bb3Cc", "Feed Constraint", "feed.example.com")
	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_feeds (
			site_id, name, location_type, url_ref, url_key, is_default
		) VALUES ($1, 'Not Default', 'RELATIVE', '/feed.xml', '/feed.xml', false)
	`, feedlessSiteID); err == nil {
		t.Fatal("enabled feed without a default unexpectedly succeeded")
	}

	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.sites (short_id, custom_id, name, normalized_host)
		VALUES ('2Aa3Bb4Cc', 'bad--id', 'Invalid Custom ID', 'invalid-custom.example.com')
	`); err == nil {
		t.Fatal("invalid custom ID unexpectedly succeeded")
	}

	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.sites (short_id, name, normalized_host)
		VALUES ('3Aa4Bb5Cc', 'Duplicate Host', 'example.com')
	`); err == nil {
		t.Fatal("duplicate normalized host unexpectedly succeeded")
	}

}

func verifyPublicViewQueries(ctx context.Context, t *testing.T, connection *pgxpool.Pool) {
	t.Helper()
	queries := dbgen.New(connection)

	countBefore, err := queries.CountVisibleSites(ctx)
	if err != nil {
		t.Fatalf("count visible sites before fixture: %v", err)
	}
	visibleSiteID := insertSite(ctx, t, connection, "6Pv7Qw8Er", "Public Query", "public-query.example.com")
	hiddenSiteID := insertSite(ctx, t, connection, "7Pv8Qw9Er", "Hidden Query", "hidden-query.example.com")
	if _, err := connection.Exec(ctx, `
		UPDATE directory.sites
		   SET visibility = 'HIDDEN', visibility_reason = 'integration fixture'
		 WHERE id = $1
	`, hiddenSiteID); err != nil {
		t.Fatalf("hide public query fixture: %v", err)
	}
	countAfter, err := queries.CountVisibleSites(ctx)
	if err != nil {
		t.Fatalf("count visible sites after fixture: %v", err)
	}
	if countAfter != countBefore+1 {
		t.Fatalf("visible site count = %d, want %d", countAfter, countBefore+1)
	}
	randomSites, err := queries.ListRandomVisibleSites(ctx, int32(countAfter)) // #nosec G115 -- isolated fixtures keep the test count small.
	if err != nil {
		t.Fatalf("list random visible sites: %v", err)
	}
	var foundVisible bool
	for _, candidate := range randomSites {
		if candidate.ID == hiddenSiteID {
			t.Fatal("random visible sites included a hidden site")
		}
		if candidate.ID == visibleSiteID {
			foundVisible = true
		}
	}
	if !foundVisible {
		t.Fatal("random visible sites omitted a visible site within a full-size result")
	}

	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_feeds (
			site_id, name, location_type, url_ref, url_key, format, is_enabled, is_default
		) VALUES ($1, 'Public feed', 'RELATIVE', '/feed.xml', '/feed.xml', 'ATOM', true, true)
	`, visibleSiteID); err != nil {
		t.Fatalf("insert public feed fixture: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_feeds (
			site_id, name, location_type, url_ref, url_key, format, is_enabled, is_default
		) VALUES ($1, 'Disabled feed', 'RELATIVE', '/disabled.xml', '/disabled.xml', 'RSS', false, false)
	`, visibleSiteID); err != nil {
		t.Fatalf("insert disabled feed fixture: %v", err)
	}
	feeds, err := queries.ListPublicSiteFeeds(ctx, visibleSiteID)
	if err != nil {
		t.Fatalf("list public site feeds: %v", err)
	}
	if len(feeds) != 1 || feeds[0].Name != "Public feed" || !feeds[0].IsEnabled {
		t.Fatalf("public site feeds = %#v", feeds)
	}
	batchFeeds, err := queries.ListDefaultPublicSiteFeedsBySiteIDs(
		ctx,
		[]pgtype.UUID{visibleSiteID, hiddenSiteID},
	)
	if err != nil {
		t.Fatalf("list default public feeds by site IDs: %v", err)
	}
	if len(batchFeeds) != 1 || batchFeeds[0].SiteID != visibleSiteID || !batchFeeds[0].IsDefault {
		t.Fatalf("default public site feeds = %#v", batchFeeds)
	}

	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_resources (
			site_id, kind, location_type, url_ref, url_key
		) VALUES ($1, 'SITEMAP', 'RELATIVE', '/sitemap.xml', '/sitemap.xml')
	`, visibleSiteID); err != nil {
		t.Fatalf("insert public sitemap fixture: %v", err)
	}
	sitemaps, err := queries.ListPublicSitemapsBySiteIDs(ctx, []pgtype.UUID{visibleSiteID, hiddenSiteID})
	if err != nil {
		t.Fatalf("list public sitemaps by site IDs: %v", err)
	}
	if len(sitemaps) != 1 || sitemaps[0].SiteID != visibleSiteID || sitemaps[0].Kind != "SITEMAP" {
		t.Fatalf("public site sitemaps = %#v", sitemaps)
	}

	var enabledTagID, disabledTagID, mergedTagID, canonicalTagID pgtype.UUID
	for _, fixture := range []struct {
		name           string
		normalizedName string
		slug           string
		enabled        bool
		id             *pgtype.UUID
	}{
		{name: "Public Topic", normalizedName: "public topic", slug: "public-topic", enabled: true, id: &enabledTagID},
		{name: "Disabled Topic", normalizedName: "disabled topic", slug: "disabled-topic", enabled: false, id: &disabledTagID},
		{name: "Merged Topic", normalizedName: "merged topic", slug: "merged-topic", enabled: true, id: &mergedTagID},
		{name: "Canonical Topic", normalizedName: "canonical topic", slug: "canonical-topic", enabled: true, id: &canonicalTagID},
	} {
		if err := connection.QueryRow(ctx, `
			INSERT INTO directory.tags (name, normalized_name, slug, is_enabled)
			VALUES ($1, $2, $3, $4)
			RETURNING id
		`, fixture.name, fixture.normalizedName, fixture.slug, fixture.enabled).Scan(fixture.id); err != nil {
			t.Fatalf("insert tag fixture %q: %v", fixture.name, err)
		}
	}
	for _, tagID := range []pgtype.UUID{enabledTagID, disabledTagID, mergedTagID} {
		if _, err := connection.Exec(ctx, `
			INSERT INTO directory.site_tags (site_id, tag_id, role)
			VALUES ($1, $2, 'WARNING')
		`, visibleSiteID, tagID); err != nil {
			t.Fatalf("assign public view tag fixture: %v", err)
		}
	}
	if _, err := connection.Exec(ctx, `
		UPDATE directory.tags
		   SET merged_into_id = $2, merged_at = clock_timestamp()
		 WHERE id = $1
	`, mergedTagID, canonicalTagID); err != nil {
		t.Fatalf("merge public view tag fixture: %v", err)
	}
	tags, err := queries.ListPublicSiteTags(ctx, visibleSiteID)
	if err != nil {
		t.Fatalf("list public site tags: %v", err)
	}
	if len(tags) != 1 || tags[0].TagID != enabledTagID || tags[0].Name != "Public Topic" {
		t.Fatalf("public site tags = %#v", tags)
	}
	batchTags, err := queries.ListPublicSiteTagsBySiteIDs(ctx, []pgtype.UUID{visibleSiteID, hiddenSiteID})
	if err != nil {
		t.Fatalf("list public site tags by site IDs: %v", err)
	}
	if len(batchTags) != 1 || batchTags[0].SiteID != visibleSiteID || batchTags[0].TagID != enabledTagID {
		t.Fatalf("batch public site tags = %#v", batchTags)
	}

	var enabledComponentID, disabledComponentID pgtype.UUID
	for _, fixture := range []struct {
		name           string
		normalizedName string
		enabled        bool
		id             *pgtype.UUID
	}{
		{name: "Public Runtime", normalizedName: "public runtime", enabled: true, id: &enabledComponentID},
		{name: "Disabled Runtime", normalizedName: "disabled runtime", enabled: false, id: &disabledComponentID},
	} {
		if err := connection.QueryRow(ctx, `
			INSERT INTO directory.software_components (name, normalized_name, is_enabled)
			VALUES ($1, $2, $3)
			RETURNING id
		`, fixture.name, fixture.normalizedName, fixture.enabled).Scan(fixture.id); err != nil {
			t.Fatalf("insert software fixture %q: %v", fixture.name, err)
		}
	}
	for _, componentID := range []pgtype.UUID{enabledComponentID, disabledComponentID} {
		if _, err := connection.Exec(ctx, `
			INSERT INTO directory.site_software_components (
				site_id, component_id, role, evidence_source
			) VALUES ($1, $2, 'RUNTIME', 'MANUAL')
		`, visibleSiteID, componentID); err != nil {
			t.Fatalf("assign public view software fixture: %v", err)
		}
	}
	technologies, err := queries.ListPublicSiteSoftwareComponents(ctx, visibleSiteID)
	if err != nil {
		t.Fatalf("list public site software components: %v", err)
	}
	if len(technologies) != 1 || technologies[0].ComponentID != enabledComponentID || technologies[0].Name != "Public Runtime" {
		t.Fatalf("public site software components = %#v", technologies)
	}
}

func verifyAnnouncementConstraints(
	ctx context.Context,
	t *testing.T,
	connection *pgxpool.Pool,
	migrationURL string,
) {
	t.Helper()

	var actorID pgtype.UUID
	if err := connection.QueryRow(ctx, `
		INSERT INTO identity.users (email, username, display_name)
		VALUES ('announcement@example.com', 'announcement_admin', 'Announcement Admin')
		RETURNING id
	`).Scan(&actorID); err != nil {
		t.Fatalf("create announcement actor: %v", err)
	}

	now := time.Now().UTC()
	activeStart := now.Add(-2 * time.Hour)
	activeEnd := now.Add(2 * time.Hour)
	highPriorityID, highPriorityVersion := insertAnnouncement(
		ctx,
		t,
		connection,
		actorID,
		"MAIN",
		"High priority announcement",
		20,
		activeStart,
		&activeEnd,
	)
	_, _ = insertAnnouncement(
		ctx,
		t,
		connection,
		actorID,
		"MAIN",
		"Lower priority announcement",
		10,
		activeStart.Add(time.Minute),
		&activeEnd,
	)
	leading, err := dbgen.New(connection).GetLeadingActiveMainAnnouncement(ctx)
	if err != nil {
		t.Fatalf("get leading active main announcement: %v", err)
	}
	if leading.ID != highPriorityID || leading.Title != "High priority announcement" {
		t.Fatalf("leading active main announcement = %#v", leading)
	}

	rows, err := connection.Query(ctx, `
		SELECT title
		  FROM content.announcements
		 WHERE kind = 'MAIN'
		   AND status = 'PUBLISHED'
		   AND starts_at <= clock_timestamp()
		   AND (ends_at IS NULL OR ends_at > clock_timestamp())
		 ORDER BY priority DESC, starts_at DESC, id DESC
	`)
	if err != nil {
		t.Fatalf("query active main announcements: %v", err)
	}
	mainTitles, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect active main announcements: %v", err)
	}
	if !slices.Equal(mainTitles, []string{"High priority announcement", "Lower priority announcement"}) {
		t.Fatalf("active main announcement titles = %v", mainTitles)
	}

	bannerEnd := now.Add(time.Hour)
	_, _ = insertAnnouncement(
		ctx,
		t,
		connection,
		actorID,
		"BANNER",
		"Current banner",
		0,
		now.Add(-time.Hour),
		&bannerEnd,
	)
	if _, err := connection.Exec(ctx, `
		INSERT INTO content.announcements (
			kind, title, status, starts_at, ends_at, published_at,
			created_by, updated_by, published_by
		) VALUES ('BANNER', 'Overlapping banner', 'PUBLISHED', $1, $2, $3, $4, $4, $4)
	`, now, now.Add(30*time.Minute), now.Add(-time.Hour), actorID); err == nil {
		t.Fatal("overlapping published banner unexpectedly succeeded")
	}

	adjacentBannerID, adjacentBannerVersion := insertAnnouncement(
		ctx,
		t,
		connection,
		actorID,
		"BANNER",
		"Adjacent scheduled banner",
		0,
		bannerEnd,
		timePointer(bannerEnd.Add(time.Hour)),
	)
	if _, err := connection.Exec(ctx, `
		UPDATE content.announcements
		   SET title = 'Adjusted scheduled banner', updated_by = $2
		 WHERE id = $1 AND row_version = $3
	`, adjacentBannerID, actorID, adjacentBannerVersion); err != nil {
		t.Fatalf("update scheduled banner before public window: %v", err)
	}
	var scheduledRevisionCount int
	if err := connection.QueryRow(ctx, `
		SELECT count(*) FROM content.announcement_revisions WHERE announcement_id = $1
	`, adjacentBannerID).Scan(&scheduledRevisionCount); err != nil {
		t.Fatalf("count scheduled banner revisions: %v", err)
	}
	if scheduledRevisionCount != 0 {
		t.Fatalf("scheduled banner revision count = %d, want 0", scheduledRevisionCount)
	}
	if _, err := connection.Exec(ctx, `
		DELETE FROM content.announcements WHERE id = $1
	`, adjacentBannerID); err == nil {
		t.Fatal("scheduled announcement hard deletion unexpectedly succeeded")
	}

	if _, err := connection.Exec(ctx, `
		INSERT INTO content.announcements (
			kind, title, status, action_type, action_label, action_external_url,
			created_by, updated_by
		) VALUES ('MAIN', 'Invalid internal action', 'DRAFT', 'INTERNAL', 'Read',
		          'https://example.com', $1, $1)
	`, actorID); err == nil {
		t.Fatal("internal action with external URL unexpectedly succeeded")
	}
	verifyAnnouncementRevisionActionConstraints(ctx, t, migrationURL, highPriorityID, actorID)

	var draftID pgtype.UUID
	if err := connection.QueryRow(ctx, `
		INSERT INTO content.announcements (
			kind, title, body_markdown, status, action_type, action_label,
			action_external_url, created_by, updated_by
		) VALUES ('MAIN', 'External action draft', 'Read [more](https://example.com)',
		          'DRAFT', 'EXTERNAL', 'Open', 'https://example.com', $1, $1)
		RETURNING id
	`, actorID).Scan(&draftID); err != nil {
		t.Fatalf("insert draft announcement with external action: %v", err)
	}
	deleteResult, err := connection.Exec(ctx, `DELETE FROM content.announcements WHERE id = $1`, draftID)
	if err != nil {
		t.Fatalf("delete draft announcement: %v", err)
	}
	if deleteResult.RowsAffected() != 1 {
		t.Fatalf("deleted draft rows = %d, want 1", deleteResult.RowsAffected())
	}

	var updatedVersion int64
	if err := connection.QueryRow(ctx, `
		UPDATE content.announcements
		   SET title = 'Corrected announcement',
		       body_markdown = 'Read **the correction**.',
		       action_type = 'INTERNAL',
		       action_label = 'Details',
		       action_path = '/announcements/correction',
		       updated_by = $2
		 WHERE id = $1 AND row_version = $3
		RETURNING row_version
	`, highPriorityID, actorID, highPriorityVersion).Scan(&updatedVersion); err != nil {
		t.Fatalf("update public announcement: %v", err)
	}
	if updatedVersion != highPriorityVersion+1 {
		t.Fatalf("updated row version = %d, want %d", updatedVersion, highPriorityVersion+1)
	}

	var revisionTitle string
	var revision int64
	if err := connection.QueryRow(ctx, `
		SELECT title, revision
		  FROM content.announcement_revisions
		 WHERE announcement_id = $1
	`, highPriorityID).Scan(&revisionTitle, &revision); err != nil {
		t.Fatalf("query public announcement revision: %v", err)
	}
	if revisionTitle != "High priority announcement" || revision != highPriorityVersion {
		t.Fatalf("public announcement revision = (%q, %d)", revisionTitle, revision)
	}

	if _, err := connection.Exec(ctx, `
		UPDATE content.announcements SET status = 'DRAFT', updated_by = $2 WHERE id = $1
	`, highPriorityID, actorID); err == nil {
		t.Fatal("published announcement revert to draft unexpectedly succeeded")
	}
	if _, err := connection.Exec(ctx, `
		UPDATE content.announcements SET kind = 'BANNER', priority = 0, updated_by = $2 WHERE id = $1
	`, highPriorityID, actorID); err == nil {
		t.Fatal("public announcement kind change unexpectedly succeeded")
	}

	if _, err := connection.Exec(ctx, `
		UPDATE content.announcements
		   SET status = 'ARCHIVED', archived_at = clock_timestamp(),
		       archived_by = $2, updated_by = $2
		 WHERE id = $1 AND row_version = $3
	`, highPriorityID, actorID, updatedVersion); err != nil {
		t.Fatalf("archive public announcement: %v", err)
	}
	var revisionCount int
	if err := connection.QueryRow(ctx, `
		SELECT count(*) FROM content.announcement_revisions WHERE announcement_id = $1
	`, highPriorityID).Scan(&revisionCount); err != nil {
		t.Fatalf("count archived announcement revisions: %v", err)
	}
	if revisionCount != 2 {
		t.Fatalf("archived announcement revision count = %d, want 2", revisionCount)
	}
	if _, err := connection.Exec(ctx, `
		UPDATE content.announcements SET title = 'Tampered archive', updated_by = $2 WHERE id = $1
	`, highPriorityID, actorID); err == nil {
		t.Fatal("archived announcement update unexpectedly succeeded")
	}
	if _, err := connection.Exec(ctx, `DELETE FROM content.announcements WHERE id = $1`, highPriorityID); err == nil {
		t.Fatal("archived announcement hard deletion unexpectedly succeeded")
	}
	if _, err := connection.Exec(ctx, `
		UPDATE content.announcement_revisions
		   SET title = 'Tampered revision'
		 WHERE announcement_id = $1
	`, highPriorityID); err == nil {
		t.Fatal("runtime revision update unexpectedly succeeded")
	}
	if _, err := connection.Exec(ctx, `
		DELETE FROM content.announcement_revisions WHERE announcement_id = $1
	`, highPriorityID); err == nil {
		t.Fatal("runtime revision deletion unexpectedly succeeded")
	}

	concurrentStart := now.Add(3 * time.Hour)
	concurrentEnd := now.Add(4 * time.Hour)
	results := make(chan error, 2)
	for _, title := range []string{"Concurrent banner A", "Concurrent banner B"} {
		go func(title string) {
			_, insertErr := connection.Exec(ctx, `
				INSERT INTO content.announcements (
					kind, title, status, starts_at, ends_at, published_at,
					created_by, updated_by, published_by
				) VALUES ('BANNER', $1, 'PUBLISHED', $2, $3, $4, $5, $5, $5)
			`, title, concurrentStart, concurrentEnd, now, actorID)
			results <- insertErr
		}(title)
	}
	concurrentSuccesses := 0
	for range 2 {
		if insertErr := <-results; insertErr == nil {
			concurrentSuccesses++
		}
	}
	if concurrentSuccesses != 1 {
		t.Fatalf("concurrent banner successes = %d, want 1", concurrentSuccesses)
	}
}

func verifyAnnouncementRevisionActionConstraints(
	ctx context.Context,
	t *testing.T,
	migrationURL string,
	announcementID pgtype.UUID,
	actorID pgtype.UUID,
) {
	t.Helper()
	connection, err := pgx.Connect(ctx, migrationURL)
	if err != nil {
		t.Fatalf("connect as migrator for announcement revision constraints: %v", err)
	}
	defer func() { _ = connection.Close(context.Background()) }()

	tests := []struct {
		name        string
		revision    int64
		actionType  string
		label       *string
		path        *string
		externalURL *string
	}{
		{
			name: "blank label", revision: 1001, actionType: "INTERNAL",
			label: stringPointer(" "), path: stringPointer("/valid"),
		},
		{
			name: "invalid internal path", revision: 1002, actionType: "INTERNAL",
			label: stringPointer("Open"), path: stringPointer("//example.com"),
		},
		{
			name: "invalid external URL", revision: 1003, actionType: "EXTERNAL",
			label: stringPointer("Open"), externalURL: stringPointer("ftp://example.com"),
		},
	}
	for _, testCase := range tests {
		if _, err := connection.Exec(ctx, `
			INSERT INTO content.announcement_revisions (
				announcement_id, revision, kind, title, priority,
				action_type, action_label, action_path, action_external_url,
				starts_at, published_at, published_by, changed_by
			) VALUES ($1, $2, 'MAIN', 'Invalid action revision', 0,
			          $3, $4, $5, $6, clock_timestamp(), clock_timestamp(), $7, $7)
		`, announcementID, testCase.revision, testCase.actionType, testCase.label, testCase.path, testCase.externalURL, actorID); err == nil {
			t.Errorf("%s revision unexpectedly succeeded", testCase.name)
		}
	}
}

func verifyAnnouncementQueries(ctx context.Context, t *testing.T, connection *pgxpool.Pool) {
	t.Helper()
	queries := dbgen.New(connection)
	actor, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       "announcement-query@example.com",
		Username:    "announcement_query_admin",
		DisplayName: "Announcement Query Admin",
	})
	if err != nil {
		t.Fatalf("create announcement query actor: %v", err)
	}

	body := "Read **the release notes**."
	draft, err := queries.CreateAnnouncement(ctx, dbgen.CreateAnnouncementParams{
		Kind:         string(content.KindMain),
		Title:        "Release announcement",
		BodyMarkdown: &body,
		Priority:     50,
		ActionType:   string(content.ActionNone),
		ActorID:      actor.ID,
	})
	if err != nil {
		t.Fatalf("create announcement draft: %v", err)
	}
	if draft.Status != string(content.StatusDraft) || draft.RowVersion != 1 {
		t.Fatalf("created announcement = %#v", draft)
	}

	now := time.Now().UTC()
	published, err := queries.PublishAnnouncement(ctx, dbgen.PublishAnnouncementParams{
		ID:                 draft.ID,
		ExpectedRowVersion: draft.RowVersion,
		StartsAt:           pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true},
		EndsAt:             pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true},
		ActorID:            actor.ID,
	})
	if err != nil {
		t.Fatalf("publish announcement: %v", err)
	}
	if published.Status != string(content.StatusPublished) || published.RowVersion != 2 {
		t.Fatalf("published announcement = %#v", published)
	}

	activeMain, err := queries.ListActiveMainAnnouncements(ctx)
	if err != nil {
		t.Fatalf("list active main announcements: %v", err)
	}
	if len(activeMain) != 1 || activeMain[0].ID != published.ID {
		t.Fatalf("active main announcements = %#v", activeMain)
	}

	correctedTitle := "Corrected release announcement"
	actionLabel := "Open release"
	actionPath := "/releases/current"
	updated, err := queries.UpdateAnnouncement(ctx, dbgen.UpdateAnnouncementParams{
		ID:                 published.ID,
		ExpectedRowVersion: published.RowVersion,
		Kind:               published.Kind,
		Title:              correctedTitle,
		BodyMarkdown:       published.BodyMarkdown,
		Priority:           published.Priority,
		ActionType:         string(content.ActionInternal),
		ActionLabel:        &actionLabel,
		ActionPath:         &actionPath,
		StartsAt:           published.StartsAt,
		EndsAt:             published.EndsAt,
		ActorID:            actor.ID,
	})
	if err != nil {
		t.Fatalf("update announcement through sqlc: %v", err)
	}
	if updated.Title != correctedTitle || updated.RowVersion != 3 {
		t.Fatalf("updated announcement = %#v", updated)
	}

	revisions, err := queries.ListAnnouncementRevisions(ctx, published.ID)
	if err != nil {
		t.Fatalf("list announcement revisions: %v", err)
	}
	if len(revisions) != 1 || revisions[0].Title != "Release announcement" || revisions[0].Revision != 2 {
		t.Fatalf("announcement revisions = %#v", revisions)
	}

	bannerDraft, err := queries.CreateAnnouncement(ctx, dbgen.CreateAnnouncementParams{
		Kind:       string(content.KindBanner),
		Title:      "Current banner query",
		Priority:   0,
		ActionType: string(content.ActionNone),
		ActorID:    actor.ID,
	})
	if err != nil {
		t.Fatalf("create banner draft: %v", err)
	}
	banner, err := queries.PublishAnnouncement(ctx, dbgen.PublishAnnouncementParams{
		ID:                 bannerDraft.ID,
		ExpectedRowVersion: bannerDraft.RowVersion,
		StartsAt:           pgtype.Timestamptz{Time: now.Add(-30 * time.Minute), Valid: true},
		EndsAt:             pgtype.Timestamptz{Time: now.Add(30 * time.Minute), Valid: true},
		ActorID:            actor.ID,
	})
	if err != nil {
		t.Fatalf("publish banner: %v", err)
	}
	activeBanner, err := queries.GetActiveBannerAnnouncement(ctx)
	if err != nil {
		t.Fatalf("get active banner: %v", err)
	}
	if activeBanner.ID != banner.ID {
		t.Fatalf("active banner ID = %v, want %v", activeBanner.ID, banner.ID)
	}
	if _, err := queries.ArchiveAnnouncement(ctx, dbgen.ArchiveAnnouncementParams{
		ID:                 banner.ID,
		ExpectedRowVersion: banner.RowVersion,
		ActorID:            actor.ID,
	}); err != nil {
		t.Fatalf("archive banner: %v", err)
	}

	archived, err := queries.ArchiveAnnouncement(ctx, dbgen.ArchiveAnnouncementParams{
		ID:                 updated.ID,
		ExpectedRowVersion: updated.RowVersion,
		ActorID:            actor.ID,
	})
	if err != nil {
		t.Fatalf("archive main announcement: %v", err)
	}
	if archived.Status != string(content.StatusArchived) {
		t.Fatalf("archived announcement status = %q", archived.Status)
	}
	publicArchive, err := queries.ListPublicAnnouncementArchive(ctx, dbgen.ListPublicAnnouncementArchiveParams{
		PageSize:   20,
		PageOffset: 0,
	})
	if err != nil {
		t.Fatalf("list public announcement archive: %v", err)
	}
	if len(publicArchive) != 1 || publicArchive[0].ID != archived.ID {
		t.Fatalf("public announcement archive = %#v", publicArchive)
	}

	deletableDraft, err := queries.CreateAnnouncement(ctx, dbgen.CreateAnnouncementParams{
		Kind:       string(content.KindMain),
		Title:      "Delete this draft",
		Priority:   0,
		ActionType: string(content.ActionNone),
		ActorID:    actor.ID,
	})
	if err != nil {
		t.Fatalf("create deletable announcement draft: %v", err)
	}
	deletedRows, err := queries.DeleteDraftAnnouncement(ctx, deletableDraft.ID)
	if err != nil {
		t.Fatalf("delete announcement draft through sqlc: %v", err)
	}
	if deletedRows != 1 {
		t.Fatalf("deleted announcement rows = %d, want 1", deletedRows)
	}
}

func insertAnnouncement(
	ctx context.Context,
	t *testing.T,
	connection *pgxpool.Pool,
	actorID pgtype.UUID,
	kind content.Kind,
	title string,
	priority int32,
	startsAt time.Time,
	endsAt *time.Time,
) (pgtype.UUID, int64) {
	t.Helper()
	var announcementID pgtype.UUID
	var rowVersion int64
	if err := connection.QueryRow(ctx, `
		INSERT INTO content.announcements (
			kind, title, status, priority, starts_at, ends_at, published_at,
			created_by, updated_by, published_by
		) VALUES ($1, $2, 'PUBLISHED', $3, $4, $5, $6, $7, $7, $7)
		RETURNING id, row_version
	`, kind, title, priority, startsAt, endsAt, time.Now().UTC(), actorID).Scan(
		&announcementID,
		&rowVersion,
	); err != nil {
		t.Fatalf("insert %s announcement %q: %v", kind, title, err)
	}
	return announcementID, rowVersion
}

func timePointer(value time.Time) *time.Time {
	return &value
}

func stringPointer(value string) *string {
	return &value
}

func verifySoftwareComponentDependencies(ctx context.Context, t *testing.T, connection *pgxpool.Pool) {
	t.Helper()
	queries := dbgen.New(connection)

	softwareA := createSoftwareComponent(ctx, t, queries, "Software A", "software a")
	astro := createSoftwareComponent(ctx, t, queries, "Astro", "astro")
	nodejs := createSoftwareComponent(ctx, t, queries, "Node.js", "node.js")

	for _, dependency := range []dbgen.AddSoftwareComponentDependencyParams{
		{ComponentID: softwareA.ID, DependencyComponentID: astro.ID, Role: "FRAMEWORK"},
		{ComponentID: softwareA.ID, DependencyComponentID: nodejs.ID, Role: "RUNTIME"},
	} {
		if _, err := queries.AddSoftwareComponentDependency(ctx, dependency); err != nil {
			t.Fatalf("add software component dependency: %v", err)
		}
	}

	dependencies, err := queries.ListSoftwareComponentDependencies(ctx, softwareA.ID)
	if err != nil {
		t.Fatalf("list software component dependencies: %v", err)
	}
	if len(dependencies) != 2 ||
		dependencies[0].Name != "Astro" || dependencies[0].Role != "FRAMEWORK" ||
		dependencies[1].Name != "Node.js" || dependencies[1].Role != "RUNTIME" {
		t.Fatalf("software component dependencies = %#v", dependencies)
	}

	if _, err := queries.AddSoftwareComponentDependency(ctx, dbgen.AddSoftwareComponentDependencyParams{
		ComponentID:           softwareA.ID,
		DependencyComponentID: softwareA.ID,
		Role:                  "OTHER",
	}); err == nil {
		t.Fatal("self software component dependency unexpectedly succeeded")
	}

	transaction, err := connection.Begin(ctx)
	if err != nil {
		t.Fatalf("begin dependency cycle transaction: %v", err)
	}
	_, cycleErr := transaction.Exec(ctx, `
		INSERT INTO directory.software_component_dependencies (
			component_id, dependency_component_id, role
		) VALUES ($1, $2, 'OTHER')
	`, astro.ID, softwareA.ID)
	if err := transaction.Rollback(ctx); err != nil {
		t.Fatalf("rollback dependency cycle transaction: %v", err)
	}
	if cycleErr == nil {
		t.Fatal("indirect software component dependency cycle unexpectedly succeeded")
	}

	if _, err := connection.Exec(ctx, "DELETE FROM directory.software_components WHERE id = $1", astro.ID); err == nil {
		t.Fatal("referenced dependency component deletion unexpectedly succeeded")
	}
	if _, err := connection.Exec(ctx, "DELETE FROM directory.software_components WHERE id = $1", softwareA.ID); err != nil {
		t.Fatalf("delete component owning dependency relations: %v", err)
	}

	var dependencyCount int
	if err := connection.QueryRow(ctx, `
		SELECT count(*)
		  FROM directory.software_component_dependencies
		 WHERE component_id = $1
	`, softwareA.ID).Scan(&dependencyCount); err != nil {
		t.Fatalf("count deleted component dependencies: %v", err)
	}
	if dependencyCount != 0 {
		t.Fatalf("deleted component dependency count = %d, want 0", dependencyCount)
	}
}

func createSoftwareComponent(
	ctx context.Context,
	t *testing.T,
	queries *dbgen.Queries,
	name string,
	normalizedName string,
) dbgen.DirectorySoftwareComponent {
	t.Helper()
	component, err := queries.CreateSoftwareComponent(ctx, dbgen.CreateSoftwareComponentParams{
		Name:           name,
		NormalizedName: normalizedName,
		Description:    "",
		IsOpenSource:   true,
	})
	if err != nil {
		t.Fatalf("create software component %q: %v", name, err)
	}
	return component
}

func verifyAnnouncementActorDeletionSemantics(
	ctx context.Context,
	t *testing.T,
	runtimeConnection *pgxpool.Pool,
	migrationURL string,
) {
	t.Helper()
	queries := dbgen.New(runtimeConnection)
	actor, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       "deleted-announcement-actor@example.com",
		Username:    "deleted_announcement_actor",
		DisplayName: "Deleted Announcement Actor",
	})
	if err != nil {
		t.Fatalf("create deletable announcement actor: %v", err)
	}

	now := time.Now().UTC()
	announcementID, initialVersion := insertAnnouncement(
		ctx,
		t,
		runtimeConnection,
		actor.ID,
		"MAIN",
		"Announcement with deletable actor",
		0,
		now.Add(-time.Hour),
		timePointer(now.Add(time.Hour)),
	)
	var updatedVersion int64
	if err := runtimeConnection.QueryRow(ctx, `
		UPDATE content.announcements
		   SET title = 'Announcement with deleted actor', updated_by = $2
		 WHERE id = $1 AND row_version = $3
		RETURNING row_version
	`, announcementID, actor.ID, initialVersion).Scan(&updatedVersion); err != nil {
		t.Fatalf("update announcement before actor deletion: %v", err)
	}
	var archivedVersion int64
	if err := runtimeConnection.QueryRow(ctx, `
		UPDATE content.announcements
		   SET status = 'ARCHIVED', archived_at = clock_timestamp(),
		       archived_by = $2, updated_by = $2
		 WHERE id = $1 AND row_version = $3
		RETURNING row_version
	`, announcementID, actor.ID, updatedVersion).Scan(&archivedVersion); err != nil {
		t.Fatalf("archive announcement before actor deletion: %v", err)
	}

	migrationConnection, err := pgx.Connect(ctx, migrationURL)
	if err != nil {
		t.Fatalf("connect as migrator for announcement actor deletion: %v", err)
	}
	defer func() { _ = migrationConnection.Close(context.Background()) }()
	if _, err := migrationConnection.Exec(ctx, "DELETE FROM identity.users WHERE id = $1", actor.ID); err != nil {
		t.Fatalf("hard delete announcement actor as migrator: %v", err)
	}

	var actorReferencesCleared bool
	var currentVersion int64
	if err := migrationConnection.QueryRow(ctx, `
		SELECT created_by IS NULL
		       AND updated_by IS NULL
		       AND published_by IS NULL
		       AND archived_by IS NULL,
		       row_version
		  FROM content.announcements
		 WHERE id = $1
	`, announcementID).Scan(&actorReferencesCleared, &currentVersion); err != nil {
		t.Fatalf("query announcement after actor deletion: %v", err)
	}
	if !actorReferencesCleared {
		t.Fatal("announcement retained actor references after actor deletion")
	}
	if currentVersion != archivedVersion {
		t.Fatalf("announcement row version after actor deletion = %d, want %d", currentVersion, archivedVersion)
	}

	var revisionCount int
	var revisionActorReferencesCleared bool
	if err := migrationConnection.QueryRow(ctx, `
		SELECT count(*), bool_and(published_by IS NULL AND changed_by IS NULL)
		  FROM content.announcement_revisions
		 WHERE announcement_id = $1
	`, announcementID).Scan(&revisionCount, &revisionActorReferencesCleared); err != nil {
		t.Fatalf("query announcement revisions after actor deletion: %v", err)
	}
	if revisionCount != 2 || !revisionActorReferencesCleared {
		t.Fatalf(
			"announcement revisions after actor deletion = count:%d actors-cleared:%t",
			revisionCount,
			revisionActorReferencesCleared,
		)
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

func verifyMigrationRollback(
	ctx context.Context,
	t *testing.T,
	adminConnection *pgx.Conn,
	migrationURL string,
) {
	t.Helper()
	migrationConfig, err := pgx.ParseConfig(migrationURL)
	if err != nil {
		t.Fatalf("parse migration URL for rollback: %v", err)
	}
	migrationDB := stdlib.OpenDB(*migrationConfig)
	defer func() { _ = migrationDB.Close() }()
	migrationFS, err := migrations.Filesystem()
	if err != nil {
		t.Fatalf("open migrations for rollback: %v", err)
	}
	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		migrationDB,
		migrationFS,
		goose.WithTableName("migration.goose_db_version"),
	)
	if err != nil {
		t.Fatalf("create Goose rollback provider: %v", err)
	}
	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("roll migrations down to zero: %v", err)
	}

	var businessSchemaCount int
	if err := adminConnection.QueryRow(ctx, `
		SELECT count(*) FROM pg_namespace
		WHERE nspname = ANY($1::text[])
	`, []string{"identity", "directory", "content"}).Scan(&businessSchemaCount); err != nil {
		t.Fatalf("query business schemas after rollback: %v", err)
	}
	if businessSchemaCount != 0 {
		t.Fatalf("business schema count after rollback = %d, want 0", businessSchemaCount)
	}

	var graphExists bool
	if err := adminConnection.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM ag_catalog.ag_graph WHERE name = 'directory_graph')
	`).Scan(&graphExists); err != nil {
		t.Fatalf("query AGE graph after rollback: %v", err)
	}
	if graphExists {
		t.Fatal("directory_graph AGE graph remained after rollback")
	}

	if err := database.Migrate(ctx, migrationURL); err != nil {
		t.Fatalf("reapply migrations after rollback: %v", err)
	}
	verifyDatabaseCatalog(ctx, t, adminConnection)
}

func verifyFriendLinkGraph(
	ctx context.Context,
	t *testing.T,
	connection *pgxpool.Pool,
	adminConnection *pgx.Conn,
) {
	t.Helper()
	queries := dbgen.New(connection)

	sourceID := insertSite(ctx, t, connection, "4Aa5Bb6Cc", "Source", "source.example.com")
	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_feeds (
			site_id, name, location_type, url_ref, url_key, is_default
		) VALUES ($1, 'Default', 'RELATIVE', '/feed.xml', '/feed.xml', true)
	`, sourceID); err != nil {
		t.Fatalf("insert source feed: %v", err)
	}

	externalURL := "https://target.example.com/friends"
	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
		PSourceSiteID: sourceID,
		PTargetUrl:    externalURL,
		PTargetHost:   "target.example.com",
		PStatus:       "ACTIVE",
	}); err != nil {
		t.Fatalf("upsert external friend link: %v", err)
	}

	links := listFriendLinks(ctx, t, queries, sourceID, false)
	if len(links) != 1 || links[0].TargetSiteID.Valid || links[0].TargetUrl != externalURL {
		t.Fatalf("external friend links = %#v", links)
	}
	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
		PSourceSiteID: sourceID,
		PTargetUrl:    externalURL + "?from=source",
		PTargetHost:   "target.example.com",
		PStatus:       "ACTIVE",
	}); err == nil {
		t.Fatal("friend-link URL with query unexpectedly succeeded")
	}
	if _, err := connection.Exec(ctx, `
		UPDATE directory.sites
		   SET visibility = 'HIDDEN', visibility_reason = 'private'
		 WHERE id = $1
	`, sourceID); err != nil {
		t.Fatalf("hide source site: %v", err)
	}
	if links = listFriendLinks(ctx, t, queries, sourceID, true); len(links) != 0 {
		t.Fatalf("hidden source friend links remained public: %#v", links)
	}
	if _, err := connection.Exec(ctx, `
		UPDATE directory.sites
		   SET visibility = 'VISIBLE', visibility_reason = NULL
		 WHERE id = $1
	`, sourceID); err != nil {
		t.Fatalf("show source site: %v", err)
	}

	targetID := insertSite(ctx, t, connection, "5Aa6Bb7Cc", "Target", "target.example.com")
	links = listFriendLinks(ctx, t, queries, sourceID, false)
	if len(links) != 1 || !links[0].TargetSiteID.Valid || links[0].TargetSiteID != targetID ||
		links[0].TargetUrl != "https://target.example.com/" {
		t.Fatalf("promoted friend links = %#v", links)
	}
	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
		PSourceSiteID: sourceID,
		PTargetUrl:    "https://target.example.com/friends",
		PTargetHost:   "target.example.com",
		PStatus:       "ACTIVE",
	}); err == nil {
		t.Fatal("registered friend-link URL differing from the site address unexpectedly succeeded")
	}
	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
		PSourceSiteID: sourceID,
		PTargetUrl:    "https://target.example.com/",
		PTargetHost:   "target.example.com",
		PStatus:       "INACTIVE",
	}); err != nil {
		t.Fatalf("deactivate friend link: %v", err)
	}
	if links = listFriendLinks(ctx, t, queries, sourceID, false); len(links) != 0 {
		t.Fatalf("inactive friend links remained active: %#v", links)
	}
	links = listFriendLinks(ctx, t, queries, sourceID, true)
	if len(links) != 1 || links[0].LinkStatus != "INACTIVE" {
		t.Fatalf("inactive friend links = %#v", links)
	}
	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
		PSourceSiteID: sourceID,
		PTargetUrl:    "https://target.example.com/",
		PTargetHost:   "target.example.com",
		PStatus:       "ACTIVE",
	}); err != nil {
		t.Fatalf("reactivate friend link: %v", err)
	}
	if _, err := adminConnection.Exec(ctx, "LOAD 'age'"); err != nil {
		t.Fatalf("load AGE in admin session: %v", err)
	}
	futureEvent := time.Now().Add(time.Hour).UnixMilli()
	for _, event := range []struct {
		status      string
		updatedAtMS int64
	}{
		{status: "INACTIVE", updatedAtMS: futureEvent},
		{status: "ACTIVE", updatedAtMS: futureEvent - 1},
	} {
		if _, err := adminConnection.Exec(ctx, `
			SELECT directory.merge_friend_link_graph($1, $2, $3, $4, $5, $6)
		`, sourceID, "target.example.com", "https://target.example.com/", event.status, int64(1), event.updatedAtMS); err != nil {
			t.Fatalf("merge timestamped friend link: %v", err)
		}
	}
	links = listFriendLinks(ctx, t, queries, sourceID, true)
	if len(links) != 1 || links[0].LinkStatus != "INACTIVE" || links[0].CreatedAtMs != 1 ||
		links[0].UpdatedAtMs != futureEvent {
		t.Fatalf("latest timestamp friend links = %#v", links)
	}
	if _, err := adminConnection.Exec(ctx, `
		SELECT directory.merge_friend_link_graph($1, $2, $3, 'ACTIVE', $4, $5)
	`, sourceID, "target.example.com", "https://target.example.com/", int64(2), futureEvent); err != nil {
		t.Fatalf("merge equal timestamp friend link: %v", err)
	}
	links = listFriendLinks(ctx, t, queries, sourceID, false)
	if len(links) != 1 || links[0].LinkStatus != "ACTIVE" || links[0].CreatedAtMs != 1 ||
		links[0].UpdatedAtMs != futureEvent {
		t.Fatalf("equal timestamp friend links = %#v", links)
	}

	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
		PSourceSiteID: targetID,
		PTargetUrl:    "https://source.example.com/",
		PTargetHost:   "source.example.com",
		PStatus:       "ACTIVE",
	}); err != nil {
		t.Fatalf("upsert reciprocal friend link: %v", err)
	}
	links = listFriendLinks(ctx, t, queries, sourceID, false)
	if len(links) != 1 || !links[0].IsReciprocal {
		t.Fatalf("reciprocal friend links = %#v", links)
	}

	if _, err := connection.Exec(ctx, `
		UPDATE directory.sites
		   SET visibility = 'HIDDEN', visibility_reason = 'private'
		 WHERE id = $1
	`, targetID); err != nil {
		t.Fatalf("hide target site: %v", err)
	}
	if links = listFriendLinks(ctx, t, queries, sourceID, false); len(links) != 0 {
		t.Fatalf("hidden registered target remained public: %#v", links)
	}

	if _, err := connection.Exec(ctx, `
		UPDATE directory.sites
		   SET visibility = 'VISIBLE', visibility_reason = NULL,
		       scheme = 'http', normalized_host = 'moved.example.com', base_path = '/blog'
		 WHERE id = $1
	`, targetID); err != nil {
		t.Fatalf("move target site: %v", err)
	}
	links = listFriendLinks(ctx, t, queries, sourceID, false)
	if len(links) != 1 || links[0].TargetHost != "moved.example.com" ||
		links[0].TargetUrl != "http://moved.example.com/blog" {
		t.Fatalf("moved friend links = %#v", links)
	}

	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
		PSourceSiteID: sourceID,
		PTargetUrl:    "https://source.example.com/",
		PTargetHost:   "source.example.com",
		PStatus:       "ACTIVE",
	}); err == nil {
		t.Fatal("friend-link self edge unexpectedly succeeded")
	}

	if _, err := connection.Exec(ctx, "DELETE FROM directory.sites WHERE id = $1", targetID); err == nil {
		t.Fatal("hard site deletion unexpectedly succeeded")
	}

	var relativeFeed string
	if err := connection.QueryRow(ctx, `
		SELECT url_ref FROM directory.site_feeds WHERE site_id = $1 AND is_default
	`, sourceID).Scan(&relativeFeed); err != nil {
		t.Fatalf("query relative feed: %v", err)
	}
	if relativeFeed != "/feed.xml" {
		t.Fatalf("relative feed = %q, want /feed.xml", relativeFeed)
	}
}

func listFriendLinks(
	ctx context.Context,
	t *testing.T,
	queries *dbgen.Queries,
	sourceID pgtype.UUID,
	includeInactive bool,
) []dbgen.ListFriendLinksRow {
	t.Helper()
	links, err := queries.ListFriendLinks(ctx, dbgen.ListFriendLinksParams{
		PSourceSiteID:    sourceID,
		PIncludeInactive: includeInactive,
	})
	if err != nil {
		t.Fatalf("list friend links: %v", err)
	}
	return links
}

func insertSite(
	ctx context.Context,
	t *testing.T,
	connection *pgxpool.Pool,
	shortID string,
	name string,
	host string,
) pgtype.UUID {
	t.Helper()
	var siteID pgtype.UUID
	if err := connection.QueryRow(ctx, `
		INSERT INTO directory.sites (short_id, name, normalized_host)
		VALUES ($1, $2, $3)
		RETURNING id
	`, shortID, name, host).Scan(&siteID); err != nil {
		t.Fatalf("insert site %q: %v", host, err)
	}
	return siteID
}

func verifyRuntimePermissions(ctx context.Context, t *testing.T, connection *pgxpool.Pool) {
	t.Helper()
	if _, err := connection.Exec(ctx, "CREATE SCHEMA forbidden_runtime_schema"); err == nil {
		t.Fatal("runtime role unexpectedly received schema DDL permission")
	}
	if _, err := connection.Exec(ctx, `SELECT * FROM directory_graph."SiteRef"`); err == nil {
		t.Fatal("runtime role unexpectedly received direct graph table access")
	}
}

func TestRedisInfrastructure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := tcredis.Run(ctx, "redis:8.4-alpine")
	if err != nil {
		t.Fatalf("start Redis container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Errorf("terminate Redis container: %v", err)
		}
	})

	redisURL, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("get Redis connection string: %v", err)
	}
	client, err := cache.OpenRedis(ctx, config.RedisConfig{
		URL:          redisURL,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("open Redis client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if err := client.Set(ctx, "integration:redis", "ready", time.Minute).Err(); err != nil {
		t.Fatalf("write Redis key: %v", err)
	}
	if value, err := client.Get(ctx, "integration:redis").Result(); err != nil || value != "ready" {
		t.Fatalf("read Redis key = (%q, %v), want (%q, nil)", value, err, "ready")
	}

	limiter := ratelimit.New(client)
	concurrencyPolicy := ratelimit.Policy{
		Name:           "integration-concurrency",
		Capacity:       10,
		RefillTokens:   1,
		RefillInterval: time.Hour,
	}
	var allowed atomic.Int32
	var waitGroup sync.WaitGroup
	for range 20 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			decision, err := limiter.Allow(ctx, "concurrent-client", concurrencyPolicy)
			if err != nil {
				t.Errorf("concurrent limiter Allow() error = %v", err)
				return
			}
			if decision.Allowed {
				allowed.Add(1)
			}
		}()
	}
	waitGroup.Wait()
	if allowed.Load() != 10 {
		t.Fatalf("concurrent allowed requests = %d, want 10", allowed.Load())
	}

	refillPolicy := ratelimit.Policy{
		Name:           "integration-refill",
		Capacity:       1,
		RefillTokens:   1,
		RefillInterval: 25 * time.Millisecond,
	}
	first, err := limiter.Allow(ctx, "refill-client", refillPolicy)
	if err != nil || !first.Allowed {
		t.Fatalf("first refill decision = (%#v, %v), want allowed", first, err)
	}
	second, err := limiter.Allow(ctx, "refill-client", refillPolicy)
	if err != nil || second.Allowed {
		t.Fatalf("second refill decision = (%#v, %v), want denied", second, err)
	}
	time.Sleep(40 * time.Millisecond)
	third, err := limiter.Allow(ctx, "refill-client", refillPolicy)
	if err != nil || !third.Allowed {
		t.Fatalf("refilled decision = (%#v, %v), want allowed", third, err)
	}
}

func databaseURLForRole(t *testing.T, rawURL, username, password string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	parsed.User = url.UserPassword(username, password)
	return parsed.String()
}
