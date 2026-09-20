//go:build integration && migrationdata

package dataimport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"heyblog-api/internal/config"
	"heyblog-api/internal/database"
)

func TestRealCleanedBundlesImportIntoEmptyDatabase(t *testing.T) {
	directory := os.Getenv("HEYBLOG_IMPORT_FIXTURE_DIR")
	if directory == "" {
		t.Skip("HEYBLOG_IMPORT_FIXTURE_DIR is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	defer cancel()
	blogs, err := os.ReadFile(filepath.Join(directory, "blogs.cleaned.json"))
	if err != nil {
		t.Fatalf("read cleaned blogs: %v", err)
	}
	graph, err := os.ReadFile(filepath.Join(directory, "graph.cleaned.json"))
	if err != nil {
		t.Fatalf("read cleaned graph: %v", err)
	}
	bundles, err := DecodeBundles(blogs, graph)
	if err != nil {
		t.Fatalf("DecodeBundles() error = %v", err)
	}

	container, err := postgres.Run(
		ctx,
		"apache/age:release_PG18_1.7.0",
		postgres.WithDatabase("heyblog"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres-secret"),
		postgres.BasicWaitStrategies(),
		testcontainers.WithCmdArgs("-c", "max_locks_per_transaction=512"),
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
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect as PostgreSQL administrator: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	bootstrapImportTestRoles(ctx, t, admin)
	migrationURL := importTestRoleURL(t, adminURL, "migrator", "migrator-secret")
	if err := database.Migrate(ctx, migrationURL); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	runtimeURL := importTestRoleURL(t, adminURL, "api_runtime", "runtime-secret")
	pool, err := database.OpenPool(ctx, config.DatabaseConfig{
		URL: runtimeURL, MaxConnections: 4, MinConnections: 0,
		MaxConnectionLifetime: 10 * time.Minute, MaxConnectionIdleTime: time.Minute,
		HealthCheckPeriod: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("open runtime pool: %v", err)
	}
	t.Cleanup(pool.Close)

	shortID := 0
	service := NewService(NewRepository(pool), func() (string, error) {
		shortID++
		return fmt.Sprintf("%09d", shortID), nil
	})
	started := time.Now()
	router := newImportTestRouter(t, service)
	request := multipartImportRequest(t, blogs, graph)
	request.Header.Set("Authorization", "Bearer "+testImportToken)
	responseRecorder := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	router.ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("HTTP import response = (%d, %q), want 200", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response importResponse
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode HTTP import response: %v", err)
	}
	counts := response.Counts
	if counts.Sites != bundles.Blogs.Count || counts.FriendLinks != bundles.Graph.EdgeCount ||
		counts.Sources != 3 || counts.Origins < counts.Sites {
		t.Fatalf("Import() counts = %#v, want sites/edges from bundles, three sources, and at least one origin per site", counts)
	}
	t.Logf("real data import duration: %s", time.Since(started))

	var sites, feeds, tags, origins, dependencies int
	if err := pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM directory.sites),
		       (SELECT count(*) FROM directory.site_feeds),
		       (SELECT count(*) FROM directory.tags),
		       (SELECT count(*) FROM directory.site_origins),
		       (SELECT count(*) FROM directory.software_component_dependencies)
	`).Scan(&sites, &feeds, &tags, &origins, &dependencies); err != nil {
		t.Fatalf("query imported row counts: %v", err)
	}
	if sites != counts.Sites || feeds != counts.Feeds || tags != counts.Tags || origins != counts.Origins || dependencies != counts.Dependencies {
		t.Fatalf("stored counts = sites:%d feeds:%d tags:%d origins:%d dependencies:%d", sites, feeds, tags, origins, dependencies)
	}
}
