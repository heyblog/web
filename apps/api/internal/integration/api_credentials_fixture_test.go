//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"heyblog-api/internal/apikey"
	"heyblog-api/internal/application/publicview"
	"heyblog-api/internal/auth"
	"heyblog-api/internal/cache"
	"heyblog-api/internal/config"
	"heyblog-api/internal/database"
	"heyblog-api/internal/exampleapi"
	"heyblog-api/internal/httpapi"
	"heyblog-api/internal/mail"
)

const credentialWebToken = "credential-integration-web-token-123456"

type credentialFixture struct {
	pool    *pgxpool.Pool
	service *apikey.Service
	auth    *auth.Service
	router  *httpapi.Router
	actor   string
	access  string
}

func newCredentialFixture(t *testing.T) credentialFixture {
	t.Helper()
	ctx := t.Context()
	container, err := postgres.Run(ctx, "apache/age:release_PG18_1.7.0", postgres.WithDatabase("heyblog"), postgres.WithUsername("postgres"), postgres.WithPassword("postgres-secret"), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Error(err)
		}
	})
	adminURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	bootstrapDatabaseRoles(ctx, t, admin)
	if err := database.Migrate(ctx, databaseURLForRole(t, adminURL, "migrator", "migrator-secret")); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, databaseURLForRole(t, adminURL, "api_runtime", "runtime-secret"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	redisContainer, err := tcredis.Run(ctx, "redis:8.4-alpine")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(redisContainer); err != nil {
			t.Error(err)
		}
	})
	redisURL, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	redisClient, err := cache.OpenRedis(ctx, config.RedisConfig{URL: redisURL, DialTimeout: 3 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := redisClient.Close(); err != nil {
			t.Error(err)
		}
	})
	recorder := &authMailRecorder{}
	authService := auth.NewService(auth.Dependencies{Pool: pool, Redis: redisClient,
		VerificationMailer: mail.NewVerificationMailer(recorder, "verify@example.test", 10*time.Minute),
		Config:             auth.Config{AccessSecret: "credential-access-test-secret", RefreshSecret: "credential-refresh-test-secret", AccessTTL: time.Hour, RefreshTTL: time.Hour, VerificationTTL: time.Hour},
	})
	if err := authService.Register(ctx, "credential_admin", "credential_admin@example.test", "correct-password"); err != nil {
		t.Fatal(err)
	}
	code := regexp.MustCompile(`[0-9]{6}`).FindString(recorder.messages[0].Text)
	if err := authService.VerifyEmail(ctx, "credential_admin@example.test", code); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE identity.users SET role = 'SYS_ADMIN' WHERE username = 'credential_admin'"); err != nil {
		t.Fatal(err)
	}
	actor, tokens, err := authService.Login(ctx, "credential_admin", "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	service := apikey.NewService(apikey.NewRepository(pool), time.Now)
	router, err := httpapi.NewRouter(httpapi.Options{Mode: config.ModeDevelopment,
		HTTP:   config.HTTPConfig{MaxBodyBytes: 1 << 20, TrustedProxies: []string{}},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), WebToken: credentialWebToken,
		HealthcheckToken: "credential-integration-health-token", PublicViews: publicview.New(nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.RegisterAPIKeyManagementRoutes(router.API, authService, service, credentialWebToken, nil); err != nil {
		t.Fatal(err)
	}
	exampleapi.RegisterRoutes(router.API, service)
	return credentialFixture{pool: pool, service: service, auth: authService, router: router, actor: actor.ID, access: tokens[0]}
}

func (fixture credentialFixture) create(t *testing.T, audience apikey.Audience) apikey.Credential {
	t.Helper()
	expires := time.Now().Add(24 * time.Hour)
	credential, err := fixture.service.CreateClient(t.Context(), apikey.CreateClientRequest{
		Name: t.Name(), Audience: audience, Scopes: []apikey.Scope{apikey.ScopeExampleCall}, ExpiresAt: &expires, CreatedBy: fixture.actor,
	})
	if err != nil {
		t.Fatal(err)
	}
	return credential
}

func (fixture credentialFixture) request(method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set(httpapi.WebTokenHeader, credentialWebToken)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "heyblog_access_token", Value: fixture.access})
	response := httptest.NewRecorder()
	fixture.router.ServeHTTP(response, request)
	return response
}

func decodeCredential(t *testing.T, response *httptest.ResponseRecorder) apikey.Credential {
	t.Helper()
	var output struct {
		Credential apikey.Credential `json:"credential"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if output.Credential.Token == "" {
		t.Fatal("missing one-time credential")
	}
	return output.Credential
}
