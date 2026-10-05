//go:build integration

package integration_test

import (
	"context"
	"heyblog-api/internal/infrastructure/cache"
	"heyblog-api/internal/infrastructure/database"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/platform/config"
	"heyblog-api/internal/platform/ratelimit"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
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
	verifyRandomClassificationSelection(ctx, t, pool)
	verifyTagAndIconConstraints(ctx, t, pool)
	verifyAnnouncementQueries(ctx, t, pool)
	verifyAnnouncementConstraints(ctx, t, pool, migrationURL)
	verifySoftwareComponentDependencies(ctx, t, pool)
	verifySiteAuditReviewDraftQueries(ctx, t, pool)
	verifySiteAuditShortIDMaintenance(ctx, t, pool)
	verifySiteAuditAddressConflicts(ctx, t, pool)
	verifySiteAuditLifecycle(ctx, t, pool)
	verifyFriendLinkGraph(ctx, t, pool, adminConnection)
	verifyPublicFriendGraph(ctx, t, pool)
	verifyRuntimePermissions(ctx, t, pool)
	verifyAnnouncementActorDeletionSemantics(ctx, t, pool, migrationURL)
	verifyUserDeletionSemantics(ctx, t, pool, migrationURL)
	verifyAuthenticationFlows(ctx, t, pool)
	verifyMigrationRollback(ctx, t, adminConnection, migrationURL)
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
