package sluggeneration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/siteaudit"
)

type reviewLookupStore struct {
	*memoryStore
	slugs map[string]string
}

func (store reviewLookupStore) ExistingSlug(_ context.Context, name string) (string, error) {
	return store.slugs[name], nil
}

func TestReviewReusesExistingSlugWithoutTaxonomyPermission(t *testing.T) {
	t.Parallel()
	// Given an audit reviewer and a proposal matching the global tag dictionary.
	service, store, provider, guard := testService()
	service.store = reviewLookupStore{memoryStore: store, slugs: map[string]string{"生活": "life"}}
	reviewer := auth.User{ID: "reviewer", Role: auth.RoleAdmin, Permissions: []auth.Permission{auth.PermissionSiteAuditReview}}
	tags := []siteaudit.TagSnapshot{{SuggestedName: "生活", Role: "TERTIARY", Level: 3}}

	// When the review fills missing slugs.
	result, err := service.ReviewedTags(context.Background(), reviewer, "ip", tags)

	// Then the canonical slug is reused without paid generation or elevated permission.
	require.NoError(t, err)
	require.Equal(t, "life", result[0].Slug)
	require.Empty(t, tags[0].Slug)
	require.Zero(t, provider.calls)
	require.Zero(t, guard.allowed)
	require.Zero(t, guard.charged)
}
