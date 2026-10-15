package sitemanagement

import (
	"context"

	"heyblog-api/internal/domain/site"
	"heyblog-api/internal/features/auth"
)

func (s *Service) ValidateOAuthClaim(ctx context.Context, user auth.User, id string) error {
	_, err := s.oauthClaim(ctx, user, id)
	if err != nil {
		return routeError(err)
	}
	return nil
}

func (s *Service) oauthClaim(ctx context.Context, user auth.User, id string) (Claim, error) {
	claim, err := s.applicant(ctx, user, id)
	if err != nil {
		return Claim{}, err
	}
	if claim.Method != BlogsClub || claim.Status != "PENDING" {
		return Claim{}, failure("claim_finished", "This application cannot accept BlogsClub authorization", 409)
	}
	if !s.present(claim, user.ID, false).CanCheck {
		return Claim{}, failure("claim_unavailable", "This verification application is unavailable", 409)
	}
	return claim, nil
}

func (s *Service) CompleteOAuthClaim(ctx context.Context, user auth.User, id string, proof auth.OAuthSiteProof) error {
	claim, err := s.oauthClaim(ctx, user, id)
	if err != nil {
		return routeError(err)
	}
	if !proof.Verified {
		return routeError(failure("blogsclub_site_unverified", "The BlogsClub site is not verified", 409))
	}
	address, err := site.NormalizeAddress(proof.BlogURL)
	if err != nil || address.CanonicalURL() != claim.Address {
		return routeError(failure("blogsclub_site_mismatch", "The BlogsClub site address does not match this application", 409))
	}
	_, err = s.store.Decide(ctx, Decision{Claim: claim, Status: "VERIFIED", Now: s.now(), OAuthActor: &user, OAuthSubject: proof.Subject})
	if err != nil {
		return routeError(err)
	}
	return nil
}
