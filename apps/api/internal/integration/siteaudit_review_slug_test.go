//go:build integration

package integration_test

import (
	"context"
	"testing"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/siteaudit"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func TestReviewDraftGeneratesSlugForWhitespaceOnlyInput(t *testing.T) {
	// Given a pending audit and a reviewer correction with normalized empty fields.
	f := newAuditMigrationFixture(t)
	ctx := t.Context()
	if _, err := f.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	q := dbgen.New(f.pool)
	user, err := q.CreateUser(ctx, dbgen.CreateUserParams{Email: "draft-slug@example.test", Username: "draft_slug", DisplayName: "Reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	reviewer := auth.User{ID: integrationUUIDText(t, user.ID), Role: auth.RoleSysAdmin}
	service := siteaudit.NewService(siteaudit.Dependencies{
		Repository: siteaudit.NewRepository(f.pool),
		SlugGenerator: func(_ context.Context, _ auth.User, tags []siteaudit.TagSnapshot) ([]siteaudit.TagSnapshot, error) {
			for i := range tags {
				if tags[i].SuggestedName == "审核新标签" {
					tags[i].Slug = "review-new-tag"
				}
			}
			return tags, nil
		},
	})
	submitted := f.pendingCreate(t, "draft-slug.example.test")
	cascades, err := q.ListEnabledSiteTagCascades(ctx)
	if err != nil || len(cascades) == 0 {
		t.Fatalf("classification: %v", err)
	}
	program, err := q.GetSoftwareComponentByNormalizedName(ctx, "其他")
	if err != nil {
		t.Fatal(err)
	}
	input := siteauditConflictSubmission("https://draft-slug.example.test", siteauditConflictTaxonomy{
		Level1ID: integrationUUIDText(t, cascades[0].Level1ID), Level2ID: integrationUUIDText(t, cascades[0].Level2ID),
	}, integrationUUIDText(t, program.ID))
	input.Site.Tags = append(input.Site.Tags, siteaudit.TagInput{ID: " \t", SuggestedName: " 审核新标签 ", Slug: " \t", Description: " 描述 ", Role: "TERTIARY", Level: 3})

	// When the correction is saved, generation must survive input normalization.
	draft, err := service.SaveReviewDraft(ctx, reviewer, siteaudit.ReviewDraftInput{AuditID: submitted.AuditID, Site: input.Site})
	if err != nil {
		t.Fatal(err)
	}

	// Then the persisted preview contains the generated slug.
	if draft.ReviewDraftSnapshot == nil {
		t.Fatal("missing review draft")
	}
	for _, tag := range draft.ReviewDraftSnapshot.Tags {
		if tag.Name == "审核新标签" || tag.SuggestedName == "审核新标签" {
			if tag.Slug != "review-new-tag" {
				t.Fatalf("draft slug = %q, want review-new-tag", tag.Slug)
			}
			return
		}
	}
	t.Fatal("generated tag missing from draft")
}
