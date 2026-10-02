package siteaudit

import (
	"errors"
	"testing"

	"heyblog-api/internal/domain/site"
)

func validTagInputs() []TagInput {
	return []TagInput{
		{ID: "level-1", Role: "PRIMARY", Level: 1},
		{ID: "level-2", Role: "SECONDARY", Level: 2, ParentID: "level-1"},
	}
}

func TestBuildProposedSnapshotRequiresOneFixedClassification(t *testing.T) {
	t.Parallel()

	_, err := BuildProposedSnapshot(SiteInput{
		Name: "Example",
		URL:  "https://example.test",
		Tags: []TagInput{{ID: "tag-id", Role: "SECONDARY", Level: 2}},
	}, Snapshot{AccessScope: "ALL", Visibility: "VISIBLE"})

	if !errors.Is(err, ErrInvalidSubmission) {
		t.Fatalf("BuildProposedSnapshot() error = %v, want ErrInvalidSubmission", err)
	}
}

func TestBuildProposedSnapshotRequiresSiteProgram(t *testing.T) {
	t.Parallel()

	_, err := BuildProposedSnapshot(SiteInput{
		Name: "Example",
		URL:  "https://example.test",
		Tags: validTagInputs(),
	}, Snapshot{AccessScope: "ALL", Visibility: "VISIBLE"})

	if !errors.Is(err, ErrInvalidSubmission) {
		t.Fatalf("BuildProposedSnapshot() error = %v, want ErrInvalidSubmission", err)
	}
}

func TestNormalizeSubmissionAllowsCreateWithoutReason(t *testing.T) {
	t.Parallel()

	if _, err := NormalizeSubmission(ActionCreate, SubmissionInput{}); err != nil {
		t.Fatalf("NormalizeSubmission(CREATE) error = %v, want nil", err)
	}
}

func TestNormalizeSubmissionRequiresReasonForNonCreateActions(t *testing.T) {
	t.Parallel()

	if _, err := NormalizeSubmission(ActionUpdate, SubmissionInput{}); !errors.Is(err, ErrInvalidSubmission) {
		t.Fatalf("NormalizeSubmission(UPDATE) error = %v, want ErrInvalidSubmission", err)
	}
}

func TestNormalizeSubmissionRequiresContactNameAndEmailTogether(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		contact ContactInput
		valid   bool
	}{
		{name: "both empty", contact: ContactInput{}, valid: true},
		{name: "name only", contact: ContactInput{Name: "Owner"}},
		{name: "email only", contact: ContactInput{Email: "owner@example.test"}},
		{name: "both filled", contact: ContactInput{Name: "Owner", Email: "owner@example.test"}, valid: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NormalizeSubmission(ActionCreate, SubmissionInput{Contact: test.contact})
			if test.valid && err != nil {
				t.Fatalf("NormalizeSubmission() error = %v, want nil", err)
			}
			if !test.valid && !errors.Is(err, ErrInvalidSubmission) {
				t.Fatalf("NormalizeSubmission() error = %v, want ErrInvalidSubmission", err)
			}
		})
	}
}

func TestNormalizeFeedsRejectsHomepageURL(t *testing.T) {
	t.Parallel()

	address := site.Address{Scheme: "https", NormalizedHost: "example.test", BasePath: "/blog"}
	_, err := normalizeFeeds([]FeedInput{{Name: "Main", URL: "http://example.test/blog#latest", Format: "RSS", IsDefault: true}}, address)

	if !errors.Is(err, ErrSiteURLPurposeConflict) || !errors.Is(err, ErrInvalidSubmission) {
		t.Fatalf("normalizeFeeds() error = %v, want URL purpose and invalid submission errors", err)
	}
}

func TestNormalizeFeedsKeepsDuplicateURLsAsInvalidSubmission(t *testing.T) {
	t.Parallel()

	address := site.Address{Scheme: "https", NormalizedHost: "example.test", BasePath: "/blog"}
	_, err := normalizeFeeds([]FeedInput{
		{Name: "Main", URL: "/feed.xml", Format: "RSS", IsDefault: true},
		{Name: "Mirror", URL: "https://example.test/feed.xml#latest", Format: "RSS"},
	}, address)

	if !errors.Is(err, ErrInvalidSubmission) || errors.Is(err, ErrSiteURLPurposeConflict) {
		t.Fatalf("normalizeFeeds() error = %v, want only ErrInvalidSubmission", err)
	}
}

func TestNormalizeResourcesRejectsHomepageAndDuplicateURLs(t *testing.T) {
	t.Parallel()

	address := site.Address{Scheme: "https", NormalizedHost: "example.test", BasePath: "/blog"}
	tests := []struct {
		name      string
		resources []ResourceInput
	}{
		{name: "homepage", resources: []ResourceInput{{Kind: "SITEMAP", URL: "/blog"}}},
		{name: "duplicate", resources: []ResourceInput{
			{Kind: "SITEMAP", URL: "/sitemap.xml"},
			{Kind: "LINK_PAGE", URL: "https://example.test/sitemap.xml#navigation"},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := normalizeResources(test.resources, address)
			if !errors.Is(err, ErrSiteURLPurposeConflict) || !errors.Is(err, ErrInvalidSubmission) {
				t.Fatalf("normalizeResources() error = %v, want URL purpose and invalid submission errors", err)
			}
		})
	}
}

