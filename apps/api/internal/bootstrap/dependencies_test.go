package bootstrap

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"heyblog-api/internal/config"
)

func TestOpenRollsBackDatabaseWhenRedisFails(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("redis unavailable")
	var events []string
	_, err := open(context.Background(), config.Config{}, dependencyOperations{
		migrate: func(context.Context, string) error {
			events = append(events, "migrate")
			return nil
		},
		openDatabase: func(context.Context, config.DatabaseConfig) (*pgxpool.Pool, error) {
			events = append(events, "open-database")
			return nil, nil
		},
		closeDatabase: func(*pgxpool.Pool) {
			events = append(events, "close-database")
		},
		openRedis: func(context.Context, config.RedisConfig) (*redis.Client, error) {
			events = append(events, "open-redis")
			return nil, wantErr
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("open() error = %v, want Redis failure", err)
	}
	wantEvents := []string{"migrate", "open-database", "open-redis", "close-database"}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %v, want %v", events, wantEvents)
	}
}

func TestDependenciesCloseIsIdempotentAndReverseOrder(t *testing.T) {
	t.Parallel()

	var events []string
	dependencies := &Dependencies{
		closeRedis: func() error {
			events = append(events, "redis")
			return errors.New("redis close failed")
		},
		closeDatabase: func() {
			events = append(events, "database")
		},
	}

	firstErr := dependencies.Close()
	secondErr := dependencies.Close()
	if firstErr == nil || secondErr == nil || firstErr.Error() != secondErr.Error() {
		t.Fatalf("Close() errors = (%v, %v), want stable close error", firstErr, secondErr)
	}
	if want := []string{"redis", "database"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("close events = %v, want %v", events, want)
	}
}

func TestDependenciesReadinessRunsChecksConcurrently(t *testing.T) {
	t.Parallel()

	started := make(chan string, 2)
	release := make(chan struct{})
	dependencies := &Dependencies{
		pingDatabase: func(context.Context) error {
			started <- "database"
			<-release
			return nil
		},
		pingRedis: func(context.Context) error {
			started <- "redis"
			<-release
			return nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- dependencies.Ready(context.Background()) }()

	seen := map[string]bool{<-started: true, <-started: true}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("Ready() error = %v", err)
	}
	if !seen["database"] || !seen["redis"] {
		t.Fatalf("started checks = %v, want database and Redis", seen)
	}
}

func TestReadinessErrorNamesFailedComponent(t *testing.T) {
	t.Parallel()

	dependencies := &Dependencies{
		pingDatabase: func(context.Context) error { return errors.New("down") },
		pingRedis:    func(context.Context) error { return nil },
	}
	err := dependencies.Ready(context.Background())
	var component interface{ Component() string }
	if !errors.As(err, &component) || component.Component() != "database" {
		t.Fatalf("Ready() error = %v, want database component metadata", err)
	}
}
