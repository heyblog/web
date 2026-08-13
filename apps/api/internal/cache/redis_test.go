package cache

import (
	"testing"
	"time"

	"heyblog-api/internal/config"
)

func TestRedisOptionsAppliesRuntimePolicy(t *testing.T) {
	t.Parallel()

	input := config.RedisConfig{
		URL:          "rediss://user:secret@example.test:6380/2", // #nosec G101 -- this is a non-functional test fixture.
		DialTimeout:  4 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 2 * time.Second,
	}

	got, err := redisOptions(input)
	if err != nil {
		t.Fatalf("redisOptions() error = %v", err)
	}

	if got.Addr != "example.test:6380" || got.DB != 2 {
		t.Fatalf("Redis endpoint = (%q, %d), want (%q, %d)", got.Addr, got.DB, "example.test:6380", 2)
	}
	if got.DialTimeout != input.DialTimeout || got.ReadTimeout != input.ReadTimeout || got.WriteTimeout != input.WriteTimeout {
		t.Fatalf("Redis timeouts not applied: %#v", got)
	}
}

func TestRedisOptionsRejectsInvalidURL(t *testing.T) {
	t.Parallel()

	_, err := redisOptions(config.RedisConfig{URL: "not a redis url"})
	if err == nil {
		t.Fatal("redisOptions() error = nil, want invalid URL error")
	}
}
