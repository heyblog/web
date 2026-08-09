package database

import (
	"testing"
	"time"

	"heyblog-api/internal/config"
)

func TestPoolConfigAppliesRuntimePolicy(t *testing.T) {
	t.Parallel()

	input := config.DatabaseConfig{
		URL:                   "postgres://runtime@example.test/heyblog",
		MaxConnections:        15,
		MinConnections:        4,
		MaxConnectionLifetime: 45 * time.Minute,
		MaxConnectionIdleTime: 10 * time.Minute,
		HealthCheckPeriod:     30 * time.Second,
	}

	got, err := poolConfig(input)
	if err != nil {
		t.Fatalf("poolConfig() error = %v", err)
	}

	if got.MaxConns != input.MaxConnections || got.MinConns != input.MinConnections {
		t.Fatalf("pool bounds = (%d, %d), want (%d, %d)", got.MaxConns, got.MinConns, input.MaxConnections, input.MinConnections)
	}
	if got.MaxConnLifetime != input.MaxConnectionLifetime || got.MaxConnIdleTime != input.MaxConnectionIdleTime {
		t.Fatalf("connection lifetime policy not applied: %#v", got)
	}
	if got.HealthCheckPeriod != input.HealthCheckPeriod {
		t.Fatalf("HealthCheckPeriod = %s, want %s", got.HealthCheckPeriod, input.HealthCheckPeriod)
	}
	if _, configuredAtStartup := got.ConnConfig.RuntimeParams["search_path"]; configuredAtStartup {
		t.Fatal("search_path configured as a startup parameter, want connection hook configuration")
	}
	if got.AfterConnect == nil {
		t.Fatal("AfterConnect = nil, want runtime search path hook")
	}
}

func TestPoolConfigRejectsInvalidURL(t *testing.T) {
	t.Parallel()

	_, err := poolConfig(config.DatabaseConfig{URL: "not a postgres url"})
	if err == nil {
		t.Fatal("poolConfig() error = nil, want invalid URL error")
	}
}