func TestNormalizeLocationsAllowsOmittedOrDistinctURLs(t *testing.T) {
	t.Parallel()

	address := site.Address{Scheme: "https", NormalizedHost: "example.test", BasePath: "/blog"}
	if _, err := normalizeFeeds(nil, address); err != nil {
		t.Fatalf("normalizeFeeds(nil) error = %v", err)
	}
	if _, err := normalizeResources([]ResourceInput{
		{Kind: "SITEMAP", URL: "sitemap.xml"},
		{Kind: "LINK_PAGE", URL: "/friends"},
	}, address); err != nil {
		t.Fatalf("normalizeResources() error = %v", err)
	}
}

func TestNormalizeTagsRejectsWarningInEditableTaxonomy(t *testing.T) {
	t.Parallel()

	inputs := append(validTagInputs(), TagInput{ID: "warning", Role: "WARNING", Level: 3})
	_, err := normalizeTags(inputs)

	if !errors.Is(err, ErrInvalidSubmission) {
		t.Fatalf("normalizeTags() error = %v, want ErrInvalidSubmission", err)
	}
}

func TestBuildProposedSnapshotPreservesNonProgramSiteComponents(t *testing.T) {
	t.Parallel()

	openSource := true
	base := Snapshot{
		AccessScope: "ALL",
		Visibility:  "VISIBLE",
		Components: []ComponentSnapshot{
			{ID: "framework-id", Name: "Astro", Role: "FRAMEWORK", IsOpenSource: &openSource},
			{ID: "old-program", Name: "Old", Role: "SITE_PROGRAM", IsOpenSource: &openSource},
		},
	}
	proposed, err := BuildProposedSnapshot(SiteInput{
		Name:       "Example",
		URL:        "https://example.test",
		Tags:       validTagInputs(),
		Components: []ComponentInput{{ID: "new-program", Role: "SITE_PROGRAM"}},
	}, base)

	if err != nil {
		t.Fatalf("BuildProposedSnapshot() error = %v", err)
	}
	if len(proposed.Components) != 2 {
		t.Fatalf("component count = %d, want 2: %#v", len(proposed.Components), proposed.Components)
	}
	if proposed.Components[0].ID != "framework-id" || proposed.Components[1].ID != "new-program" {
		t.Fatalf("components = %#v, want preserved framework and replaced program", proposed.Components)
	}
}

func TestBuildProposedSnapshotAllowsCustomProgramFrameworkAndLanguageDependencies(t *testing.T) {
	t.Parallel()

	openSource := true
	proposed, err := BuildProposedSnapshot(SiteInput{
		Name: "Example",
		URL:  "https://example.test",
		Tags: validTagInputs(),
		Components: []ComponentInput{{
			SuggestedName: "Example Engine",
			Role:          "SITE_PROGRAM",
			RepositoryURL: "https://example.test/repository",
			IsOpenSource:  &openSource,
		}},
		ProgramDependencies: []ComponentInput{
			{ID: "framework-id", Role: "FRAMEWORK"},
			{SuggestedName: "Go", Role: "LANGUAGE"},
		},
	}, Snapshot{AccessScope: "ALL", Visibility: "VISIBLE"})

	if err != nil {
		t.Fatalf("BuildProposedSnapshot() error = %v", err)
	}
	if proposed.Components[0].HomepageURL != "https://example.test/repository" {
		t.Errorf("HomepageURL = %q, want repository fallback", proposed.Components[0].HomepageURL)
	}
	if len(proposed.ProgramDependencies) != 2 {
		t.Fatalf("dependency count = %d, want 2", len(proposed.ProgramDependencies))
	}
}

func TestBuildProposedSnapshotPreservesCustomDependencyMetadata(t *testing.T) {
	t.Parallel()

	openSource := true
	proposed, err := BuildProposedSnapshot(SiteInput{
		Name: "Example",
		URL:  "https://example.test",
		Tags: validTagInputs(),
		Components: []ComponentInput{{
			SuggestedName: "Example Engine",
			Role:          "SITE_PROGRAM",
			HomepageURL:   "https://engine.example",
			IsOpenSource:  &openSource,
		}},
		ProgramDependencies: []ComponentInput{{
			SuggestedName: "Custom runtime",
			Role:          "LANGUAGE",
			HomepageURL:   "https://runtime.example",
			RepositoryURL: "https://code.example/runtime",
			IsOpenSource:  &openSource,
		}},
	}, Snapshot{AccessScope: "ALL", Visibility: "VISIBLE"})

	if err != nil {
		t.Fatalf("BuildProposedSnapshot() error = %v", err)
	}
	dependency := proposed.ProgramDependencies[0]
	if dependency.HomepageURL != "https://runtime.example" || dependency.RepositoryURL != "https://code.example/runtime" {
		t.Errorf("dependency links = (%q, %q), want preserved metadata", dependency.HomepageURL, dependency.RepositoryURL)
	}
	if dependency.IsOpenSource == nil || !*dependency.IsOpenSource {
		t.Errorf("dependency open source = %v, want true", dependency.IsOpenSource)
	}
}

func TestBuildProposedSnapshotRejectsRuntimeDependencyFromPublicSubmission(t *testing.T) {
	t.Parallel()

	openSource := false
	_, err := BuildProposedSnapshot(SiteInput{
		Name: "Example",
		URL:  "https://example.test",
		Tags: validTagInputs(),
		Components: []ComponentInput{{
			SuggestedName: "Example Engine",
			Role:          "SITE_PROGRAM",
			HomepageURL:   "https://example.test/engine",
			IsOpenSource:  &openSource,
		}},
		ProgramDependencies: []ComponentInput{{ID: "runtime-id", Role: "RUNTIME"}},
	}, Snapshot{AccessScope: "ALL", Visibility: "VISIBLE"})

	if !errors.Is(err, ErrInvalidSubmission) {
		t.Fatalf("BuildProposedSnapshot() error = %v, want ErrInvalidSubmission", err)
	}
}
