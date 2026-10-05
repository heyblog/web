//go:build integration

package dataimport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"heyblog-api/internal/domain/site"
	"heyblog-api/internal/infrastructure/database"
	"heyblog-api/internal/platform/config"
)

func incrementalTestPool(t *testing.T, ctx context.Context) (*pgxpool.Pool, *pgx.Conn) {
	t.Helper()
	container, err := postgres.Run(ctx, "apache/age:release_PG18_1.7.0", postgres.WithDatabase("heyblog"), postgres.WithUsername("postgres"), postgres.WithPassword("postgres-secret"), postgres.BasicWaitStrategies(), testcontainers.WithCmdArgs("-c", "max_locks_per_transaction=512"))
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
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	bootstrapImportTestRoles(ctx, t, admin)
	if err := database.Migrate(ctx, importTestRoleURL(t, adminURL, "migrator", "migrator-secret")); err != nil {
		t.Fatal(err)
	}
	pool, err := database.OpenPool(ctx, config.DatabaseConfig{URL: importTestRoleURL(t, adminURL, "api_runtime", "runtime-secret"), MaxConnections: 4, MinConnections: 0, MaxConnectionLifetime: time.Minute, MaxConnectionIdleTime: time.Minute, HealthCheckPeriod: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, admin
}

func TestIncrementalImportPreservesExistingRecordsAndInactiveEdges(t *testing.T) {
	// Given
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool, admin := incrementalTestPool(t, ctx)
	repository := NewRepository(pool)
	legacy := repositoryTestBundles("initial")
	if _, err := NewService(repository, sequenceGenerator("AAAAAAAAA", "BBBBBBBBB")).Import(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `SELECT directory.upsert_registered_friend_link($1::uuid, 'http://other.example/', 'other.example', 'INACTIVE')`, testSiteIDOne); err != nil {
		t.Fatal(err)
	}
	var beforeSites, beforeEdges string
	if err := admin.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(site) ORDER BY id)::text FROM directory.sites AS site`).Scan(&beforeSites); err != nil {
		t.Fatal(err)
	}
	if err := admin.QueryRow(ctx, `SELECT jsonb_agg(properties::text::jsonb ORDER BY id)::text FROM directory_graph."FRIEND_LINK"`).Scan(&beforeEdges); err != nil {
		t.Fatal(err)
	}
	bundles := incrementalTestBundles(t)
	bundles.Blogs.Blogs[0].ID = "01900000-0000-7000-8000-000000000101"
	bundles.Blogs.Blogs[1].ID = "01900000-0000-7000-8000-000000000102"
	bundles.Blogs.Blogs[1].URL = "https://other.example/different"
	// When
	counts, err := NewService(repository, sequenceGenerator("CCCCCCCCC", "DDDDDDDDD")).Import(ctx, bundles)
	// Then
	if err != nil {
		t.Fatal(err)
	}
	if counts.Sites != 0 || counts.Feeds != 0 || counts.FriendLinks != 0 || counts.MatchedSites != 2 || counts.ExistingFriendLinks != 1 || counts.Origins != 2 || counts.Sources != 1 {
		t.Fatalf("counts=%#v", counts)
	}
	var afterSites, afterEdges string
	if err := admin.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(site) ORDER BY id)::text FROM directory.sites AS site`).Scan(&afterSites); err != nil {
		t.Fatal(err)
	}
	if err := admin.QueryRow(ctx, `SELECT jsonb_agg(properties::text::jsonb ORDER BY id)::text FROM directory_graph."FRIEND_LINK"`).Scan(&afterEdges); err != nil {
		t.Fatal(err)
	}
	if beforeSites != afterSites || beforeEdges != afterEdges {
		t.Fatal("existing sites or inactive edges changed")
	}
}

func TestIncrementalImportIsRepeatableAndRollsBackLateFailure(t *testing.T) {
	// Given
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool, _ := incrementalTestPool(t, ctx)
	repository := NewRepository(pool)
	bundles := incrementalTestBundles(t)
	plan, err := BuildPlan(bundles, sequenceGenerator("AAAAAAAAA", "BBBBBBBBB"))
	if err != nil {
		t.Fatal(err)
	}
	plan.Origins[0].Metadata = json.RawMessage(`[]`)
	if _, err := repository.ImportIncremental(ctx, plan, site.NewShortID); err == nil {
		t.Fatal("invalid origin did not fail")
	}
	assertDirectorySiteCount(ctx, t, pool, 0)
	service := NewService(repository, site.NewShortID)
	if _, err := service.Import(ctx, bundles); err != nil {
		t.Fatal(err)
	}
	// When
	counts, err := service.Import(ctx, bundles)
	// Then
	if err != nil {
		t.Fatal(err)
	}
	if counts.Sites != 0 || counts.Feeds != 0 || counts.Sources != 0 || counts.Origins != 0 || counts.FriendLinks != 0 || counts.MatchedSites != 2 || counts.ExistingFriendLinks != 1 {
		t.Fatalf("repeat counts=%#v", counts)
	}
	assertDirectorySiteCount(ctx, t, pool, 2)
}

func TestIncrementalFullVolumeJSONLThroughHTTP(t *testing.T) {
	// Given
	directory := os.Getenv("HEYBLOG_IMPORT_FIXTURE_DIR")
	if directory == "" {
		t.Skip("HEYBLOG_IMPORT_FIXTURE_DIR is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), ImportTimeout)
	defer cancel()
	pool, admin := incrementalTestPool(t, ctx)
	blogs, err := os.ReadFile(filepath.Join(directory, "blogs.cleaned.json"))
	if err != nil {
		t.Fatal(err)
	}
	graph, err := os.ReadFile(filepath.Join(directory, "graph.cleaned.json"))
	if err != nil {
		t.Fatal(err)
	}
	bundles, err := DecodeBundles(blogs, graph)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(bundles, site.NewShortID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("validated full-volume plan: %#v", plan.Counts())
	// When
	request := multipartModeRequest(t, "incremental", blogs, graph).WithContext(ctx)
	recorder := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	newImportTestRouter(t, NewService(NewRepository(pool), site.NewShortID)).ServeHTTP(recorder, request)
	// Then
	if recorder.Code != http.StatusOK {
		t.Fatalf("full-volume HTTP status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response importResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	counts := response.Counts
	var edges int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM directory_graph."FRIEND_LINK"`).Scan(&edges); err != nil {
		t.Fatal(err)
	}
	if counts.Sites != bundles.Blogs.Count || counts.FriendLinks != bundles.Graph.EdgeCount || edges != counts.FriendLinks {
		t.Fatalf("counts=%#v storedEdges=%d", counts, edges)
	}
	t.Logf("full-volume imported: %#v", counts)
}
