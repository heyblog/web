//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"

	"heyblog-api/internal/features/publicview"
	"heyblog-api/internal/features/sitestats"
	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5/pgxpool"
)

func verifySiteMetrics(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// Given
	id := insertSite(ctx, t, pool, "M1a2B3c4D", "Metric Site", "metric-site.example.com")
	queries := dbgen.New(pool)
	before, err := queries.GetSiteByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	stats := sitestats.New(pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	errs := make(chan error, 80)
	var workers sync.WaitGroup
	// When: concurrent duplicate and distinct deliveries compete on the same counter.
	for worker := 0; worker < 8; worker++ {
		workers.Go(func() {
			for event := 0; event < 10; event++ {
				errs <- stats.RecordEvents(ctx, []sitestats.Event{{EventID: fmt.Sprintf("12345678-1234-4234-9234-%012d", event), ShortID: before.ShortID}}, "CLICK")
			}
		})
	}
	workers.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	event := sitestats.Event{EventID: "12345678-1234-4234-9234-000000000001", ShortID: before.ShortID}
	for i := 0; i < 2; i++ {
		if err := stats.RecordEvents(ctx, []sitestats.Event{event, event}, "IMPRESSION"); err != nil {
			t.Fatal(err)
		}
	}
	views := publicview.New(queries, stats)
	profile, err := views.SiteByIdentifier(ctx, publicview.SiteIdentifier{Kind: publicview.IdentifierShortID, Value: before.ShortID})
	if err != nil {
		t.Fatal(err)
	}
	after, err := queries.GetSiteByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	// Then
	if profile.Metrics != (publicview.SiteMetrics{ClickCount: 10, ImpressionCount: 1, QueryCount: 1, ResponseCount: 1}) {
		t.Fatalf("metrics=%#v", profile.Metrics)
	}
	if before.Revision != after.Revision || before.UpdatedAt != after.UpdatedAt {
		t.Fatal("metrics changed site revision or timestamps")
	}

	t.Run("hidden site remains available", func(t *testing.T) {
		// Given
		if _, err := pool.Exec(ctx, "UPDATE directory.sites SET visibility='HIDDEN',visibility_reason='test' WHERE id=$1", id); err != nil {
			t.Fatal(err)
		}
		// When
		target, err := stats.Outbound(ctx, before.ShortID)
		// Then
		if err != nil || target != "https://metric-site.example.com/" {
			t.Fatalf("target=%s error=%v", target, err)
		}
	})
	t.Run("removed site cannot navigate or add events", func(t *testing.T) {
		// Given
		if _, err := pool.Exec(ctx, "UPDATE directory.sites SET visibility='REMOVED',visibility_reason='test' WHERE id=$1", id); err != nil {
			t.Fatal(err)
		}
		// When
		_, err := stats.Outbound(ctx, before.ShortID)
		// Then
		if err == nil {
			t.Fatal("removed site navigable")
		}
		event.EventID = "12345678-1234-4234-9234-000000000099"
		if err := stats.RecordEvents(ctx, []sitestats.Event{event}, "CLICK"); err != nil {
			t.Fatal(err)
		}
		rows, err := queries.GetSiteMetrics(ctx, []string{before.ShortID})
		if err != nil || len(rows) != 1 || rows[0].ClickCount != 10 {
			t.Fatalf("removed metrics=%#v error=%v", rows, err)
		}
	})
}
