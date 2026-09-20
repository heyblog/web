//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"heyblog-api/internal/application/publicview"
	"heyblog-api/internal/config"
	"heyblog-api/internal/database"
	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/httpapi"
)

func TestSiteIconHTTPWithDatabase(t *testing.T) {
	// Given an isolated migrated database and the actual HTTP/application stack.
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
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
	queries := dbgen.New(pool)
	service := publicview.New(queries)
	token := "site-icon-integration-web-token-123456"
	router, err := httpapi.NewRouter(httpapi.Options{Mode: config.ModeDevelopment, HTTP: config.HTTPConfig{MaxBodyBytes: 1 << 20, TrustedProxies: []string{}}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), WebToken: token, HealthcheckToken: "site-icon-health-token-123456", PublicViews: service})
	if err != nil {
		t.Fatal(err)
	}
	var source bytes.Buffer
	if err := png.Encode(&source, image.NewNRGBA(image.Rect(0, 0, 256, 128))); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(source.Bytes())
	for _, state := range []struct {
		name, id, visibility string
		icon                 bool
		status               int
	}{
		{name: "visible", id: "A1b2C3d4E", visibility: "VISIBLE", icon: true, status: 200},
		{name: "hidden", id: "B1b2C3d4E", visibility: "HIDDEN", icon: true, status: 200},
		{name: "removed", id: "C1b2C3d4E", visibility: "REMOVED", icon: true, status: 404},
		{name: "no icon", id: "D1b2C3d4E", visibility: "VISIBLE", status: 404},
	} {
		t.Run(state.name, func(t *testing.T) {
			id := insertSite(ctx, t, pool, state.id, state.name, strings.ToLower(state.id)+".example.test")
			if state.visibility != "VISIBLE" {
				if _, err := pool.Exec(ctx, "UPDATE directory.sites SET visibility=$2,visibility_reason='test' WHERE id=$1", id, state.visibility); err != nil {
					t.Fatal(err)
				}
			}
			if state.icon {
				if _, err := queries.UpsertSiteIcon(ctx, dbgen.UpsertSiteIconParams{SiteID: id, Content: source.Bytes(), MediaType: "image/png", Sha256: digest[:]}); err != nil {
					t.Fatal(err)
				}
			}
			// When the actual HTTP route reads each supported identifier form.
			for _, identifier := range []string{state.id, id.String()} {
				request := httptest.NewRequest(http.MethodGet, "/sites/id/"+identifier+"/icon", nil)
				request.Header.Set(httpapi.WebTokenHeader, token)
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				// Then availability, PNG dimensions, and version hash agree with persisted state.
				if response.Code != state.status {
					t.Fatalf("status=%d body=%s", response.Code, response.Body)
				}
				if state.status == 200 {
					configuration, err := png.DecodeConfig(response.Body)
					if err != nil || configuration.Width != 128 || configuration.Height != 64 {
						t.Fatalf("PNG=%+v error=%v", configuration, err)
					}
				}
			}
			if state.visibility != "REMOVED" {
				profile, err := service.SiteByIdentifier(ctx, publicview.SiteIdentifier{Kind: publicview.IdentifierShortID, Value: state.id})
				if err != nil {
					t.Fatal(err)
				}
				if state.icon && (profile.IconHash == nil || *profile.IconHash != hex.EncodeToString(digest[:])) {
					t.Fatalf("hash=%v", profile.IconHash)
				}
				if !state.icon && profile.IconHash != nil {
					t.Fatalf("missing icon hash=%v", profile.IconHash)
				}
			}
		})
	}
}
