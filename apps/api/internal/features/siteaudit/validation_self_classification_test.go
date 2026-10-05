package siteaudit

import "testing"

func TestSelfClassificationRetainsBothRoles(t *testing.T) {
	tags, err := normalizeTags([]TagInput{{ID: "other", Role: "PRIMARY", Level: 1}, {ID: "other", Role: "SECONDARY", Level: 2, ParentID: "other"}})
	if err != nil || len(tags) != 2 || tags[0].Role == tags[1].Role {
		t.Fatalf("self classification: %#v %v", tags, err)
	}
	_, err = normalizeTags([]TagInput{{ID: "other", Role: "PRIMARY", Level: 1}, {ID: "other", Role: "SECONDARY", Level: 2}, {ID: "other", Role: "TERTIARY", Level: 3}})
	if err == nil {
		t.Fatal("tertiary repeated current classification")
	}
	tags, err = normalizeTags([]TagInput{{ID: "computer", Role: "PRIMARY", Level: 1}, {ID: "development", Role: "SECONDARY", Level: 2}, {ID: "other", Role: "TERTIARY", Level: 3}})
	if err != nil || len(tags) != 3 {
		t.Fatal("global classifier forbidden as unrelated tertiary", err)
	}
}
