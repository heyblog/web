//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/siteaudit"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func verifySiteAuditComponentCreation(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	// Given a submission proposing a new program and framework.
	queries := dbgen.New(pool)
	cascades, err := queries.ListEnabledSiteTagCascades(ctx)
	if err != nil || len(cascades) == 0 {
		t.Fatalf("load component review taxonomy: %v", err)
	}
	user, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email: "audit-components@example.test", Username: "audit_components", DisplayName: "Component reviewer",
	})
	if err != nil {
		t.Fatal(err)
	}
	reviewer := auth.User{ID: integrationUUIDText(t, user.ID), Role: auth.RoleSysAdmin}
	input := siteauditConflictSubmission("https://audit-components.example.test", siteauditConflictTaxonomy{
		Level1ID: integrationUUIDText(t, cascades[0].Level1ID), Level2ID: integrationUUIDText(t, cascades[0].Level2ID),
	}, "")
	openSource := true
	input.Site.Components = []siteaudit.ComponentInput{{
		SuggestedName: "  CuStOm Program  ", Role: "SITE_PROGRAM",
		HomepageURL: "https://program.example.test", IsOpenSource: &openSource,
	}}
	input.Site.ProgramDependencies = []siteaudit.ComponentInput{{
		SuggestedName: "  CuStOm Framework  ", Role: "FRAMEWORK",
		RepositoryURL: "https://framework.example.test/repository", IsOpenSource: &openSource,
	}}
	service := siteaudit.NewService(siteaudit.Dependencies{
		Repository: siteaudit.NewRepository(pool), NewShortID: func() (string, error) { return "Comp12345", nil },
	})
	created, err := service.Submit(ctx, siteaudit.ActionCreate, "", input)
	if err != nil {
		t.Fatalf("submit custom program and framework: %v", err)
	}

	// When the reviewer approves both new catalog entries in one audit.
	approved, err := service.Review(ctx, reviewer, siteaudit.ReviewInput{
		AuditID: created.AuditID, Decision: siteaudit.DecisionApprove,
	})
	if err != nil {
		t.Fatalf("approve custom program and framework: %v", err)
	}

	// Then the decision, canonical names, and associations are committed together.
	if approved.Status != siteaudit.StatusApproved {
		t.Fatalf("audit status = %q, want APPROVED", approved.Status)
	}
	assertSiteAuditStatus(t, ctx, pool, created.AuditID, siteaudit.StatusApproved)
	site, err := queries.GetSiteByShortID(ctx, "Comp12345")
	if err != nil {
		t.Fatalf("load approved site: %v", err)
	}
	components, err := queries.ListSiteSoftwareComponents(ctx, site.ID)
	if err != nil || len(components) != 1 {
		t.Fatalf("site components = %#v / %v", components, err)
	}
	program := components[0]
	if program.Name != "CuStOm Program" || program.NormalizedName != "custom program" || program.Role != "SITE_PROGRAM" {
		t.Fatalf("stored program = %#v", program)
	}
	dependencies, err := queries.ListSoftwareComponentDependencies(ctx, program.ComponentID)
	if err != nil || len(dependencies) != 1 {
		t.Fatalf("program dependencies = %#v / %v", dependencies, err)
	}
	framework := dependencies[0]
	if framework.Name != "CuStOm Framework" || framework.NormalizedName != "custom framework" || framework.Role != "FRAMEWORK" {
		t.Fatalf("stored framework = %#v", framework)
	}
	if len(approved.FinalSnapshot.Components) != 1 || len(approved.FinalSnapshot.ProgramDependencies) != 1 {
		t.Fatalf("final architecture = %#v", approved.FinalSnapshot)
	}
	finalProgram := approved.FinalSnapshot.Components[0]
	finalFramework := approved.FinalSnapshot.ProgramDependencies[0]
	if finalProgram.ID != integrationUUIDText(t, program.ComponentID) || finalProgram.SuggestedName != "" ||
		finalFramework.ID != integrationUUIDText(t, framework.DependencyComponentID) || finalFramework.SuggestedName != "" {
		t.Fatalf("final components = %#v / %#v, want canonical catalog identities", finalProgram, finalFramework)
	}
}
