package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"heyblog-api/internal/config"
)

const runtimeSearchPath = "ag_catalog,public"

func poolConfig(input config.DatabaseConfig) (*pgxpool.Config, error) {
	poolConfig, err := pgxpool.ParseConfig(input.URL)
	if err != nil {
		return nil, fmt.Errorf("parse runtime database URL: %w", err)
	}

	poolConfig.MaxConns = input.MaxConnections
	poolConfig.MinConns = input.MinConnections
	poolConfig.MaxConnLifetime = input.MaxConnectionLifetime
	poolConfig.MaxConnIdleTime = input.MaxConnectionIdleTime
	poolConfig.HealthCheckPeriod = input.HealthCheckPeriod
	poolConfig.AfterConnect = func(ctx context.Context, connection *pgx.Conn) error {
		if _, err := connection.Exec(ctx, "SET search_path = "+runtimeSearchPath); err != nil {
			return fmt.Errorf("set runtime search path: %w", err)
		}
		return nil
	}

	return poolConfig, nil
}

func OpenPool(ctx context.Context, input config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolConfig, err := poolConfig(input)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("open runtime database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping runtime database: %w", err)
	}

	return pool, nil
}
