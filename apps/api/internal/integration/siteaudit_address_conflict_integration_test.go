//go:build integration

package integration_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"heyblog-api/internal/auth"
	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/siteaudit"
)

func verifySiteAuditAddressConflicts(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	// Given a canonical site, valid submission taxonomy, and an authorized reviewer.
	const existingShortID = "4Fjw77W5U"
	insertSite(ctx, t, pool, existingShortID, "Existing Site", "existing-review.example.com")
	queries := dbgen.New(pool)
	cascades, err := queries.ListEnabledSiteTagCascades(ctx)
	if err != nil || len(cascades) == 0 {
		t.Fatalf("list review conflict tag cascades: %v / %d", err, len(cascades))
	}
	taxonomy := siteauditConflictTaxonomy{
		Level1ID: integrationUUIDText(t, cascades[0].Level1ID),
		Level2ID: integrationUUIDText(t, cascades[0].Level2ID),
	}
	program, err := queries.GetSoftwareComponentByNormalizedName(ctx, "其他")
	if err != nil {
		t.Fatalf("get review conflict program: %v", err)
	}
	reviewer, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email: "site-audit-conflict@example.test", Username: "site_audit_conflict", DisplayName: "Site Audit Conflict",
	})
	if err != nil {
		t.Fatalf("create review conflict reviewer: %v", err)
	}
	service := siteaudit.NewService(siteaudit.Dependencies{
		Repository: siteaudit.NewRepository(pool),
		NewShortID: func() (string, error) { return "7Pq8Rs9Tu", nil },
	})
	programID := integrationUUIDText(t, program.ID)
	reviewActor := auth.User{ID: integrationUUIDText(t, reviewer.ID), Role: auth.RoleSysAdmin}

	// When availability is checked with another URL form for the existing host.
	availability, err := service.CheckSiteAvailability(ctx, "http://existing-review.example.com/blog")
	if err != nil {
		t.Fatalf("check existing site availability: %v", err)
	}

	// Then the existing public site identity is returned without treating the path as distinct.
	if availability.Available || availability.ExistingSite == nil || availability.ExistingSite.ShortID != existingShortID {
		t.Fatalf("availability = %#v, want existing site %q", availability, existingShortID)
	}

	// When a CREATE submission directly targets the existing host.
	_, err = service.Submit(ctx, siteaudit.ActionCreate, "", siteauditConflictSubmission("https://existing-review.example.com", taxonomy, programID))

	// Then it is rejected as a stable conflict before an audit is created.
	assertSiteAuditServiceError(t, err, "site_address_conflict", http.StatusConflict)

	// Given a CREATE submission whose host is claimed after submission but before approval.
	createResult, err := service.Submit(ctx, siteaudit.ActionCreate, "", siteauditConflictSubmission("https://review-race.example.com", taxonomy, programID))
	if err != nil {
		t.Fatalf("submit review-race creation: %v", err)
	}
	insertSite(ctx, t, pool, "6Nop7Qr8S", "Review Race Winner", "review-race.example.com")

	// When the pending CREATE is approved.
	_, err = service.Review(ctx, reviewActor, siteaudit.ReviewInput{AuditID: createResult.AuditID, Decision: siteaudit.DecisionApprove})

	// Then the unique-host race is a conflict and the audit remains pending.
	assertSiteAuditServiceError(t, err, "site_address_conflict", http.StatusConflict)
	assertSiteAuditStatus(t, ctx, pool, createResult.AuditID, siteaudit.StatusPending)

	// Given an UPDATE that changes one site's host to another canonical site's host.
	const updateShortID = "5Klm6No7P"
	updateSiteID := insertSite(ctx, t, pool, updateShortID, "Update Source", "update-source.example.com")
	updateResult, err := service.Submit(ctx, siteaudit.ActionUpdate, updateShortID, siteauditConflictSubmission("https://existing-review.example.com", taxonomy, programID))
	if err != nil {
		t.Fatalf("submit conflicting site update: %v", err)
	}

	// When the pending UPDATE is approved.
	_, err = service.Review(ctx, reviewActor, siteaudit.ReviewInput{
		AuditID: updateResult.AuditID, Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: 1,
	})

	// Then it uses the same conflict contract and preserves both the audit and source site.
	assertSiteAuditServiceError(t, err, "site_address_conflict", http.StatusConflict)
	assertSiteAuditStatus(t, ctx, pool, updateResult.AuditID, siteaudit.StatusPending)
	var sourceHost string
	if err := pool.QueryRow(ctx, `SELECT normalized_host FROM directory.sites WHERE id = $1`, updateSiteID).Scan(&sourceHost); err != nil {
		t.Fatalf("read source site after conflicting update: %v", err)
	}
	if sourceHost != "update-source.example.com" {
		t.Fatalf("source host = %q, want transaction rollback", sourceHost)
	}

	// Given a unique CREATE request.
	successResult, err := service.Submit(ctx, siteaudit.ActionCreate, "", siteauditConflictSubmission("https://review-success.example.com", taxonomy, programID))
	if err != nil {
		t.Fatalf("submit successful site creation: %v", err)
	}

	// When it is approved, then the ordinary lifecycle remains successful.
	reviewed, err := service.Review(ctx, reviewActor, siteaudit.ReviewInput{AuditID: successResult.AuditID, Decision: siteaudit.DecisionApprove})
	if err != nil {
		t.Fatalf("approve successful site creation: %v", err)
	}
	if reviewed.Status != siteaudit.StatusApproved || reviewed.SiteID == "" {
		t.Fatalf("reviewed creation = (status:%q site:%q), want approved with site ID", reviewed.Status, reviewed.SiteID)
	}
}

type siteauditConflictTaxonomy struct {
	Level1ID string
	Level2ID string
}

func siteauditConflictSubmission(urlValue string, taxonomy siteauditConflictTaxonomy, programID string) siteaudit.SubmissionInput {
	return siteaudit.SubmissionInput{Site: siteaudit.SiteInput{
		Name: "Review Address", URL: urlValue,
		Tags: []siteaudit.TagInput{
			{ID: taxonomy.Level1ID, Role: "PRIMARY", Level: 1},
			{ID: taxonomy.Level2ID, Role: "SECONDARY", Level: 2, ParentID: taxonomy.Level1ID},
		},
		Components: []siteaudit.ComponentInput{{ID: programID, Role: "SITE_PROGRAM"}},
	}, Reason: "Review address conflict integration scenario."}
}

func assertSiteAuditServiceError(t *testing.T, err error, code string, status int) {
	t.Helper()
	var serviceError *siteaudit.ServiceError
	if !errors.As(err, &serviceError) || serviceError.Code != code || serviceError.StatusCode != status {
		t.Fatalf("service error = %v, want (%q, %d)", err, code, status)
	}
}

func assertSiteAuditStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, auditID string, status siteaudit.Status) {
	t.Helper()
	var actual string
	if err := pool.QueryRow(ctx, `SELECT status FROM directory.site_audits WHERE id = $1`, auditID).Scan(&actual); err != nil {
		t.Fatalf("read audit status: %v", err)
	}
	if actual != string(status) {
		t.Fatalf("audit status = %q, want %q", actual, status)
	}
}

func integrationUUIDText(t *testing.T, value pgtype.UUID) string {
	t.Helper()
	driverValue, err := value.Value()
	if err != nil {
		t.Fatalf("format UUID: %v", err)
	}
	text, ok := driverValue.(string)
	if !ok {
		t.Fatalf("UUID value type = %T, want string", driverValue)
	}
	return text
}
