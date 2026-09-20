package dataimport

import (
	"encoding/json"
	"testing"
)

func TestDecodeTagTaxonomyBundleAcceptsOrderedTertiaryAssignments(t *testing.T) {
	t.Parallel()

	bundle := validTagTaxonomyBundle()
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeTagTaxonomyBundle(data)
	if err != nil {
		t.Fatalf("DecodeTagTaxonomyBundle() error = %v", err)
	}
	if decoded.SiteCount != 1 || decoded.TertiaryCount != 1 {
		t.Fatalf("decoded counts = (%d, %d), want (1, 1)", decoded.SiteCount, decoded.TertiaryCount)
	}
}

func TestDecodeTagTaxonomyBundleRejectsClassificationRepeatedAsTertiary(t *testing.T) {
	t.Parallel()

	bundle := validTagTaxonomyBundle()
	bundle.Sites[0].TertiaryTags[0] = TagTaxonomyTertiary{Source: "SQLITE", TagID: "computer", Name: "计算机", Position: 1}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := DecodeTagTaxonomyBundle(data); err == nil {
		t.Fatal("DecodeTagTaxonomyBundle() error = nil, want repeated classification rejected")
	}
}

func validTagTaxonomyBundle() TagTaxonomyBundle {
	level1, level2 := 1, 2
	parent, order1, order2 := "computer", 1, 2
	return TagTaxonomyBundle{
		Format: taxonomyFormat, Version: 1, GeneratedAt: "2026-09-05T00:00:00Z",
		Inputs: []TagTaxonomyInputMetadata{
			{Kind: "blogs_cleaned", File: "blogs.cleaned.json", SHA256: string(make([]byte, 64)), Count: 1},
			{Kind: "sqlite", File: "tags.sqlite3", SHA256: string(make([]byte, 64)), Count: 3},
		},
		TagCount: 3, CascadeCount: 2, SiteCount: 1, TertiaryCount: 1,
		Tags: []TagTaxonomyDefinition{
			{Source: "SQLITE", TagID: "computer", Name: "计算机", TaxonomyLevel: &level1, SortOrder: &order1},
			{Source: "SQLITE", TagID: "topic-computer-development", Name: "开发", TaxonomyLevel: &level2, ParentTagID: &parent, SortOrder: &order2},
			{Source: "LEGACY", TagID: "01900000-0000-7000-8000-000000000001", Name: "Go"},
		},
		Cascades: []TagTaxonomyCascade{
			{Scope: "SITE", CascadeKey: "computer/topic-computer-development", Level1TagID: "computer", Level2TagID: "topic-computer-development", SortOrder: 1},
			{Scope: "ARTICLE", CascadeKey: "computer/topic-computer-development", Level1TagID: "computer", Level2TagID: "topic-computer-development", SortOrder: 1},
		},
		Sites: []SiteTagTaxonomyMigration{{
			SiteID: "01900000-0000-7000-8000-000000000010", SourceURL: "https://example.com/", MatchMethod: "EXACT_URL",
			CascadeKey:   "computer/topic-computer-development",
			TertiaryTags: []TagTaxonomyTertiary{{Source: "LEGACY", TagID: "01900000-0000-7000-8000-000000000001", Name: "Go", Position: 1}},
		}},
	}
}
