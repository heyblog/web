// Package proof implements request-bound ES256 proofs independently of key storage.
package proof

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"

	"heyblog.local/edgerequest"
)

type Platform = edgerequest.Provider

const maxTokenBytes = 8192

const (
	Cloudflare Platform = "CF"
	EdgeOne    Platform = "EO"
)

var (
	ErrInvalid = errors.New("invalid request proof")
	ErrExpired = errors.New("request proof expired")
	ErrRevoked = errors.New("request proof key revoked")
	ErrMissing = errors.New("request proof missing")
)

type Claims struct {
	Method    string   `json:"method"`
	URL       string   `json:"url"`
	Platform  Platform `json:"platform"`
	IssuedAt  int64    `json:"iat"`
	ExpiresAt int64    `json:"exp"`
	ID        string   `json:"jti"`
}
type Header struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}
type JWK struct {
	KeyType   string `json:"kty"`
	Curve     string `json:"crv"`
	X         string `json:"x"`
	Y         string `json:"y"`
	KeyID     string `json:"kid"`
	Algorithm string `json:"alg"`
	Use       string `json:"use"`
}
type Verification struct {
	Key     *ecdsa.PublicKey
	KeyID   string
	Revoked bool
	Now     time.Time
	// Expected binds a proof to an observed request. Nil only for public inspection.
	Expected *Claims
}

func GenerateKey() (*ecdsa.PrivateKey, error) { return ecdsa.GenerateKey(elliptic.P256(), rand.Reader) }
func PublicKey(key *ecdsa.PublicKey, keyID string) (JWK, error) {
	if key == nil || key.Curve != elliptic.P256() || keyID == "" {
		return JWK{}, ErrInvalid
	}
	encoded, err := key.Bytes()
	if err != nil {
		return JWK{}, err
	}
	return JWK{KeyType: "EC", Curve: "P-256", X: base64.RawURLEncoding.EncodeToString(encoded[1:33]), Y: base64.RawURLEncoding.EncodeToString(encoded[33:]), KeyID: keyID, Algorithm: "ES256", Use: "sig"}, nil
}
func Sign(key *ecdsa.PrivateKey, keyID string, claims Claims) (string, error) {
	if key == nil || key.Curve != elliptic.P256() || keyID == "" || !validClaims(claims) {
		return "", ErrInvalid
	}
	header, err := json.Marshal(Header{Algorithm: "ES256", Type: "JWT", KeyID: keyID})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	// ES256 contributes one separator and 86 unpadded base64url characters.
	// Reject before signing so callers cannot authorize unusable proofs.
	if len(input)+1+86 > maxTokenBytes {
		return "", ErrInvalid
	}
	hash := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, key, hash[:])
	if err != nil {
		return "", err
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return input + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}
func Inspect(token string) (Header, Claims, error) {
	var header Header
	var claims Claims
	if token == "" {
		return header, claims, ErrMissing
	}
	if len(token) > maxTokenBytes {
		return header, claims, ErrInvalid
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return header, claims, ErrInvalid
	}
	h, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return header, claims, ErrInvalid
	}
	p, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return header, claims, ErrInvalid
	}
	if json.Unmarshal(h, &header) != nil || json.Unmarshal(p, &claims) != nil || header.Algorithm != "ES256" || header.Type != "JWT" || header.KeyID == "" || !validClaims(claims) {
		return header, claims, ErrInvalid
	}
	return header, claims, nil
}
func Verify(token string, verification Verification) (Claims, error) {
	header, claims, err := Inspect(token)
	if err != nil {
		return Claims{}, err
	}
	key := verification.Key
	if key == nil || key.Curve != elliptic.P256() || header.KeyID != verification.KeyID {
		return Claims{}, ErrInvalid
	}
	parts := strings.Split(token, ".")
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != 64 {
		return Claims{}, ErrInvalid
	}
	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(key, hash[:], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:])) {
		return Claims{}, ErrInvalid
	}
	if expected := verification.Expected; expected != nil && (claims.Method != expected.Method || claims.URL != expected.URL || claims.Platform != expected.Platform || (expected.ID != "" && claims.ID != expected.ID)) {
		return Claims{}, ErrInvalid
	}
	if verification.Revoked {
		return claims, ErrRevoked
	}
	if claims.IssuedAt > verification.Now.Unix() {
		return claims, ErrInvalid
	}
	if claims.ExpiresAt <= verification.Now.Unix() {
		return claims, ErrExpired
	}
	return claims, nil
}
func validClaims(c Claims) bool {
	return (c.Method == "GET" || c.Method == "HEAD") && (c.Platform == Cloudflare || c.Platform == EdgeOne) && c.URL != "" && c.ID != "" && c.IssuedAt > 0 && c.ExpiresAt-c.IssuedAt >= 60 && c.ExpiresAt-c.IssuedAt <= 300
}
