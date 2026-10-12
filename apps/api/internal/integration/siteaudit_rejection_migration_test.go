//go:build integration

package integration_test

import (
	"net/http"
	"strings"
	"testing"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/siteaudit"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func TestSiteAuditRejectionPreservesSubmission(t *testing.T) {
	ctx := t.Context()
	fixture := newAuditMigrationFixture(t)
	if _, err := fixture.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	duplicate := fixture.pendingCreate(t, "duplicate-audit.example.test")
	ordinary := fixture.pendingCreate(t, "ordinary-audit.example.test")
	approval := fixture.pendingCreate(t, "approved-audit.example.test")
	fixture.addReviewHistory(t, approval.AuditID)
	queries := dbgen.New(fixture.pool)
	reviewer, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email: "audit-migration@example.test", Username: "audit_migration", DisplayName: "Audit Migration",
	})
	if err != nil {
		t.Fatalf("create audit reviewer: %v", err)
	}
	actor := auth.User{ID: integrationUUIDText(t, reviewer.ID), Role: auth.RoleSysAdmin}
	service := siteaudit.NewService(siteaudit.Dependencies{
		Repository: siteaudit.NewRepository(fixture.pool),
		NewShortID: func() (string, error) { return "7Pq8Rs9Tu", nil },
	})
	insertSite(ctx, t, fixture.pool, "6Nop7Qr8S", "Canonical site", "duplicate-audit.example.test")
	_, err = service.Review(ctx, actor, siteaudit.ReviewInput{AuditID: duplicate.AuditID, Decision: siteaudit.DecisionApprove})
	assertSiteAuditServiceError(t, err, "site_address_conflict", http.StatusConflict)
	assertSiteAuditStatus(t, ctx, fixture.pool, duplicate.AuditID, siteaudit.StatusPending)

	// Neither historical email-only contacts nor duplicate hosts prevent rejection.
	for _, audit := range []siteaudit.SubmissionResult{duplicate, ordinary} {
		_, err = service.Review(ctx, actor, siteaudit.ReviewInput{AuditID: audit.AuditID, Decision: siteaudit.DecisionReject})
		assertSiteAuditServiceError(t, err, "review_comment_required", http.StatusUnprocessableEntity)
		reviewed, rejectErr := service.Review(ctx, actor, siteaudit.ReviewInput{
			AuditID: audit.AuditID, Decision: siteaudit.DecisionReject, ReviewerComment: "Request declined",
		})
		if rejectErr != nil {
			t.Fatalf("reject pending CREATE: %v", rejectErr)
		}
		if reviewed.Status != siteaudit.StatusRejected || reviewed.SiteID != "" || reviewed.ReviewedBy != actor.ID || reviewed.ReviewedAt == nil {
			t.Fatalf("rejection outcome = %#v", reviewed)
		}
		if reviewed.SubmitterName != "" || reviewed.SubmitterEmail != "historical@example.test" || !reviewed.NotifyByEmail {
			t.Fatal("rejection changed historical contact preferences")
		}
		public, queryErr := service.Query(ctx, audit.LookupToken)
		if queryErr != nil || public.Status != siteaudit.StatusRejected || public.ReviewerComment != "Request declined" {
			t.Fatalf("anonymous rejection lookup = %#v / %v", public, queryErr)
		}
		_, err = service.Review(ctx, actor, siteaudit.ReviewInput{
			AuditID: audit.AuditID, Decision: siteaudit.DecisionReject, ReviewerComment: "Repeated decision",
		})
		assertSiteAuditServiceError(t, err, "audit_already_reviewed", http.StatusConflict)
	}

	var siteCount int
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM directory.sites`).Scan(&siteCount); err != nil || siteCount != 2 {
		t.Fatalf("sites after rejection = %d / %v, want unchanged canonical sites", siteCount, err)
	}
	// Approval must still require a site, and successful service approval supplies one.
	_, err = fixture.pool.Exec(ctx, `UPDATE directory.site_audits SET status = 'APPROVED',
		final_snapshot = proposed_snapshot, reviewed_by = $2, reviewed_at = now() WHERE id = $1`, approval.AuditID, actor.ID)
	if err == nil || !strings.Contains(err.Error(), "site_audits_create_site_check") {
		t.Fatalf("approval without site should fail its constraint: %v", err)
	}
	reviewed, err := service.Review(ctx, actor, siteaudit.ReviewInput{
		AuditID: approval.AuditID, Decision: siteaudit.DecisionApprove, ExpectedReviewDraftRevision: 1,
	})
	if err != nil || reviewed.Status != siteaudit.StatusApproved || reviewed.SiteID == "" {
		t.Fatalf("ordinary approval = %q / %q / %v", reviewed.Status, reviewed.SiteID, err)
	}
	_, err = fixture.pool.Exec(ctx, `UPDATE directory.site_audits SET request_reason = 'changed' WHERE id = $1`, duplicate.AuditID)
	if err == nil || !strings.Contains(err.Error(), "submission fields are immutable") {
		t.Fatalf("submission immutability must survive migration: %v", err)
	}
}
