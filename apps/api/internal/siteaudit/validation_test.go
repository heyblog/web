package siteaudit

import (
	"errors"
	"testing"
)

func TestBuildProposedSnapshotRequiresOneExistingPrimaryTag(t *testing.T) {
	t.Parallel()

	_, err := BuildProposedSnapshot(SiteInput{
		Name: "Example",
		URL:  "https://example.test",
		Tags: []TagInput{{ID: "tag-id", Role: "SECONDARY"}},
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
		Tags: []TagInput{{ID: "tag-id", Role: "PRIMARY"}},
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
		Tags:       []TagInput{{ID: "tag-id", Role: "PRIMARY"}},
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
		Tags: []TagInput{{ID: "tag-id", Role: "PRIMARY"}},
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
		Tags: []TagInput{{ID: "tag-id", Role: "PRIMARY"}},
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
		Tags: []TagInput{{ID: "tag-id", Role: "PRIMARY"}},
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
