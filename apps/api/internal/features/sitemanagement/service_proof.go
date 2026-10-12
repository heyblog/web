package sitemanagement

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/apperror"
	"net"
	"net/url"
	"strings"
	"time"
)

const VerificationTimeout = 8 * time.Second

func challengeToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate site challenge: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
func proofResult(claim Claim, token string) CreateResult {
	result := CreateResult{Claim: claim, Token: token}
	if token == "" {
		return result
	}
	parsed, err := url.Parse(claim.Address)
	if err != nil {
		return result
	}
	result.Instructions = &Instructions{DNSName: "_heyblog-verification." + parsed.Hostname(), DNSValue: proofPrefix + token, Meta: `<meta name="heyblog-site-verification" content="` + token + `">`, FileURL: strings.TrimRight(claim.Address, "/") + "/.well-known/heyblog-site-verification.txt", FileContent: token}
	return result
}
func (s *Service) Regenerate(ctx context.Context, user auth.User, id string) (CreateResult, error) {
	claim, err := s.applicant(ctx, user, id)
	if err != nil {
		return CreateResult{}, err
	}
	if claim.Status != "PENDING" || claim.Method == Manual {
		return CreateResult{}, failure("claim_finished", "This application cannot issue a new challenge", 409)
	}
	token, err := challengeToken()
	if err != nil {
		return CreateResult{}, err
	}
	result, err := s.store.Regenerate(ctx, claim, digest(token), s.now().Add(24*time.Hour))
	if err != nil {
		return CreateResult{}, err
	}
	if result.TokenHash != digest(token) {
		return CreateResult{}, failure("proof_changed", "The verification proof changed; reload the application", 409)
	}
	return proofResult(s.present(result, user.ID, false), token), nil
}
func verificationError(err error) error {
	code, detail, kind, reason := "verification_network", "The verification service could not reach the site", apperror.KindUnavailable, "network"
	var network net.Error
	var dns *net.DNSError
	switch {
	case errors.Is(err, errProof) || (errors.As(err, &dns) && dns.IsNotFound):
		code, detail, kind, reason = "proof_not_found", "The published verification proof does not match", apperror.KindValidation, "proof_mismatch"
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &network) && network.Timeout()):
		code, detail, reason = "verification_timeout", "Site verification timed out; try again", "timeout"
	}
	return apperror.Wrap(err, kind, code, detail, "site verification").WithDiagnostics([]apperror.Diagnostic{{Key: "reason", Value: reason}})
}
