package dataimport

import "testing"

func TestValidatePlanAcceptsWarningSiteTag(t *testing.T) {
	t.Parallel()

	blogs, friends := testBundleJSON()
	bundles, err := DecodeBundles(blogs, friends)
	if err != nil {
		t.Fatalf("DecodeBundles() error = %v", err)
	}
	plan, err := BuildPlan(bundles, sequenceGenerator("AAAAAAAAA", "BBBBBBBBB"))
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	warningID := "0196d7f7-1000-7000-8000-000000000099"
	plan.Tags = append(plan.Tags, TagRow{
		ID: warningID, Name: "Content warning", NormalizedName: "content warning",
		Slug: "content-warning", IsEnabled: true,
	})
	plan.SiteTags = append(plan.SiteTags, SiteTagRow{
		SiteID: plan.Sites[0].ID, TagID: warningID, Role: "WARNING", Note: "Sensitive topic",
	})

	if err := validatePlan(plan); err != nil {
		t.Fatalf("validatePlan() error = %v", err)
	}
}

func TestValidatePlanRejectsMultiplePrimarySiteTags(t *testing.T) {
	t.Parallel()

	blogs, friends := testBundleJSON()
	bundles, err := DecodeBundles(blogs, friends)
	if err != nil {
		t.Fatalf("DecodeBundles() error = %v", err)
	}
	plan, err := BuildPlan(bundles, sequenceGenerator("AAAAAAAAA", "BBBBBBBBB"))
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	plan.SiteTags[1].Role = "PRIMARY"

	if err := validatePlan(plan); err == nil {
		t.Fatal("validatePlan() error = nil, want multiple primary tags rejected")
	}
}

func TestValidatePlanRejectsWhitespaceOnlySiteTagNote(t *testing.T) {
	t.Parallel()

	blogs, friends := testBundleJSON()
	bundles, err := DecodeBundles(blogs, friends)
	if err != nil {
		t.Fatalf("DecodeBundles() error = %v", err)
	}
	plan, err := BuildPlan(bundles, sequenceGenerator("AAAAAAAAA", "BBBBBBBBB"))
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	plan.SiteTags[0].Note = " \t\n"

	err = validatePlan(plan)
	if err == nil {
		t.Fatal("validatePlan() error = nil, want whitespace-only site tag note rejected")
	}
	if got, want := err.Error(), "site tag note must not contain only whitespace"; got != want {
		t.Fatalf("validatePlan() error = %q, want %q", got, want)
	}
}
