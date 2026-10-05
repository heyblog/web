package taxonomy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTagUsageCountsObjectsOnceAcrossSharedRoles(t *testing.T) {
	t.Parallel()
	// Given shared dictionary IDs, self paths, disabled paths and a retired path.
	g := graph{
		Catalog: Catalog{
			Tags: []Tag{{ID: "shared"}, {ID: "secondary"}, {ID: "tertiary"}, {ID: "warning"}, {ID: "retired"}, {ID: "unused"}},
			Cascades: []Cascade{
				{ID: "self", PrimaryID: "shared", SecondaryID: "shared", Enabled: true},
				{ID: "disabled", PrimaryID: "shared", SecondaryID: "secondary"},
				{ID: "retired", PrimaryID: "retired", SecondaryID: "retired", MergedIntoID: "self"},
			},
		},
		Objects: []object{
			{ID: "site", Scope: "SITE", CascadeID: "self", Tags: []assignment{{TagID: "shared", Role: "TERTIARY"}, {TagID: "tertiary", Role: "TERTIARY"}, {TagID: "warning", Role: "WARNING"}}},
			{ID: "article", Scope: "ARTICLE", CascadeID: "disabled", Tags: []assignment{{TagID: "secondary", Role: "TERTIARY"}, {TagID: "tertiary", Role: "TERTIARY"}}},
			{ID: "unclassified", Scope: "SITE", Tags: []assignment{{TagID: "shared", Role: "WARNING"}, {TagID: "unknown", Role: "TERTIARY"}}},
		},
	}

	// When usage is summarized for the management catalog.
	annotateTagUsage(&g)

	// Then classification and assignments contribute roles without double-counting objects.
	want := []Tag{
		{ID: "shared", Roles: []string{"PRIMARY", "SECONDARY", "TERTIARY", "WARNING"}, SiteCount: 2, ArticleCount: 1},
		{ID: "secondary", Roles: []string{"SECONDARY", "TERTIARY"}, ArticleCount: 1},
		{ID: "tertiary", Roles: []string{"TERTIARY"}, SiteCount: 1, ArticleCount: 1},
		{ID: "warning", Roles: []string{"WARNING"}, SiteCount: 1},
		{ID: "retired", Roles: []string{}},
		{ID: "unused", Roles: []string{}},
	}
	require.Equal(t, want, g.Tags)
}
