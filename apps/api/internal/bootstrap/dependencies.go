package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"heyblog-api/internal/cache"
	"heyblog-api/internal/config"
	"heyblog-api/internal/database"
)

type Dependencies struct {
	Database *pgxpool.Pool
	Redis    *redis.Client

	pingDatabase  func(context.Context) error
	pingRedis     func(context.Context) error
	closeDatabase func()
	closeRedis    func() error
	closeOnce     sync.Once
	closeErr      error
}

type dependencyOperations struct {
	migrate       func(context.Context, string) error
	openDatabase  func(context.Context, config.DatabaseConfig) (*pgxpool.Pool, error)
	closeDatabase func(*pgxpool.Pool)
	openRedis     func(context.Context, config.RedisConfig) (*redis.Client, error)
}

type ReadinessError struct {
	component string
	cause     error
}

func Open(ctx context.Context, configuration config.Config) (*Dependencies, error) {
	return open(ctx, configuration, dependencyOperations{
		migrate:       database.Migrate,
		openDatabase:  database.OpenPool,
		closeDatabase: func(pool *pgxpool.Pool) { pool.Close() },
		openRedis:     cache.OpenRedis,
	})
}

func open(ctx context.Context, configuration config.Config, operations dependencyOperations) (*Dependencies, error) {
	if err := operations.migrate(ctx, configuration.MigrationDatabaseURL); err != nil {
		return nil, withStage("database_migration", err)
	}

	pool, err := operations.openDatabase(ctx, configuration.Database)
	if err != nil {
		return nil, withStage("database_open", err)
	}

	redisClient, err := operations.openRedis(ctx, configuration.Redis)
	if err != nil {
		operations.closeDatabase(pool)
		return nil, withStage("redis_open", err)
	}

	return &Dependencies{
		Database: pool,
		Redis:    redisClient,
		pingDatabase: func(ctx context.Context) error {
			return pool.Ping(ctx)
		},
		pingRedis: func(ctx context.Context) error {
			return redisClient.Ping(ctx).Err()
		},
		closeDatabase: func() {
			operations.closeDatabase(pool)
		},
		closeRedis: redisClient.Close,
	}, nil
}

func (dependencies *Dependencies) Ready(ctx context.Context) error {
	checks := []struct {
		component string
		check     func(context.Context) error
	}{
		{component: "database", check: dependencies.pingDatabase},
		{component: "redis", check: dependencies.pingRedis},
	}
	errorsChannel := make(chan error, len(checks))
	for _, check := range checks {
		go func() {
			if check.check == nil {
				errorsChannel <- &ReadinessError{component: check.component, cause: errors.New("readiness check is not configured")}
				return
			}
			if err := check.check(ctx); err != nil {
				errorsChannel <- &ReadinessError{component: check.component, cause: err}
				return
			}
			errorsChannel <- nil
		}()
	}

	var readinessErrors []error
	for range checks {
		if err := <-errorsChannel; err != nil {
			readinessErrors = append(readinessErrors, err)
		}
	}
	return errors.Join(readinessErrors...)
}

func (dependencies *Dependencies) Close() error {
	dependencies.closeOnce.Do(func() {
		var closeErrors []error
		if dependencies.closeRedis != nil {
			if err := dependencies.closeRedis(); err != nil {
				closeErrors = append(closeErrors, fmt.Errorf("close Redis: %w", err))
			}
		}
		if dependencies.closeDatabase != nil {
			dependencies.closeDatabase()
		}
		dependencies.closeErr = errors.Join(closeErrors...)
	})
	return dependencies.closeErr
}

func (err *ReadinessError) Error() string {
	return err.component + " readiness: " + err.cause.Error()
}

func (err *ReadinessError) Unwrap() error {
	return err.cause
}

func (err *ReadinessError) Component() string {
	return err.component
}
