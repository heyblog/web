package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"
)

type tokenClaims struct {
	Subject     string `json:"sub"`
	SessionID   string `json:"session_id"`
	AuthVersion int32  `json:"auth_version"`
	TokenType   string `json:"token_type"`
	IssuedAt    int64  `json:"iat"`
	ExpiresAt   int64  `json:"exp"`
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func digestToken(secret, token string) string {
	digest := hmac.New(sha256.New, []byte(secret))
	_, _ = digest.Write([]byte(token))
	return fmt.Sprintf("%x", digest.Sum(nil))
}

func signToken(claims tokenClaims, secret string) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encode token claims: %w", err)
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(header + "." + encodedPayload))
	return header + "." + encodedPayload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifyToken(token, secret, expectedType string, now time.Time) (tokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return tokenClaims{}, newAuthError("invalid_token", 401, "authentication is required")
	}
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || string(header) != `{"alg":"HS256","typ":"JWT"}` {
		return tokenClaims{}, newAuthError("invalid_token", 401, "authentication is required")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	received, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(mac.Sum(nil), received) {
		return tokenClaims{}, newAuthError("invalid_token", 401, "authentication is required")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return tokenClaims{}, newAuthError("invalid_token", 401, "authentication is required")
	}
	var claims tokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil || claims.TokenType != expectedType || claims.Subject == "" || claims.SessionID == "" || claims.ExpiresAt <= now.Unix() {
		return tokenClaims{}, newAuthError("invalid_token", 401, "authentication is required")
	}
	return claims, nil
}

func randomDigits(length int) (string, error) {
	digits := make([]byte, length)
	for index := range digits {
		value, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("generate verification code: %w", err)
		}
		digits[index] = "0123456789"[value.Int64()]
	}
	return string(digits), nil
}
