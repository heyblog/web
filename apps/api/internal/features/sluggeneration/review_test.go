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

func TestKnownMultilingualLabelReusesSlugWithoutProvider(t *testing.T) {
	// Given a confirmed mixed-language label in the dictionary.
	service, store, provider, guard := testService()
	service.store = reviewLookupStore{memoryStore: store, slugs: map[string]string{"algorithm 算法": "algorithm"}}
	// When generation is requested for that exact confirmed label.
	result, err := service.Generate(context.Background(), Identity{UserID: "user"}, SlugGenerationInput{Name: "Algorithm 算法"})
	// Then the shared slug is reused without paid generation.
	require.NoError(t, err)
	require.Equal(t, "algorithm", result.Slug)
	require.Equal(t, "existing", result.Source)
	require.Equal(t, "ready", result.State)
	require.Zero(t, provider.calls)
	require.Zero(t, guard.charged)
}

func TestReviewCollisionRequiresConfirmationAndDoesNotChangeInput(t *testing.T) {
	// Given an unconfirmed translated label whose candidate belongs to another concept.
	service, store, provider, _ := testService()
	store.occupied["programming"] = "owner"
	tags := []siteaudit.TagSnapshot{{SuggestedName: "programming 编程", Role: "TERTIARY", Level: 3}}
	// When approval requests automatic generation.
	_, err := service.ReviewedTags(context.Background(), auth.User{ID: "reviewer", Role: auth.RoleSysAdmin}, "ip", tags)
	// Then the caller receives a recoverable conflict and the pending proposal stays intact.
	require.Error(t, err)
	require.Contains(t, err.Error(), "slug_needs_confirmation")
	require.Empty(t, tags[0].Slug)
	require.Equal(t, 1, provider.calls)
}

func TestConfirmedNameOwnedByAnotherConceptNeedsMappingConfirmation(t *testing.T) {
	// Given an existing synonym and an administrator editing a different concept.
	service, store, provider, _ := testService()
	store.occupied["algorithm"] = "existing-owner"
	service.store = reviewLookupStore{memoryStore: store, slugs: map[string]string{"算法": "algorithm"}}
	// When slug generation receives the other concept ID.
	result, err := service.Generate(context.Background(), Identity{}, SlugGenerationInput{Name: "算法", TagID: "different-concept"})
	// Then the existing spelling requires an explicit mapping instead of claiming to be ready.
	require.NoError(t, err)
	require.Equal(t, "algorithm", result.Slug)
	require.Equal(t, "needs_confirmation", result.State)
	require.Zero(t, provider.calls)
}
