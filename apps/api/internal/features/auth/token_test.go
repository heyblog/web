package auth

import (
	"strings"
	"testing"
	"time"
)

func TestTokenRoundTripWhenClaimsAreValid(t *testing.T) {
	// Given
	now := time.Unix(1_700_000_000, 0)
	claims := tokenClaims{Subject: "user-id", SessionID: "session-id", AuthVersion: 3, TokenType: "access", IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Hour).Unix()}

	// When
	token, err := signToken(claims, "test-secret")
	if err != nil {
		t.Fatalf("signToken() error = %v", err)
	}
	got, err := verifyToken(token, "test-secret", "access", now)

	// Then
	if err != nil || got != claims {
		t.Fatalf("verifyToken() = (%#v, %v), want (%#v, nil)", got, err, claims)
	}
}

func TestTokenRejectsTamperingAndWrongType(t *testing.T) {
	// Given
	now := time.Now()
	token, err := signToken(tokenClaims{Subject: "user-id", SessionID: "session-id", AuthVersion: 1, TokenType: "refresh", IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Hour).Unix()}, "test-secret")
	if err != nil {
		t.Fatalf("signToken() error = %v", err)
	}

	// When / Then
	if _, err := verifyToken(token+"x", "test-secret", "refresh", now); err == nil {
		t.Fatal("tampered token was accepted")
	}
	if _, err := verifyToken(token, "test-secret", "access", now); err == nil {
		t.Fatal("wrong token type was accepted")
	}
}

func TestRandomDigitsProducesSixDigits(t *testing.T) {
	// Given / When
	code, err := randomDigits(6)

	// Then
	if err != nil || len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		t.Fatalf("randomDigits() = (%q, %v), want six digits", code, err)
	}
}
