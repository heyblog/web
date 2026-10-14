//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/siteaudit"
	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5/pgxpool"
)

func verifySiteAuditAccessScope(ctx context.Context, t *testing.T, pool *pgxpool.Pool, reviewer auth.User, input siteaudit.SiteInput) {
	t.Helper()
	repository := siteaudit.NewRepository(pool)
	queries := dbgen.New(pool)
	for index, scenario := range []struct {
		name, override, want string
		save                 bool
	}{
		{name: "legacy-create", want: "ALL"},
		{name: "default-draft", want: "ALL", save: true},
		{name: "admin-draft", override: "CN_ONLY", want: "CN_ONLY", save: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			shortID := fmt.Sprintf("Scope%04d", index)
			service := siteaudit.NewService(siteaudit.Dependencies{Repository: repository, NewShortID: func() (string, error) { return shortID, nil }})
			input.URL = fmt.Sprintf("https://scope-%d.example.com", index)
			proposed, err := siteaudit.BuildProposedSnapshot(input, siteaudit.Snapshot{AccessScope: "GLOBAL_ONLY", Visibility: "VISIBLE"})
			if err != nil {
				t.Fatal(err)
			}
			proposed, err = repository.PrepareSubmission(ctx, proposed, siteaudit.Snapshot{})
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(proposed)
			if err != nil {
				t.Fatal(err)
			}
			row, err := queries.CreateSiteAudit(ctx, dbgen.CreateSiteAuditParams{LookupSecretHash: bytes.Repeat([]byte{byte(170 + index)}, 32), Action: "CREATE", ProposedSnapshot: encoded})
			if err != nil {
				t.Fatal(err)
			}
			auditID := integrationUUIDText(t, row.ID)
			detail, err := service.AuditDetail(ctx, auditID)
			if err != nil || detail.ProposedSnapshot.AccessScope != "GLOBAL_ONLY" || detail.EffectiveSnapshot.AccessScope != "ALL" {
				t.Fatalf("historical create detail = %#v / %v", detail, err)
			}
			revision := int64(0)
			if scenario.save {
				draftInput := siteaudit.ReviewDraftInput{AuditID: auditID, Site: siteaudit.ReviewSiteInput{SiteInput: input, AccessScope: scenario.override}}
				_, err = service.SaveReviewDraft(ctx, auth.User{ID: reviewer.ID, Role: auth.RoleUser}, draftInput)
				assertSiteAuditServiceError(t, err, "forbidden", http.StatusForbidden)
				saved, err := service.SaveReviewDraft(ctx, reviewer, draftInput)
				if err != nil || saved.ReviewDraftSnapshot.AccessScope != scenario.want {
					t.Fatalf("draft = %#v / %v", saved, err)
				}
				revision = saved.ReviewDraftRevision
				// An omitted field on a subsequent save preserves the authorized saved scope.
				draftInput.Site.AccessScope = ""
				draftInput.ExpectedReviewDraftRevision = revision
				saved, err = service.SaveReviewDraft(ctx, reviewer, draftInput)
				if err != nil || saved.ReviewDraftSnapshot.AccessScope != scenario.want {
					t.Fatalf("resaved draft = %#v / %v", saved, err)
				}
				revision = saved.ReviewDraftRevision
			}
			approved, err := service.Review(ctx, reviewer, siteaudit.ReviewInput{AuditID: auditID, Decision: siteaudit.DecisionApprove, ExpectedReviewDraftRevision: revision})
			if err != nil || approved.FinalSnapshot.AccessScope != scenario.want {
				t.Fatalf("create final = %#v / %v", approved, err)
			}
			verifySiteAuditScopeDrift(ctx, t, pool, service, reviewer, input, shortID)
		})
	}
}

func verifySiteAuditScopeDrift(ctx context.Context, t *testing.T, pool *pgxpool.Pool, service *siteaudit.Service, reviewer auth.User, input siteaudit.SiteInput, shortID string) {
	t.Helper()
	for index, scope := range []string{"CN_ONLY", "GLOBAL_ONLY"} {
		current, err := service.ResolveSite(ctx, shortID)
		if err != nil {
			t.Fatal(err)
		}
		proposed, err := siteaudit.BuildProposedSnapshot(input, current)
		if err != nil {
			t.Fatal(err)
		}
		proposed.Name = fmt.Sprintf("Scope update %d", index)
		proposed.AccessScope = "ALL" // Historical applicant-controlled field.
		encoded, err := json.Marshal(proposed)
		if err != nil {
			t.Fatal(err)
		}
		base, err := json.Marshal(current)
		if err != nil {
			t.Fatal(err)
		}
		siteRow, err := dbgen.New(pool).GetSiteByShortID(ctx, shortID)
		if err != nil {
			t.Fatal(err)
		}
		hash := []byte(fmt.Sprintf("%032s", shortID+scope))
		row, err := dbgen.New(pool).CreateSiteAudit(ctx, dbgen.CreateSiteAuditParams{LookupSecretHash: hash, Action: "UPDATE", SiteID: siteRow.ID, BaseRevision: &current.Revision, BaseSnapshot: base, ProposedSnapshot: encoded, RequestReason: "Update name"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "UPDATE directory.sites SET access_scope = $1 WHERE short_id = $2", scope, shortID); err != nil {
			t.Fatal(err)
		}
		auditID := integrationUUIDText(t, row.ID)
		detail, err := service.AuditDetail(ctx, auditID)
		if err != nil || detail.EffectiveSnapshot.AccessScope != scope || len(detail.Diff.Conflicts) != 0 {
			t.Fatalf("update detail = %#v / %v", detail, err)
		}
		_, err = service.Review(ctx, reviewer, siteaudit.ReviewInput{AuditID: auditID, Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: current.Revision})
		assertSiteAuditServiceError(t, err, "site_revision_changed", http.StatusConflict)
		approved, err := service.Review(ctx, reviewer, siteaudit.ReviewInput{AuditID: auditID, Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: detail.CurrentSnapshot.Revision})
		if err != nil || approved.FinalSnapshot.AccessScope != scope || approved.FinalSnapshot.Name != proposed.Name {
			t.Fatalf("update final = %#v / %v", approved, err)
		}
	}
}
