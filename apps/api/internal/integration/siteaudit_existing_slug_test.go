//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/siteaudit"
	"heyblog-api/internal/features/sluggeneration"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/platform/config"
)

func TestReviewExistingTagRespectsReactivationPermission(t *testing.T) {
	for _, name := range []string{"enabled", "disabled"} {
		t.Run(name, func(t *testing.T) {
			// Given a named dictionary tag and a reviewer with audit permission only.
			f := newAuditMigrationFixture(t)
			ctx := t.Context()
			_, err := f.provider.Up(ctx)
			require.NoError(t, err)
			var tagID, reviewerID string
			tagName, slug := "Reviewed existing "+name, "reviewed-existing-"+name
			require.NoError(t, f.pool.QueryRow(ctx, `INSERT INTO directory.tags(name,normalized_name,slug,is_enabled) VALUES($1,lower($1),$2,$3) RETURNING id::text`, tagName, slug, name == "enabled").Scan(&tagID))
			require.NoError(t, f.pool.QueryRow(ctx, `INSERT INTO identity.users(email,username,display_name,role) VALUES($1,$2,'Reviewer','ADMIN') RETURNING id::text`, "review-existing-"+name+"@example.test", "review_existing_"+name).Scan(&reviewerID))
			reviewer := auth.User{ID: reviewerID, Role: auth.RoleAdmin, Permissions: []auth.Permission{auth.PermissionSiteAuditReview}}
			generation := sluggeneration.NewService(sluggeneration.Dependencies{Pool: f.pool, Config: config.AIConfig{DefaultModel: "deepseek/deepseek-flash"}})
			service := siteaudit.NewService(siteaudit.Dependencies{
				Repository: siteaudit.NewRepository(f.pool),
				NewShortID: func() (string, error) { return "SlgOk1234", nil },
				SlugGenerator: func(ctx context.Context, user auth.User, tags []siteaudit.TagSnapshot) ([]siteaudit.TagSnapshot, error) {
					return generation.ReviewedTags(ctx, user, "127.0.0.1", tags)
				},
			})
			queries := dbgen.New(f.pool)
			paths, err := queries.ListEnabledSiteTagCascades(ctx)
			require.NoError(t, err)
			require.NotEmpty(t, paths)
			program, err := queries.GetSoftwareComponentByNormalizedName(ctx, "其他")
			require.NoError(t, err)
			input := siteauditConflictSubmission("https://review-existing-"+name+".example.test", siteauditConflictTaxonomy{Level1ID: integrationUUIDText(t, paths[0].Level1ID), Level2ID: integrationUUIDText(t, paths[0].Level2ID)}, integrationUUIDText(t, program.ID))
			input.Site.Tags = append(input.Site.Tags, siteaudit.TagInput{SuggestedName: tagName, Role: "TERTIARY", Level: 3})
			submitted, err := service.Submit(ctx, siteaudit.ActionCreate, "", input)
			require.NoError(t, err)

			// When approval automatically fills the matching dictionary slug.
			approved, err := service.Review(ctx, reviewer, siteaudit.ReviewInput{AuditID: submitted.AuditID, Decision: siteaudit.DecisionApprove})

			// Then existing enabled tags need no new privilege; disabled tags still require reactivation authority.
			if name == "disabled" {
				assertSiteAuditServiceError(t, err, "taxonomy_permission_required", 403)
				assertSiteAuditStatus(t, ctx, f.pool, submitted.AuditID, siteaudit.StatusPending)
				return
			}
			require.NoError(t, err)
			require.Equal(t, siteaudit.StatusApproved, approved.Status)
			var count int
			require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM directory.site_tags WHERE site_id=$1::uuid AND tag_id=$2::uuid`, approved.SiteID, tagID).Scan(&count))
			require.Equal(t, 1, count)
		})
	}
}
