package siteaudit

import "testing"

func TestBuildDiffViewsSeparatesRequestDriftAndReviewerCorrection(t *testing.T) {
	t.Parallel()

	base := Snapshot{Name: "Old", Summary: "Base"}
	proposed := Snapshot{Name: "Requested", Summary: "Base"}
	current := Snapshot{Name: "Old", Summary: "Changed elsewhere"}
	final := Snapshot{Name: "Reviewed", Summary: "Changed elsewhere"}

	views := BuildDiffViews(base, proposed, current, &final)

	assertDiffFields(t, views.Requested, []string{"name"})
	assertDiffFields(t, views.Drift, []string{"summary"})
	assertDiffFields(t, views.ReviewerCorrection, []string{"name", "summary"})
	if len(views.Conflicts) != 0 {
		t.Fatalf("conflicts = %#v, want none", views.Conflicts)
	}
}

func TestBuildDiffViewsMarksConcurrentChangeToRequestedFieldAsConflict(t *testing.T) {
	t.Parallel()

	base := Snapshot{Name: "Old"}
	proposed := Snapshot{Name: "Requested"}
	current := Snapshot{Name: "Changed elsewhere"}

	final := Snapshot{Name: "Resolved"}
	views := BuildDiffViews(base, proposed, current, &final)

	assertDiffFields(t, views.Conflicts, []string{"name"})
}

func TestBuildDiffViewsOmitsReviewerCorrectionWhenFinalSnapshotIsAbsent(t *testing.T) {
	t.Parallel()

	base := Snapshot{Name: "Old"}
	proposed := Snapshot{Name: "Requested"}

	views := BuildDiffViews(base, proposed, base, nil)

	assertDiffFields(t, views.ReviewerCorrection, nil)
}

func TestBuildDiffViewsIncludesProgramDependencies(t *testing.T) {
	t.Parallel()

	base := Snapshot{ProgramDependencies: []ComponentSnapshot{{ID: "astro", Name: "Astro", Role: "FRAMEWORK"}}}
	proposed := Snapshot{ProgramDependencies: []ComponentSnapshot{{ID: "go", Name: "Go", Role: "LANGUAGE"}}}

	views := BuildDiffViews(base, proposed, base, nil)

	assertDiffFields(t, views.Requested, []string{"program_dependencies"})
}

func TestBuildDiffViewsKeepsMultipleCustomDependenciesDistinct(t *testing.T) {
	t.Parallel()

	base := Snapshot{}
	proposed := Snapshot{ProgramDependencies: []ComponentSnapshot{
		{SuggestedName: "Astro", Role: "FRAMEWORK"},
		{SuggestedName: "Svelte", Role: "FRAMEWORK"},
	}}

	views := BuildDiffViews(base, proposed, base, nil)

	if len(views.Requested) != 1 || len(views.Requested[0].Added) != 2 {
		t.Fatalf("custom dependency additions = %#v, want two distinct entries", views.Requested)
	}
}

func assertDiffFields(t *testing.T, items []DiffItem, want []string) {
	t.Helper()
	if len(items) != len(want) {
		t.Fatalf("diff count = %d, want %d: %#v", len(items), len(want), items)
	}
	for index, field := range want {
		if items[index].Field != field {
			t.Errorf("diff[%d].Field = %q, want %q", index, items[index].Field, field)
		}
	}
}
