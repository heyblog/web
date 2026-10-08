package sitemanagement

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"heyblog-api/internal/domain/site"
	"heyblog-api/internal/features/auth"
)

type Service struct {
	store    Store
	auth     Authentication
	verifier Verifier
	now      func() time.Time
}

func NewService(store Store, authentication Authentication, verifier Verifier) *Service {
	return &Service{store: store, auth: authentication, verifier: verifier, now: time.Now}
}
func (s *Service) Current(ctx context.Context, request *http.Request) (auth.User, error) {
	return s.auth.Current(ctx, request)
}
func (s *Service) Reviewer(ctx context.Context, request *http.Request) (auth.User, error) {
	user, err := s.Current(ctx, request)
	if err != nil {
		return auth.User{}, err
	}
	if !canReview(user) {
		return auth.User{}, failure("forbidden", "Site verification review permission is required", 403)
	}
	return user, nil
}
func (s *Service) CurrentOwner(ctx context.Context, user auth.User, shortID string) (Ownership, error) {
	target, err := s.store.Site(ctx, shortID)
	if err != nil {
		return Ownership{}, err
	}
	owner, err := s.store.Owner(ctx, target.ID)
	if errors.Is(err, ErrNotFound) {
		return Ownership{}, failure("site_owner_required", "Verified site ownership is required", 403)
	}
	if err != nil {
		return Ownership{}, err
	}
	if owner.UserID != user.ID || owner.Address != target.Address || target.Visibility == "REMOVED" {
		return Ownership{}, failure("site_owner_required", "Verified site ownership is required", 403)
	}
	return owner, nil
}
func (s *Service) Create(ctx context.Context, user auth.User, input SiteClaimCreateInput) (CreateResult, error) {
	target, err := s.store.Site(ctx, input.ShortID)
	if err != nil {
		return CreateResult{}, err
	}
	if target.Visibility == "REMOVED" {
		return CreateResult{}, failure("site_removed", "Removed sites cannot be verified", 409)
	}
	address := target.Address
	if input.Address != "" {
		parsed, err := site.NormalizeAddress(input.Address)
		if err != nil {
			return CreateResult{}, failure("invalid_address", "The site address is invalid", 422)
		}
		address = parsed.CanonicalURL()
	}
	switch input.Method {
	case DNS, Meta, File, Manual:
	default:
		return CreateResult{}, failure("invalid_method", "The verification method is invalid", 422)
	}
	claim := Claim{ShortID: target.ShortID, SiteID: target.ID, Address: address, UserID: user.ID, Method: input.Method, Status: "PENDING", CreatedAt: s.now()}
	var token string
	if input.Method == Manual {
		evidence := strings.TrimSpace(input.Evidence)
		if evidence == "" || len(evidence) > 4000 {
			return CreateResult{}, failure("invalid_evidence", "Ownership evidence is required", 422)
		}
		claim.Evidence = &evidence
		if input.EvidenceURL != "" {
			u, urlErr := url.Parse(input.EvidenceURL)
			if urlErr != nil || u.Hostname() == "" || u.User != nil || len(input.EvidenceURL) > 2048 {
				return CreateResult{}, failure("invalid_evidence_url", "The evidence URL is invalid", 422)
			}
			if _, err := site.NormalizeAddress(u.Scheme + "://" + u.Host); err != nil {
				return CreateResult{}, failure("invalid_evidence_url", "The evidence URL is invalid", 422)
			}
			value := input.EvidenceURL
			claim.EvidenceURL = &value
		}
	} else {
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			return CreateResult{}, fmt.Errorf("generate site challenge: %w", err)
		}
		token = base64.RawURLEncoding.EncodeToString(bytes)
		claim.TokenHash = digest(token)
		expiry := s.now().Add(24 * time.Hour)
		claim.ExpiresAt = &expiry
	}
	created, err := s.store.Create(ctx, StoredClaim{Claim: claim, UserID: user.ID})
	if err != nil {
		return CreateResult{}, err
	}
	result := CreateResult{Claim: created, Token: token}
	if token != "" {
		parsed, err := url.Parse(address)
		if err != nil {
			return CreateResult{}, err
		}
		result.Instructions = &Instructions{DNSName: "_heyblog-verification." + parsed.Hostname(), DNSValue: proofPrefix + token, Meta: `<meta name="heyblog-site-verification" content="` + token + `">`, FileURL: strings.TrimRight(address, "/") + "/.well-known/heyblog-site-verification.txt", FileContent: token}
	}
	return result, nil
}
func (s *Service) Check(ctx context.Context, user auth.User, id string) (Claim, error) {
	claim, err := s.applicant(ctx, user, id)
	if err != nil {
		return Claim{}, err
	}
	if claim.Status != "PENDING" {
		return Claim{}, failure("claim_finished", "This verification application is already finished", 409)
	}
	if claim.Method == Manual {
		return Claim{}, failure("manual_review_required", "This verification application requires human review", 409)
	}
	if claim.ExpiresAt == nil || !s.now().Before(*claim.ExpiresAt) {
		return Claim{}, failure("claim_expired", "This verification challenge has expired", 409)
	}
	if err := s.verifier.Verify(ctx, claim); err != nil {
		if ctx.Err() != nil {
			return Claim{}, ctx.Err()
		}
		return Claim{}, failure("proof_not_found", "Verification failed. Check the published proof and try again", 422)
	}
	return s.store.Decide(ctx, Decision{Claim: claim, Status: "VERIFIED", Now: s.now()})
}
func (s *Service) applicant(ctx context.Context, user auth.User, id string) (Claim, error) {
	claim, err := s.store.Claim(ctx, id)
	if err != nil {
		return Claim{}, err
	}
	if claim.UserID != user.ID {
		return Claim{}, ErrNotFound
	}
	return claim, nil
}
func (s *Service) Cancel(ctx context.Context, user auth.User, id string) (Claim, error) {
	claim, err := s.applicant(ctx, user, id)
	if err != nil {
		return Claim{}, err
	}
	return s.store.Decide(ctx, Decision{Claim: claim, Status: "CANCELLED", Now: s.now()})
}
func (s *Service) Review(ctx context.Context, user auth.User, id string, input SiteClaimReviewInput) (Claim, error) {
	if !canReview(user) {
		return Claim{}, failure("forbidden", "Site verification review permission is required", 403)
	}
	claim, err := s.store.Claim(ctx, id)
	if err != nil {
		return Claim{}, err
	}
	if claim.Method != Manual {
		return Claim{}, failure("automatic_claim", "Automatic challenges cannot be manually approved", 409)
	}
	if claim.UserID == user.ID {
		return Claim{}, failure("self_review", "You cannot review your own verification application", 403)
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" || len(reason) > 2000 {
		return Claim{}, failure("invalid_reason", "A review reason is required", 422)
	}
	status := "REJECTED"
	if input.Approve {
		status = "VERIFIED"
	}
	return s.store.Decide(ctx, Decision{Claim: claim, Status: status, ActorID: user.ID, Reason: reason, Now: s.now()})
}
func (s *Service) ChangeOwner(ctx context.Context, user auth.User, shortID string, input OwnershipInput) error {
	if !canReview(user) {
		return failure("forbidden", "Site verification review permission is required", 403)
	}
	if strings.TrimSpace(input.Reason) == "" || len(input.Reason) > 2000 || strings.TrimSpace(input.Evidence) == "" || len(input.Evidence) > 4000 {
		return failure("invalid_evidence", "A reason and ownership evidence are required", 422)
	}
	if input.UserID == user.ID {
		return failure("self_assignment", "You cannot assign a site to yourself", 403)
	}
	target, err := s.store.Site(ctx, shortID)
	if err != nil {
		return err
	}
	return s.store.ChangeOwner(ctx, target, input, user.ID)
}
func canReview(user auth.User) bool {
	return user.Role == auth.RoleSysAdmin || user.Role == auth.RoleAdmin && slices.Contains(user.Permissions, auth.PermissionSiteAuditReview)
}
