package sitemanagement

import (
	"context"
	"strings"

	"heyblog-api/internal/features/auth"
)

type VerificationTask struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	StatusURL string `json:"status_url"`
	Reason    string `json:"reason"`
}

type VerificationQueue interface {
	EnqueueClaim(context.Context, Claim) (VerificationTask, error)
	ClaimTask(context.Context, string, string) (*VerificationTask, error)
}

func (s *Service) SetVerificationQueue(queue VerificationQueue) { s.queue = queue }

func (s *Service) StartCheck(ctx context.Context, user auth.User, id string) (Claim, error) {
	claim, err := s.applicant(ctx, user, id)
	if err != nil {
		return Claim{}, err
	}
	if claim.Method == DNS {
		return s.Check(ctx, user, id)
	}
	if claim.Method != Meta && claim.Method != File {
		return Claim{}, failure("automatic_check_unavailable", "Use the application verification method", 409)
	}
	if claim.Status != "PENDING" || claim.ExpiresAt == nil || !s.now().Before(*claim.ExpiresAt) {
		return Claim{}, failure("claim_expired", "This verification challenge is no longer active", 409)
	}
	if s.queue == nil {
		return Claim{}, failure("verification_unavailable", "Verification is temporarily unavailable", 503)
	}
	task, err := s.queue.EnqueueClaim(ctx, claim)
	if err != nil {
		return Claim{}, err
	}
	claim = s.present(claim, user.ID, false)
	claim.Task = &task
	return claim, nil
}

func CheckHTTPProof(claim Claim, status int, body []byte) bool {
	if status != 200 {
		return false
	}
	switch claim.Method {
	case File:
		return len(body) <= 512 && matches(strings.TrimSpace(string(body)), claim.TokenHash)
	case Meta:
		return len(body) <= 1<<20 && verifyMeta(string(body), claim.TokenHash) == nil
	default:
		return false
	}
}
