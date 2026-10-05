package siteaudit

import "testing"

func TestMergeRequestedSnapshotMovesClassificationWithTags(t *testing.T) {
	base := Snapshot{TagCascadeID: "old", Tags: []TagSnapshot{{ID: "old-secondary", Role: "SECONDARY", Level: 2}}}
	proposed := Snapshot{TagCascadeID: "new", Classification: &CascadeSnapshot{ID: "new"}, Tags: []TagSnapshot{{ID: "new-secondary", Role: "SECONDARY", Level: 2}}}
	merged, conflicts := MergeRequestedSnapshot(base, proposed, base)
	if len(conflicts) != 0 || merged.TagCascadeID != "new" || merged.Classification == nil || merged.Classification.ID != "new" {
		t.Fatalf("classification was not merged with tags: %#v", merged)
	}
}
