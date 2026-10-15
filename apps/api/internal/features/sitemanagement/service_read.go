package sitemanagement

import (
	"context"
	"errors"
	"heyblog-api/internal/features/auth"
	"slices"
)

func ownershipState(userID, ownerID, ownerAddress, currentAddress, visibility string) string {
	if ownerID == "" {
		return "none"
	}
	if ownerID != userID {
		return "other"
	}
	if ownerAddress != currentAddress || visibility == "REMOVED" {
		return "stale"
	}
	return "mine"
}

func (s *Service) present(claim Claim, viewer string, management bool) Claim {
	claim.OwnershipState = ownershipState(viewer, claim.OwnerUserID, claim.OwnerAddress, claim.CurrentAddress, claim.Visibility)
	claim.CanManage = claim.OwnershipState == "mine"
	applicable := claim.UserID == viewer && claim.Status == "PENDING" && claim.Visibility != "REMOVED" && claim.OwnershipState != "other" && (claim.OwnerUserID == viewer || claim.Address == claim.CurrentAddress)
	claim.CanRegenerate = applicable && claim.Method != Manual && claim.Method != BlogsClub
	claim.CanCheck = applicable && (claim.Method == BlogsClub || (claim.CanRegenerate && claim.ExpiresAt != nil && s.now().Before(*claim.ExpiresAt)))
	claim.CanCancel = claim.UserID == viewer && claim.Status == "PENDING"
	if !management {
		claim.Applicant = nil
	}
	return claim
}

func (s *Service) Detail(ctx context.Context, user auth.User, id string, management bool) (Claim, error) {
	if management && !canReview(user) {
		return Claim{}, failure("forbidden", "Site verification review permission is required", 403)
	}
	claim, err := s.store.Claim(ctx, id)
	if err != nil {
		return Claim{}, err
	}
	if !management && claim.UserID != user.ID {
		return Claim{}, ErrNotFound
	}
	return s.present(claim, user.ID, management), nil
}

func (s *Service) List(ctx context.Context, user auth.User, filter ClaimFilter, management bool) (ClaimList, error) {
	if management && !canReview(user) {
		return ClaimList{}, failure("forbidden", "Site verification review permission is required", 403)
	}
	if !slices.Contains([]string{"", "PENDING", "VERIFIED", "REJECTED", "CANCELLED"}, filter.Status) || !slices.Contains([]string{"", "DNS_TXT", "META", "FILE", "MANUAL", "BLOGSCLUB"}, filter.Method) {
		return ClaimList{}, failure("invalid_filter", "The claim status or method filter is invalid", 422)
	}
	filter.UserID = user.ID
	if management {
		filter.UserID = ""
	}
	limit := filter.Limit
	filter.Limit++
	items, err := s.store.List(ctx, filter)
	if err != nil {
		return ClaimList{}, err
	}
	result := ClaimList{Items: items, HasMore: len(items) > int(limit)}
	if result.HasMore {
		result.Items = result.Items[:limit]
	}
	for i, claim := range result.Items {
		result.Items[i] = s.present(claim, user.ID, management)
	}
	return result, nil
}

func (s *Service) Context(ctx context.Context, user auth.User, shortID string) (ClaimContext, error) {
	target, err := s.store.Site(ctx, shortID)
	if err != nil {
		return ClaimContext{}, err
	}
	owner, err := s.store.Owner(ctx, target.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return ClaimContext{}, err
	}
	state := ownershipState(user.ID, owner.UserID, owner.Address, target.Address, target.Visibility)
	claims, err := s.store.List(ctx, ClaimFilter{UserID: user.ID, SiteID: target.ID, Status: "PENDING"})
	if err != nil {
		return ClaimContext{}, err
	}
	for i, claim := range claims {
		claims[i] = s.present(claim, user.ID, false)
	}
	return ClaimContext{Site: target, OwnershipState: state, CanClaim: target.Visibility != "REMOVED" && (state == "none" || state == "stale"), CanManage: state == "mine", CanVerifyAddress: owner.UserID == user.ID && target.Visibility != "REMOVED", PendingClaims: claims}, nil
}

func (s *Service) Ownership(ctx context.Context, user auth.User, shortID string) (OwnershipDetail, error) {
	if !canReview(user) {
		return OwnershipDetail{}, failure("forbidden", "Site verification review permission is required", 403)
	}
	target, err := s.store.Site(ctx, shortID)
	if err != nil {
		return OwnershipDetail{}, err
	}
	result := OwnershipDetail{Site: target}
	owner, err := s.store.Owner(ctx, target.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return OwnershipDetail{}, err
	}
	if err == nil {
		owner.IsCurrent = owner.Address == target.Address && target.Visibility != "REMOVED"
		result.Current = &owner
		result.CanRevoke = owner.UserID != user.ID
	}
	result.History, err = s.store.History(ctx, target.ID)
	return result, err
}
