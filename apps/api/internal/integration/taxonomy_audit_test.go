//go:build integration

package integration_test

import (
	"testing"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/siteaudit"
	"heyblog-api/internal/features/taxonomy"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func TestTaxonomyPendingAuditCanonicalization(t *testing.T) {
	f := newAuditMigrationFixture(t)
	ctx := t.Context()
	if _, err := f.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	q := dbgen.New(f.pool)
	manager := taxonomy.NewService(f.pool, nil)
	catalog, err := manager.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var source, target taxonomy.Tag
	var parentID string
	for _, c := range catalog.Cascades {
		if c.Scope != "SITE" || c.Key == "other/topic-other-other" || c.PrimaryID == c.SecondaryID || c.Key == "life/topic-life-other" {
			continue
		}
		if parentID == "" {
			var protected bool
			for _, tag := range catalog.Tags {
				if tag.ID == c.SecondaryID && tag.Slug == "other" {
					protected = true
				}
			}
			if protected {
				continue
			}
			parentID = c.PrimaryID
			for _, tag := range catalog.Tags {
				if tag.ID == c.SecondaryID {
					source = tag
				}
			}
		}
	}
	for _, tag := range catalog.Tags {
		if tag.ID != source.ID && tag.ID != parentID && tag.Slug != "other" && tag.Enabled {
			target = tag
			break
		}
	}
	program, err := q.GetSoftwareComponentByNormalizedName(ctx, "其他")
	if err != nil {
		t.Fatal(err)
	}
	user, err := q.CreateUser(ctx, dbgen.CreateUserParams{Email: "taxonomy-review@example.test", Username: "taxonomy_review", DisplayName: "Reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	reviewer := auth.User{ID: integrationUUIDText(t, user.ID), Role: auth.RoleSysAdmin}
	service := siteaudit.NewService(siteaudit.Dependencies{Repository: siteaudit.NewRepository(f.pool), NewShortID: func() (string, error) { return "TaxA12345", nil }})
	input := siteauditConflictSubmission("https://taxonomy-audit.example.test", siteauditConflictTaxonomy{Level1ID: parentID, Level2ID: source.ID}, integrationUUIDText(t, program.ID))
	input.Site.Tags = append(input.Site.Tags, siteaudit.TagInput{SuggestedName: "Reviewed tag", Slug: "reviewed-tag", Role: "TERTIARY", Level: 3})
	submitted, err := service.Submit(ctx, siteaudit.ActionCreate, "", input)
	if err != nil {
		t.Fatal(err)
	}
	var original string
	if err = f.pool.QueryRow(ctx, `SELECT proposed_snapshot::text FROM directory.site_audits WHERE id=$1::uuid`, submitted.AuditID).Scan(&original); err != nil {
		t.Fatal(err)
	}
	catalog, err = manager.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	change := taxonomy.ChangeInput{Kind: "merge", SourceID: source.ID, TargetID: target.ID, ExpectedRevision: catalog.Revision}
	preview, err := manager.Preview(ctx, change)
	if err != nil || len(preview.Blockers) > 0 {
		t.Fatalf("merge %#v %v", preview, err)
	}
	change.Fingerprint = preview.Fingerprint
	if _, err = manager.Apply(ctx, change); err != nil {
		t.Fatal(err)
	}
	detail, err := service.AuditDetail(ctx, submitted.AuditID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.ProposedSnapshot.Classification.Level2.ID != target.ID || detail.ProposedSnapshot.Classification.Level1.ID != parentID {
		t.Fatalf("stale classification %#v", detail.ProposedSnapshot.Classification)
	}
	approved, err := service.Review(ctx, reviewer, siteaudit.ReviewInput{AuditID: submitted.AuditID, Decision: siteaudit.DecisionApprove})
	if err != nil || approved.Status != siteaudit.StatusApproved {
		t.Fatalf("approve merged snapshot %#v %v", approved, err)
	}
	var frozen string
	if err = f.pool.QueryRow(ctx, `SELECT proposed_snapshot::text FROM directory.site_audits WHERE id=$1::uuid`, submitted.AuditID).Scan(&frozen); err != nil || frozen != original {
		t.Fatal("approval rewrote original snapshot", err)
	}
	// Disabled classification remains readable and can be kept in an unrelated update and review draft.
	catalog, err = manager.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manager.Update(ctx, target.ID, taxonomy.UpdateInput{Name: target.Name, Slug: target.Slug, Enabled: false, ExpectedRevision: catalog.Revision}); err != nil {
		t.Fatal(err)
	}
	input.Site.Name = "Unrelated name change"
	input.Site.Tags = []siteaudit.TagInput{{ID: parentID, Role: "PRIMARY", Level: 1}, {ID: target.ID, Role: "SECONDARY", Level: 2, ParentID: parentID}}
	updated, err := service.Submit(ctx, siteaudit.ActionUpdate, "TaxA12345", input)
	if err != nil {
		t.Fatalf("submit disabled selection: %#v", err)
	}
	detail, err = service.AuditDetail(ctx, updated.AuditID)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.SaveReviewDraft(ctx, reviewer, siteaudit.ReviewDraftInput{AuditID: updated.AuditID, Site: input.Site, ExpectedSiteRevision: detail.CurrentSnapshot.Revision})
	if err != nil {
		t.Fatalf("save disabled selection: %v", err)
	}
	approved, err = service.Review(ctx, reviewer, siteaudit.ReviewInput{AuditID: updated.AuditID, Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: detail.CurrentSnapshot.Revision, ExpectedReviewDraftRevision: draft.ReviewDraftRevision})
	if err != nil || approved.Status != siteaudit.StatusApproved {
		t.Fatalf("approve disabled selection %#v %v", approved, err)
	}
}
