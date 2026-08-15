//go:build integration

package dataimport

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"heyblog-api/internal/config"
	"heyblog-api/internal/database"
)

func TestRepositoryImportsDirectoryAtomicallyOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

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
		MaxConnectionLifetime: time.Minute, MaxConnectionIdleTime: time.Minute,
		HealthCheckPeriod: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("open runtime pool: %v", err)
	}
	t.Cleanup(pool.Close)

	repository := NewRepository(pool)
	bundles := repositoryTestBundles("rollback")
	plan, err := BuildPlan(bundles, sequenceShortIDGenerator("Rollback1", "Rollback2"))
	if err != nil {
		t.Fatalf("build rollback plan: %v", err)
	}
	plan.FriendLinks[0].TargetHost = "mismatched.invalid"
	if _, err := repository.Import(ctx, plan); err == nil {
		t.Fatal("Import() error = nil, want late friend-link failure")
	}
	assertDirectorySiteCount(ctx, t, pool, 0)

	bundles = repositoryTestBundles("locked")
	plan, err = BuildPlan(bundles, sequenceShortIDGenerator("Locked001", "Locked002"))
	if err != nil {
		t.Fatalf("build lock plan: %v", err)
	}
	lockTransaction, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin advisory lock transaction: %v", err)
	}
	if _, err := lockTransaction.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", importLockName); err != nil {
		t.Fatalf("acquire import advisory lock: %v", err)
	}
	if _, err := repository.Import(ctx, plan); !errors.Is(err, ErrImportRunning) {
		t.Fatalf("locked Import() error = %v, want ErrImportRunning", err)
	}
	if err := lockTransaction.Rollback(ctx); err != nil {
		t.Fatalf("release import advisory lock: %v", err)
	}
	assertDirectorySiteCount(ctx, t, pool, 0)

	bundles = repositoryTestBundles("success")
	service := NewService(repository, sequenceShortIDGenerator("Success01", "Success02"))
	counts, err := service.Import(ctx, bundles)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	wantCounts := Counts{Sites: 2, Feeds: 1, Resources: 2, Tags: 2, SiteTags: 2, SoftwareComponents: 2, Dependencies: 1, SiteComponents: 1, Sources: 4, Origins: 4, FriendLinks: 1}
	if counts != wantCounts {
		t.Fatalf("Import() counts = %#v, want %#v", counts, wantCounts)
	}
	assertDirectorySiteCount(ctx, t, pool, 2)
	var feedFormat string
	var isDefault bool
	if err := pool.QueryRow(ctx, "SELECT format, is_default FROM directory.site_feeds").Scan(&feedFormat, &isDefault); err != nil {
		t.Fatalf("query imported feed: %v", err)
	}
	if feedFormat != "ATOM" || !isDefault {
		t.Fatalf("feed = (%q, %t), want ATOM default", feedFormat, isDefault)
	}
	var sourceCount, originCount, dependencyCount int
	if err := pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM directory.site_sources),
		       (SELECT count(*) FROM directory.site_origins),
		       (SELECT count(*) FROM directory.software_component_dependencies)
	`).Scan(&sourceCount, &originCount, &dependencyCount); err != nil {
		t.Fatalf("query imported relations: %v", err)
	}
	if sourceCount != 4 || originCount != 4 || dependencyCount != 1 {
		t.Fatalf("relations = sources:%d origins:%d dependencies:%d, want 4/4/1", sourceCount, originCount, dependencyCount)
	}
	var hiddenVisibility string
	var hiddenReason *string
	if err := pool.QueryRow(ctx, "SELECT visibility, visibility_reason FROM directory.sites WHERE id = $1::uuid", bundles.Blogs.Blogs[1].ID).Scan(&hiddenVisibility, &hiddenReason); err != nil {
		t.Fatalf("query hidden site: %v", err)
	}
	if hiddenVisibility != "HIDDEN" || hiddenReason == nil || *hiddenReason != "FRIEND_LINK_DISCOVERY_PENDING_REVIEW" {
		t.Fatalf("hidden state = (%q, %v), want review reason", hiddenVisibility, hiddenReason)
	}
	var metadataOrigins int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM directory.site_origins WHERE jsonb_array_length(metadata->'input_kinds') > 0").Scan(&metadataOrigins); err != nil {
		t.Fatalf("query origin metadata: %v", err)
	}
	if metadataOrigins != 4 {
		t.Fatalf("origins with metadata = %d, want 4", metadataOrigins)
	}
	var storedGraphEdges int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM heyblog_directory."FRIEND_LINK"`).Scan(&storedGraphEdges); err != nil {
		t.Fatalf("query stored graph edges: %v", err)
	}
	if storedGraphEdges != 1 {
		t.Fatalf("stored graph edges = %d, want 1", storedGraphEdges)
	}
	var registeredLinks, externalLinks int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE target_site_id IS NOT NULL),
		       count(*) FILTER (WHERE target_site_id IS NULL)
		  FROM directory.list_friend_links($1::uuid, false)
	`, bundles.Blogs.Blogs[0].ID).Scan(&registeredLinks, &externalLinks); err != nil {
		t.Fatalf("query imported friend links: %v", err)
	}
	if registeredLinks != 0 || externalLinks != 0 {
		t.Fatalf("visible friend links = registered:%d external:%d, want hidden target to produce 0/0", registeredLinks, externalLinks)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE directory.sites
		   SET visibility = 'VISIBLE', visibility_reason = NULL
		 WHERE id = $1::uuid
	`, bundles.Blogs.Blogs[1].ID); err != nil {
		t.Fatalf("make imported graph target visible: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE target_site_id IS NOT NULL),
		       count(*) FILTER (WHERE target_site_id IS NULL)
		  FROM directory.list_friend_links($1::uuid, false)
	`, bundles.Blogs.Blogs[0].ID).Scan(&registeredLinks, &externalLinks); err != nil {
		t.Fatalf("query visible imported friend links: %v", err)
	}
	if registeredLinks != 1 || externalLinks != 0 {
		t.Fatalf("visible friend links = registered:%d external:%d, want 1/0 after target becomes visible", registeredLinks, externalLinks)
	}
	if _, err := service.Import(ctx, bundles); !errors.Is(err, ErrDirectoryNotEmpty) {
		t.Fatalf("second Import() error = %v, want ErrDirectoryNotEmpty", err)
	}
}

func repositoryTestBundles(_ string) Bundles {
	blogs, graph := testBundleJSON()
	bundles, err := DecodeBundles(blogs, graph)
	if err != nil {
		panic(err)
	}
	return bundles
}

func sequenceShortIDGenerator(values ...string) func() (string, error) {
	index := 0
	return func() (string, error) {
		value := values[index%len(values)]
		index++
		return value, nil
	}
}

func bootstrapImportTestRoles(ctx context.Context, t *testing.T, connection *pgx.Conn) {
	t.Helper()
	if _, err := connection.Exec(ctx, `
		CREATE EXTENSION IF NOT EXISTS age;
		CREATE ROLE migrator LOGIN PASSWORD 'migrator-secret' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
		CREATE ROLE api_runtime LOGIN PASSWORD 'runtime-secret' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
		ALTER ROLE migrator SET session_preload_libraries = 'age';
		ALTER ROLE api_runtime SET session_preload_libraries = 'age';
		REVOKE ALL ON DATABASE heyblog FROM PUBLIC;
		GRANT CONNECT, CREATE ON DATABASE heyblog TO migrator;
		GRANT CONNECT ON DATABASE heyblog TO api_runtime;
		GRANT USAGE ON SCHEMA ag_catalog TO migrator;
		CREATE SCHEMA migration AUTHORIZATION migrator;
	`); err != nil {
		t.Fatalf("bootstrap database roles: %v", err)
	}
}

func importTestRoleURL(t *testing.T, raw, username, password string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse PostgreSQL URL: %v", err)
	}
	parsed.User = url.UserPassword(username, password)
	return parsed.String()
}

func assertDirectorySiteCount(ctx context.Context, t *testing.T, pool *pgxpool.Pool, want int) {
	t.Helper()
	var got int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM directory.sites").Scan(&got); err != nil {
		t.Fatalf("count directory sites: %v", err)
	}
	if got != want {
		t.Fatalf("directory site count = %d, want %d", got, want)
	}
}
