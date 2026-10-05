//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"heyblog-api/internal/features/sluggeneration"
	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/config"
)

func TestSlugGenerationRedisAdmission(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcredis.Run(ctx, "redis:8-alpine")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Error(err)
		}
	})
	connectionURL, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	options, err := redis.ParseURL(connectionURL)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(options)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	configuration := config.AIConfig{Timeout: time.Second, Limits: config.AILimits{UserPerMinute: 5, IPPerMinute: 20, UserPerDay: 3, GlobalPerDay: 5, Concurrent: 2}}
	first := sluggeneration.NewRedisGuard(client, configuration)
	second := sluggeneration.NewRedisGuard(client, configuration)
	t.Run("atomic shared minute counters", func(t *testing.T) {
		var admitted atomic.Int64
		var group sync.WaitGroup
		for index := range 20 {
			group.Add(1)
			go func() {
				defer group.Done()
				guard := first
				if index%2 == 0 {
					guard = second
				}
				if guard.Allow(ctx, sluggeneration.Identity{UserID: "same-user", IP: "192.0.2.1"}) == nil {
					admitted.Add(1)
				}
			}()
		}
		group.Wait()
		if admitted.Load() != 5 {
			t.Fatalf("admitted=%d want=5", admitted.Load())
		}
	})
	t.Run("IP cannot evade with multiple users", func(t *testing.T) {
		admitted := 0
		for index := range 25 {
			if first.Allow(ctx, sluggeneration.Identity{UserID: fmt.Sprintf("user-%d", index), IP: "192.0.2.2"}) == nil {
				admitted++
			}
		}
		if admitted != 20 {
			t.Fatalf("admitted=%d want=20", admitted)
		}
	})
	t.Run("duplicate and global capacity recover after release", func(t *testing.T) {
		lease, err := first.Acquire(ctx, sluggeneration.Attempt{UserID: "one", Key: "same"})
		if err != nil {
			t.Fatal(err)
		}
		_, err = second.Acquire(ctx, sluggeneration.Attempt{UserID: "two", Key: "same"})
		assertSlugErrorCode(t, err, "generation_in_progress")
		lease2, err := second.Acquire(ctx, sluggeneration.Attempt{UserID: "two", Key: "other"})
		if err != nil {
			t.Fatal(err)
		}
		_, err = first.Acquire(ctx, sluggeneration.Attempt{UserID: "three", Key: "third"})
		assertSlugErrorCode(t, err, "slug_concurrency_limited")
		if err := first.Release(ctx, lease); err != nil {
			t.Fatal(err)
		}
		lease3, err := first.Acquire(ctx, sluggeneration.Attempt{UserID: "three", Key: "third"})
		if err != nil {
			t.Fatal(err)
		}
		if err := second.Release(ctx, lease2); err != nil {
			t.Fatal(err)
		}
		if err := first.Release(ctx, lease3); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("lease expiry and stale owner cannot release successor", func(t *testing.T) {
		expiringConfig := configuration
		expiringConfig.Timeout = 20 * time.Millisecond
		expiring := sluggeneration.NewRedisGuard(client, expiringConfig)
		firstLease, err := expiring.Acquire(ctx, sluggeneration.Attempt{Key: "lease-expiry"})
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(5100 * time.Millisecond)
		nextLease, err := expiring.Acquire(ctx, sluggeneration.Attempt{Key: "lease-expiry"})
		if err != nil {
			t.Fatal(err)
		}
		if err := expiring.Release(ctx, firstLease); err != nil {
			t.Fatal(err)
		}
		_, err = expiring.Acquire(ctx, sluggeneration.Attempt{Key: "lease-expiry"})
		assertSlugErrorCode(t, err, "generation_in_progress")
		if err := expiring.Release(ctx, nextLease); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("paid attempts atomic and never refunded by release", func(t *testing.T) {
		lease, err := first.Acquire(ctx, sluggeneration.Attempt{UserID: "paid-user", Key: "paid-cancelled"})
		if err != nil {
			t.Fatal(err)
		}
		if err := first.Charge(ctx, "paid-user"); err != nil {
			t.Fatal(err)
		}
		if err := first.Release(ctx, lease); err != nil {
			t.Fatal(err)
		}
		var admitted atomic.Int64
		var group sync.WaitGroup
		for range 20 {
			group.Add(1)
			go func() {
				defer group.Done()
				if first.Charge(ctx, "paid-user") == nil {
					admitted.Add(1)
				}
			}()
		}
		group.Wait()
		if admitted.Load() != 2 {
			t.Fatalf("user paid attempts=%d", admitted.Load())
		}
		for range 2 {
			if err := second.Charge(ctx, "second-paid-user"); err != nil {
				t.Fatal(err)
			}
		}
		assertSlugErrorCode(t, first.Charge(ctx, "third-paid-user"), "slug_daily_limit")
	})
	t.Run("cancel and dependency failure are closed", func(t *testing.T) {
		stopped, stop := context.WithCancel(ctx)
		stop()
		assertSlugErrorCode(t, first.Allow(stopped, sluggeneration.Identity{}), "slug_generation_unavailable")
		if _, err := first.Acquire(stopped, sluggeneration.Attempt{Key: "cancelled"}); err == nil {
			t.Fatal("cancelled acquisition allowed")
		}
		disconnected := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 20 * time.Millisecond, MaxRetries: -1})
		defer disconnected.Close()
		broken := sluggeneration.NewRedisGuard(disconnected, configuration)
		assertSlugErrorCode(t, broken.Charge(ctx, "user"), "slug_generation_unavailable")
	})
}

func assertSlugErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	var failure *apperror.Error
	if !errors.As(err, &failure) || failure.Code() != code {
		t.Fatalf("error=%v want code=%s", err, code)
	}
}
