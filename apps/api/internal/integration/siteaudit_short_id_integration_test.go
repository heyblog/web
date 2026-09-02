//go:build integration

package integration_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"heyblog-api/internal/siteaudit"
)

func verifySiteAuditShortIDMaintenance(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	const shortID = "8Qa9Rb0Sc"
	const customID = "maintenance-alias"
	siteID := insertSite(ctx, t, pool, shortID, "Maintenance Target", "maintenance.example.com")
	if _, err := pool.Exec(ctx, `UPDATE directory.sites SET custom_id = $2 WHERE id = $1`, siteID, customID); err != nil {
		t.Fatalf("set maintenance site custom ID: %v", err)
	}

	service := siteaudit.NewService(siteaudit.Dependencies{Repository: siteaudit.NewRepository(pool)})
	snapshot, err := service.ResolveSite(ctx, shortID)
	if err != nil {
		t.Fatalf("resolve site by short ID: %v", err)
	}
	if snapshot.ShortID != shortID || snapshot.SiteID != "" {
		t.Fatalf("public snapshot identifiers = (short:%q uuid:%q), want (%q, empty)", snapshot.ShortID, snapshot.SiteID, shortID)
	}

	results, err := service.SearchSites(ctx, shortID)
	if err != nil {
		t.Fatalf("search site by exact short ID: %v", err)
	}
	if len(results) != 1 || results[0].ShortID != shortID {
		t.Fatalf("short ID search results = %#v, want one result for %q", results, shortID)
	}

	siteIDValue, err := siteID.Value()
	if err != nil {
		t.Fatalf("format maintenance site UUID: %v", err)
	}
	siteUUID, ok := siteIDValue.(string)
	if !ok {
		t.Fatalf("maintenance site UUID value type = %T, want string", siteIDValue)
	}
	for label, identifier := range map[string]string{"UUID": siteUUID, "custom ID": customID} {
		_, submitErr := service.Submit(ctx, siteaudit.ActionDelete, identifier, siteaudit.SubmissionInput{Reason: "Routine removal request."})
		if !errors.Is(submitErr, siteaudit.ErrInvalidSubmission) {
			t.Errorf("submit with %s error = %v, want invalid_submission", label, submitErr)
		}
	}

	result, err := service.Submit(ctx, siteaudit.ActionDelete, shortID, siteaudit.SubmissionInput{Reason: "Routine removal request."})
	if err != nil {
		t.Fatalf("submit deletion by short ID: %v", err)
	}
	if result.ShortID != shortID {
		t.Fatalf("submission short ID = %q, want %q", result.ShortID, shortID)
	}
	var storedSiteID pgtype.UUID
	if err := pool.QueryRow(ctx, `SELECT site_id FROM directory.site_audits WHERE id = $1`, result.AuditID).Scan(&storedSiteID); err != nil {
		t.Fatalf("read submitted audit site ID: %v", err)
	}
	if storedSiteID != siteID {
		t.Fatalf("stored audit site ID = %v, want %v", storedSiteID, siteID)
	}
	publicAudit, err := service.Query(ctx, result.LookupToken)
	if err != nil {
		t.Fatalf("query submitted audit: %v", err)
	}
	if publicAudit.ShortID != shortID {
		t.Fatalf("public audit short ID = %q, want %q", publicAudit.ShortID, shortID)
	}
}
