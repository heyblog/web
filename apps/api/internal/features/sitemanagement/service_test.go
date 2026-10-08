package sitemanagement

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"heyblog-api/internal/features/auth"
)

type claimStore struct {
	Store
	claim     Claim
	owner     Ownership
	target    Site
	decisions int
	creates   []StoredClaim
}

func (s *claimStore) Claim(context.Context, string) (Claim, error)     { return s.claim, nil }
func (s *claimStore) Site(context.Context, string) (Site, error)       { return s.target, nil }
func (s *claimStore) Owner(context.Context, string) (Ownership, error) { return s.owner, nil }
func (s *claimStore) Decide(_ context.Context, d Decision) (Claim, error) {
	s.decisions++
	s.claim.Status = d.Status
	return s.claim, nil
}
func (s *claimStore) Create(_ context.Context, input StoredClaim) (Claim, error) {
	s.creates = append(s.creates, input)
	return input.Claim, nil
}

type proofVerifier struct{ calls int }

func (v *proofVerifier) Verify(context.Context, Claim) error { v.calls++; return nil }

type testAuthentication struct {
	user auth.User
	err  error
}

func (a testAuthentication) Current(context.Context, *http.Request) (auth.User, error) {
	return a.user, a.err
}
func TestCheck_when_ChallengeExpiredOrReplayed(t *testing.T) {
	for _, status := range []string{"PENDING", "VERIFIED"} {
		t.Run(status, func(t *testing.T) {
			// Given an expired or previously verified claim.
			now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			expiry := now
			store := &claimStore{claim: Claim{ID: "claim", UserID: "user", Method: DNS, Status: status, ExpiresAt: &expiry}}
			verifier := &proofVerifier{}
			service := NewService(store, nil, verifier)
			service.now = func() time.Time { return now }
			// When the applicant checks it.
			_, err := service.Check(t.Context(), auth.User{ID: "user"}, "claim")
			// Then no network work or successful decision happens.
			require.Error(t, err)
			require.Zero(t, verifier.calls)
			require.Zero(t, store.decisions)
		})
	}
}
func TestCheck_when_ApplicantDiffers(t *testing.T) {
	// Given another applicant's claim.
	service := NewService(&claimStore{claim: Claim{UserID: "owner"}}, nil, &proofVerifier{})
	// When another user tries to verify it.
	_, err := service.Check(t.Context(), auth.User{ID: "other"}, "claim")
	// Then the claim remains undisclosed.
	require.ErrorIs(t, err, ErrNotFound)
}
func TestReview_when_SelfReviewOrMissingPermission(t *testing.T) {
	for _, user := range []auth.User{{ID: "owner", Role: auth.RoleSysAdmin}, {ID: "other", Role: auth.RoleAdmin}} {
		t.Run(user.ID, func(t *testing.T) {
			// Given a manual claim and a reviewer without authority over it.
			store := &claimStore{claim: Claim{UserID: "owner", Method: Manual, Status: "PENDING"}}
			service := NewService(store, nil, nil)
			// When the application is approved.
			_, err := service.Review(t.Context(), user, "claim", SiteClaimReviewInput{Approve: true, Reason: "proof checked"})
			// Then permission and self-review rules prevent the decision.
			require.Error(t, err)
			require.Zero(t, store.decisions)
		})
	}
}
func TestCreate_when_AutomaticChallengeIssued(t *testing.T) {
	// Given a registered site.
	store := &claimStore{target: Site{ShortID: "123456789", Address: "https://example.com/blog/"}}
	service := NewService(store, nil, nil)
	// When an applicant requests file verification.
	result, err := service.Create(t.Context(), auth.User{ID: "user"}, SiteClaimCreateInput{ShortID: "123456789", Method: File})
	// Then only the digest is stored and the exact instructions are returned once.
	require.NoError(t, err)
	require.Len(t, result.Token, 43)
	require.Equal(t, digest(result.Token), store.creates[0].Claim.TokenHash)
	require.Equal(t, "https://example.com/blog/.well-known/heyblog-site-verification.txt", result.Instructions.FileURL)
	require.WithinDuration(t, store.creates[0].Claim.CreatedAt.Add(24*time.Hour), *result.Claim.ExpiresAt, time.Second)
}
func TestCurrentOwner_when_CanonicalAddressChanges(t *testing.T) {
	// Given ownership verified for a previous address.
	store := &claimStore{target: Site{ID: "site", Address: "https://new.example/"}, owner: Ownership{UserID: "owner", Address: "https://old.example/"}}
	service := NewService(store, nil, nil)
	// When the owner attempts to edit canonical data.
	_, err := service.CurrentOwner(t.Context(), auth.User{ID: "owner"}, "123456789")
	// Then the address must be verified again.
	require.Error(t, err)
}
