package sitemanagement

import (
	"testing"

	"github.com/stretchr/testify/require"

	"heyblog-api/internal/features/auth"
)

func TestBlogsClubClaim_when_CreatingApplication(t *testing.T) {
	// Given an applicant and a registered site.
	store := &claimStore{target: Site{ID: "site", ShortID: "123456789", Address: "https://example.test/blog"}}
	service := NewService(store, nil, nil)
	// When the applicant selects BlogsClub.
	result, err := service.Create(t.Context(), auth.User{ID: "user"}, SiteClaimCreateInput{ShortID: "123456789", Method: BlogsClub})
	// Then authorization is available without a challenge or regeneration action.
	require.NoError(t, err)
	require.Empty(t, result.Token)
	require.Nil(t, result.Instructions)
	require.Nil(t, result.Claim.ExpiresAt)
	require.True(t, result.Claim.CanCheck)
	require.False(t, result.Claim.CanRegenerate)
}

func TestBlogsClubClaim_when_VerifiedProfileMatchesExactAddress(t *testing.T) {
	for _, test := range []struct {
		name, url      string
		verified, want bool
	}{
		{"matching path", "https://example.test/blog/", true, true},
		{"different path", "https://example.test/other/", true, false},
		{"different scheme", "http://example.test/blog/", true, false},
		{"unverified", "https://example.test/blog/", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given a pending OAuth application with no stored provider proof.
			store := &claimStore{claim: Claim{ID: "claim", UserID: "user", Status: "PENDING", Method: BlogsClub, Address: "https://example.test/blog", CurrentAddress: "https://example.test/blog"}}
			service := NewService(store, nil, nil)
			// When fresh server-fetched profile data completes it.
			err := service.CompleteOAuthClaim(t.Context(), auth.User{ID: "user"}, "claim", auth.OAuthSiteProof{Subject: "42", BlogURL: test.url, Verified: test.verified})
			// Then only a verified complete address match can decide ownership.
			if test.want {
				require.NoError(t, err)
				require.Equal(t, 1, store.decisions)
			} else {
				require.Error(t, err)
				require.Zero(t, store.decisions)
				require.Equal(t, "PENDING", store.claim.Status)
			}
		})
	}
}

func TestBlogsClubClaim_when_CheckingWithoutNewAuthorization(t *testing.T) {
	// Given a pending BlogsClub claim.
	store := &claimStore{claim: Claim{ID: "claim", UserID: "user", Method: BlogsClub, Status: "PENDING"}}
	verifier := &proofVerifier{}
	service := NewService(store, nil, verifier)
	// When the ordinary challenge endpoint is called.
	_, err := service.Check(t.Context(), auth.User{ID: "user"}, "claim")
	// Then historical profiles cannot authorize a site.
	require.Error(t, err)
	require.Zero(t, store.decisions)
	require.Zero(t, verifier.calls)
}
