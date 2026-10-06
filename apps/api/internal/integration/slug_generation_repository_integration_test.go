//go:build integration

package integration_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"heyblog-api/internal/features/sluggeneration"
	"heyblog-api/internal/infrastructure/database"
)

func TestSlugGenerationRepository(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	container, err := postgres.Run(ctx, "apache/age:release_PG18_1.7.0", postgres.WithDatabase("heyblog"), postgres.WithUsername("postgres"), postgres.WithPassword("postgres-secret"), postgres.BasicWaitStrategies())
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
	bootstrapDatabaseRoles(ctx, t, admin)
	if err := database.Migrate(ctx, databaseURLForRole(t, adminURL, "migrator", "migrator-secret")); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, databaseURLForRole(t, adminURL, "api_runtime", "runtime-secret"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repository := sluggeneration.NewRepository(pool)
	initial, err := repository.Settings(ctx)
	if err != nil || initial.Revision != "0" {
		t.Fatalf("initial=%+v err=%v", initial, err)
	}
	t.Run("concurrent first save has one winner", func(t *testing.T) {
		var succeeded atomic.Int64
		var group sync.WaitGroup
		for range 8 {
			group.Add(1)
			go func() {
				defer group.Done()
				_, err := repository.SaveSettings(ctx, sluggeneration.SaveSettingsInput{ModelID: "deepseek/deepseek-flash", ExpectedRevision: "0"})
				if err == nil {
					succeeded.Add(1)
				} else {
					assertSlugErrorCode(t, err, "settings_changed")
				}
			}()
		}
		group.Wait()
		if succeeded.Load() != 1 {
			t.Fatalf("concurrent winners=%d", succeeded.Load())
		}
	})
	changed, err := repository.SaveSettings(ctx, sluggeneration.SaveSettingsInput{ModelID: "deepseek-v4-flash", ExpectedRevision: "1"})
	if err != nil || changed.Revision != "2" {
		t.Fatalf("update=%+v err=%v", changed, err)
	}
	_, err = repository.SaveSettings(ctx, sluggeneration.SaveSettingsInput{ModelID: "deepseek/deepseek-flash", ExpectedRevision: "1"})
	assertSlugErrorCode(t, err, "settings_changed")
	key := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := repository.Cache(ctx, key, "example-slug"); err != nil {
		t.Fatal(err)
	}
	if err := repository.Cache(ctx, key, "must-not-replace"); err != nil {
		t.Fatal(err)
	}
	cached, err := repository.Cached(ctx, key)
	if err != nil || cached != "example-slug" {
		t.Fatalf("cached=%q err=%v", cached, err)
	}
	var id string
	if err := admin.QueryRow(ctx, `INSERT INTO directory.tag_dictionary (name, normalized_name, slug) VALUES ('Slug fixture','slug fixture','slug-fixture') RETURNING id::text`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO directory.tag_slug_aliases(slug,tag_id) VALUES ('old-slug-fixture',$1::uuid)`, id); err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"slug-fixture", "old-slug-fixture"} {
		occupied, err := repository.Occupied(ctx, slug, "")
		if err != nil || !occupied {
			t.Fatalf("occupied %s=%v err=%v", slug, occupied, err)
		}
		conflicts, err := repository.Conflicts(ctx, slug, "")
		if err != nil || len(conflicts) != 1 || conflicts[0].ID != id || conflicts[0].Slug != "slug-fixture" {
			t.Fatalf("conflicts=%+v err=%v", conflicts, err)
		}
		ownConflicts, err := repository.Conflicts(ctx, slug, id)
		if err != nil || len(ownConflicts) != 0 {
			t.Fatalf("own conflicts=%+v err=%v", ownConflicts, err)
		}
		occupied, err = repository.Occupied(ctx, slug, id)
		if err != nil || occupied {
			t.Fatalf("own %s=%v err=%v", slug, occupied, err)
		}
	}
}
