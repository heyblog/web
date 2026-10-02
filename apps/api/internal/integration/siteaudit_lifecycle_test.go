//go:build integration

package integration_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/siteaudit"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/infrastructure/mail"
)

type decisionObserver func(context.Context, mail.Message) error

func (observer decisionObserver) Send(ctx context.Context, message mail.Message) error {
	return observer(ctx, message)
}

func verifySiteAuditLifecycle(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	queries := dbgen.New(pool)
	cascades, err := queries.ListEnabledSiteTagCascades(ctx)
	if err != nil || len(cascades) == 0 {
		t.Fatalf("load lifecycle taxonomy: %v", err)
	}
	program, err := queries.GetSoftwareComponentByNormalizedName(ctx, "其他")
	if err != nil {
		t.Fatal(err)
	}
	user, err := queries.CreateUser(ctx, dbgen.CreateUserParams{Email: "audit-lifecycle@example.test", Username: "audit_lifecycle", DisplayName: "Reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	reviewer := auth.User{ID: integrationUUIDText(t, user.ID), Role: auth.RoleSysAdmin}
	input := siteauditConflictSubmission("https://audit-lifecycle.example.com", siteauditConflictTaxonomy{
		Level1ID: integrationUUIDText(t, cascades[0].Level1ID), Level2ID: integrationUUIDText(t, cascades[0].Level2ID),
	}, integrationUUIDText(t, program.ID))
	input.Contact = siteaudit.ContactInput{Name: "Submitter", Email: "submitter@example.test", NotifyByEmail: true}
	var notifiedAuditID string
	notifications := 0
	observer := decisionObserver(func(ctx context.Context, message mail.Message) error {
		notifications++
		var status string
		if err := pool.QueryRow(ctx, "SELECT status FROM directory.site_audits WHERE id = $1", notifiedAuditID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status == "PENDING" || message.To != input.Contact.Email {
			t.Fatalf("notification before commit or wrong recipient: %s / %s", status, message.To)
		}
		return mail.ErrDeliveryUnavailable // Delivery failure must not undo a committed decision.
	})
	service := siteaudit.NewService(siteaudit.Dependencies{
		Repository: siteaudit.NewRepository(pool), NewShortID: func() (string, error) { return "Life12345", nil },
		Mailer: mail.NewSubmissionMailer(observer, "audit@example.test"),
	})

	// Given a new submission, repeated lookup remains valid and the secret itself is never stored.
	created, err := service.Submit(ctx, siteaudit.ActionCreate, "", input)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		found, err := service.Query(ctx, created.LookupToken)
		if err != nil || found.Status != siteaudit.StatusPending || found.Action != siteaudit.ActionCreate {
			t.Fatalf("query pending audit = %#v / %v", found, err)
		}
	}
	_, err = service.Query(ctx, strings.Repeat("A", 43))
	assertSiteAuditServiceError(t, err, "audit_not_found", http.StatusNotFound)
	_, err = service.Submit(ctx, siteaudit.ActionCreate, "", input)
	assertSiteAuditServiceError(t, err, "submission_pending", http.StatusConflict)

	// When a correction is saved, stale saves/discards fail without replacing that correction.
	correction := input.Site
	correction.Name = "Corrected Name"
	draft, err := service.SaveReviewDraft(ctx, reviewer, siteaudit.ReviewDraftInput{AuditID: created.AuditID, Site: correction})
	if err != nil || draft.ReviewDraftRevision != 1 || draft.ReviewDraftSnapshot == nil || draft.ReviewDraftSnapshot.Name != correction.Name {
		t.Fatalf("save correction = %#v / %v", draft, err)
	}
	_, err = service.SaveReviewDraft(ctx, reviewer, siteaudit.ReviewDraftInput{AuditID: created.AuditID, Site: input.Site})
	assertSiteAuditServiceError(t, err, "review_draft_changed", http.StatusConflict)
	_, err = service.DiscardReviewDraft(ctx, reviewer, siteaudit.DiscardReviewDraftInput{AuditID: created.AuditID})
	assertSiteAuditServiceError(t, err, "review_draft_changed", http.StatusConflict)
	invalid := correction
	invalid.URL = "not a site URL"
	_, err = service.SaveReviewDraft(ctx, reviewer, siteaudit.ReviewDraftInput{AuditID: created.AuditID, Site: invalid, ExpectedReviewDraftRevision: 1})
	if !errors.Is(err, siteaudit.ErrInvalidSubmission) {
		t.Fatalf("invalid correction = %v", err)
	}
	detail, err := service.AuditDetail(ctx, created.AuditID)
	if err != nil || detail.ReviewDraftRevision != 1 || detail.ReviewDraftSnapshot.Name != correction.Name || detail.ProposedSnapshot.Name != input.Site.Name {
		t.Fatalf("correction rollback / frozen submission = %#v / %v", detail, err)
	}
	discarded, err := service.DiscardReviewDraft(ctx, reviewer, siteaudit.DiscardReviewDraftInput{AuditID: created.AuditID, ExpectedReviewDraftRevision: 1})
	if err != nil || discarded.ReviewDraftRevision != 2 || discarded.ReviewDraftSnapshot != nil {
		t.Fatalf("discard correction = %#v / %v", discarded, err)
	}

	// Then a stale decision does not notify; approval does so only after commit.
	notifiedAuditID = created.AuditID
	_, err = service.Review(ctx, reviewer, siteaudit.ReviewInput{AuditID: created.AuditID, Decision: siteaudit.DecisionApprove})
	assertSiteAuditServiceError(t, err, "review_draft_changed", http.StatusConflict)
	if notifications != 0 {
		t.Fatal("submission, draft, or failed review notified")
	}
	approved, err := service.Review(ctx, reviewer, siteaudit.ReviewInput{AuditID: created.AuditID, Decision: siteaudit.DecisionApprove, ExpectedReviewDraftRevision: 2})
	if err != nil || approved.Status != siteaudit.StatusApproved || notifications != 1 {
		t.Fatalf("approval = %#v / %v; notifications=%d", approved, err, notifications)
	}
	for range 2 {
		found, err := service.Query(ctx, created.LookupToken)
		if err != nil || found.Status != siteaudit.StatusApproved || found.ShortID != "" {
			t.Fatalf("query approved audit = %#v / %v", found, err)
		}
	}

	verifySiteAuditMaintenance(ctx, t, service, reviewer, input, &notifiedAuditID)
}

func verifySiteAuditMaintenance(ctx context.Context, t *testing.T, service *siteaudit.Service, reviewer auth.User, input siteaudit.SubmissionInput, notifiedAuditID *string) {
	t.Helper()
	// Given an approved site, UPDATE, DELETE, RESTORE keep revision and visibility semantics.
	for _, action := range []siteaudit.Action{siteaudit.ActionUpdate, siteaudit.ActionDelete, siteaudit.ActionRestore} {
		current, err := service.ResolveSite(ctx, "Life12345")
		if err != nil {
			t.Fatal(err)
		}
		input.Site.Name = "Updated Lifecycle"
		result, err := service.Submit(ctx, action, "Life12345", input)
		if err != nil {
			t.Fatalf("submit %s: %v", action, err)
		}
		*notifiedAuditID = result.AuditID
		_, err = service.Review(ctx, reviewer, siteaudit.ReviewInput{AuditID: result.AuditID, Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: current.Revision + 1})
		assertSiteAuditServiceError(t, err, "site_revision_changed", http.StatusConflict)
		approved, err := service.Review(ctx, reviewer, siteaudit.ReviewInput{AuditID: result.AuditID, Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: current.Revision})
		if err != nil || approved.FinalSnapshot.Revision != current.Revision+1 {
			t.Fatalf("approve %s = %#v / %v", action, approved, err)
		}
		wantVisibility := "VISIBLE"
		if action == siteaudit.ActionDelete {
			wantVisibility = "REMOVED"
		}
		if approved.FinalSnapshot.Visibility != wantVisibility {
			t.Fatalf("%s visibility = %s", action, approved.FinalSnapshot.Visibility)
		}
	}
}
