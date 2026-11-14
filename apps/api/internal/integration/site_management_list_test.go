//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"heyblog-api/internal/features/siteaudit"
)

func verifyManagedSiteList(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	visibleID := insertSite(ctx, t, pool, "5MgmtA1Bc", "Managed List Visible", "managed-list-visible.example.test")
	if _, err := pool.Exec(ctx, `UPDATE directory.sites
		SET custom_id = 'managed-visible', scheme = 'http', base_path = '/journal', access_scope = 'GLOBAL_ONLY'
		WHERE id = $1`, visibleID); err != nil {
		t.Fatalf("configure managed visible site: %v", err)
	}
	hiddenID := insertSite(ctx, t, pool, "5MgmtD2Ef", "Managed List Hidden", "managed-list-hidden.example.test")
	if _, err := pool.Exec(ctx, `UPDATE directory.sites
		SET visibility = 'HIDDEN', visibility_reason = 'Temporarily unavailable'
		WHERE id = $1`, hiddenID); err != nil {
		t.Fatalf("configure managed hidden site: %v", err)
	}
	removedID := insertSite(ctx, t, pool, "5MgmtG3Hi", "Managed List Removed", "managed-list-removed.example.test")
	if _, err := pool.Exec(ctx, `UPDATE directory.sites
		SET visibility = 'REMOVED', visibility_reason = 'Owner requested removal'
		WHERE id = $1`, removedID); err != nil {
		t.Fatalf("configure managed removed site: %v", err)
	}

	service := siteaudit.NewService(siteaudit.Dependencies{Repository: siteaudit.NewRepository(pool)})
	pending, err := service.Submit(ctx, siteaudit.ActionDelete, "5MgmtA1Bc", siteaudit.SubmissionInput{
		Reason: "Routine lifecycle request",
	})
	if err != nil {
		t.Fatalf("create pending managed-site audit: %v", err)
	}

	page, err := service.ListManagedSites(ctx, "MANAGED-LIST", "", 1, 2)
	if err != nil {
		t.Fatalf("list managed sites: %v", err)
	}
	if page.Page != 1 || page.PageSize != 2 || page.TotalItems != 3 || page.TotalPages != 2 || len(page.Items) != 2 {
		t.Fatalf("managed site page = %#v, want first two of three results", page)
	}
	last, err := service.ListManagedSites(ctx, "managed-list", "", 999, 2)
	if err != nil {
		t.Fatalf("list final managed-site page: %v", err)
	}
	if last.Page != 2 || len(last.Items) != 1 {
		t.Fatalf("final managed site page = %#v, want clamped page 2 with one result", last)
	}

	visible, err := service.ListManagedSites(ctx, "managed-visible", "VISIBLE", 1, 20)
	if err != nil {
		t.Fatalf("find managed site by custom ID: %v", err)
	}
	if len(visible.Items) != 1 || visible.Items[0].URL != "http://managed-list-visible.example.test/journal" ||
		visible.Items[0].AccessScope != "GLOBAL_ONLY" || visible.Items[0].PendingAudit == nil ||
		visible.Items[0].PendingAudit.ID != pending.AuditID || visible.Items[0].PendingAudit.Action != siteaudit.ActionDelete {
		t.Fatalf("managed visible site = %#v, want address, scope, and pending deletion", visible.Items)
	}
	hidden, err := service.ListManagedSites(ctx, "5MgmtD2Ef", "HIDDEN", 1, 20)
	if err != nil {
		t.Fatalf("find hidden managed site by short ID: %v", err)
	}
	if len(hidden.Items) != 1 || hidden.Items[0].VisibilityReason != "Temporarily unavailable" {
		t.Fatalf("managed hidden site = %#v, want visibility reason", hidden.Items)
	}
	removed, err := service.ListManagedSites(ctx, "managed-list-removed.example.test", "REMOVED", 1, 20)
	if err != nil {
		t.Fatalf("find removed managed site by domain: %v", err)
	}
	if len(removed.Items) != 1 || removed.Items[0].ShortID != "5MgmtG3Hi" {
		t.Fatalf("managed removed site = %#v, want removed fixture", removed.Items)
	}
}
