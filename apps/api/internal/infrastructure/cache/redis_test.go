package cache

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"heyblog-api/internal/platform/config"
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

func TestRedisOperationsRespectShorterContextDeadline(t *testing.T) {
	// Given: a Redis connection accepts commands but never replies.
	options, err := redisOptions(config.RedisConfig{
		URL: "redis://example.test:6379", DialTimeout: time.Second,
		ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	options.Dialer = func(context.Context, string, string) (net.Conn, error) {
		client, server := net.Pipe()
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = io.Copy(io.Discard, server)
		}()
		t.Cleanup(func() {
			_ = server.Close()
			<-done
		})
		return client, nil
	}
	client := redis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	// When
	started := time.Now()
	err = client.Ping(ctx).Err()
	// Then: the request budget takes precedence over configured I/O timeouts and retries.
	if err == nil {
		t.Fatal("unresponsive Redis operation succeeded")
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("Redis ignored context deadline: elapsed %s, error %v", elapsed, err)
	}
}

func TestRedisOptionsRejectsInvalidURL(t *testing.T) {
	t.Parallel()

	_, err := redisOptions(config.RedisConfig{URL: "not a redis url"})
	if err == nil {
		t.Fatal("redisOptions() error = nil, want invalid URL error")
	}
}
