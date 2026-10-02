package apikey

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAuthenticateRejectsInvalidCredentialStates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 19, 10, 0, 0, 0, time.UTC)
	secret := "0123456789012345678901234567890123456789012"
	validDigest := secretDigest(secret)
	wrongDigest := secretDigest("wrong-secret")
	past := now.Add(-time.Second)
	revokedAt := now.Add(-time.Minute)
	disabledAt := now.Add(-time.Hour)

	tests := map[string]StoredCredential{
		"wrong digest":    credentialForTest(wrongDigest[:], nil, nil, nil),
		"expired":         credentialForTest(validDigest[:], &past, nil, nil),
		"revoked":         credentialForTest(validDigest[:], nil, &revokedAt, nil),
		"disabled client": credentialForTest(validDigest[:], nil, nil, &disabledAt),
	}
	for name, stored := range tests {
		stored := stored
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			service := NewService(&fakeStore{authenticateResult: stored}, func() time.Time { return now })
			_, err := service.Authenticate(context.Background(), "hbk_public123456_"+secret, AccessPolicy{Audiences: []Audience{AudienceExternal}, Scope: ScopeExampleCall})
			if !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("Authenticate() error = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestAuthenticateRejectsMissingScope(t *testing.T) {
	t.Parallel()

	secret := "0123456789012345678901234567890123456789012"
	digest := secretDigest(secret)
	stored := credentialForTest(digest[:], nil, nil, nil)
	stored.Scopes = nil
	service := NewService(&fakeStore{authenticateResult: stored}, time.Now)

	_, err := service.Authenticate(context.Background(), "hbk_public123456_"+secret, AccessPolicy{Audiences: []Audience{AudienceExternal}, Scope: ScopeExampleCall})
	if !errors.Is(err, ErrInsufficientScope) {
		t.Fatalf("Authenticate() error = %v, want ErrInsufficientScope", err)
	}
}

func credentialForTest(hash []byte, expiresAt, revokedAt, disabledAt *time.Time) StoredCredential {
	return StoredCredential{
		Principal: Principal{
			ClientID: "00000000-0000-0000-0000-000000000002",
			KeyID:    "00000000-0000-0000-0000-000000000003",
			Audience: AudienceExternal,
			Scopes:   []Scope{ScopeExampleCall},
		},
		SecretHash:       hash,
		ExpiresAt:        expiresAt,
		RevokedAt:        revokedAt,
		ClientDisabledAt: disabledAt,
	}
}
