package proof

import (
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProof_when_RequestChangesOrKeyIsRevoked(t *testing.T) {
	// Given a signed request with a five-minute validity window.
	key, err := GenerateKey()
	require.NoError(t, err)
	now := time.Unix(1700000000, 0)
	claims := Claims{Method: "GET", URL: "https://example.com/a", Platform: Cloudflare, IssuedAt: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix(), ID: "unique-request"}
	token, err := Sign(key, "key-1", claims)
	require.NoError(t, err)
	cases := []struct {
		name         string
		token        string
		verification Verification
		expected     error
	}{
		{"valid", token, Verification{Key: &key.PublicKey, KeyID: "key-1", Now: now, Expected: &claims}, nil},
		{"expired", token, Verification{Key: &key.PublicKey, KeyID: "key-1", Now: now.Add(5 * time.Minute)}, ErrExpired},
		{"revoked", token, Verification{Key: &key.PublicKey, KeyID: "key-1", Now: now, Revoked: true}, ErrRevoked},
		{"different target", token, Verification{Key: &key.PublicKey, KeyID: "key-1", Now: now, Expected: &Claims{Method: "GET", URL: "https://example.com/b", Platform: Cloudflare}}, ErrInvalid},
		{"different platform", token, Verification{Key: &key.PublicKey, KeyID: "key-1", Now: now, Expected: &Claims{Method: "GET", URL: claims.URL, Platform: EdgeOne}}, ErrInvalid},
		{"tampered", token[:strings.LastIndex(token, ".")+1] + strings.Repeat("A", 86), Verification{Key: &key.PublicKey, KeyID: "key-1", Now: now}, ErrInvalid},
		{"missing", "", Verification{}, ErrMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// When the request is verified under the current key policy.
			_, err := Verify(tc.token, tc.verification)
			// Then only the unmodified, unexpired authorized request is accepted.
			require.ErrorIs(t, err, tc.expected)
		})
	}
}
func TestEncryption_when_CiphertextIsMovedToAnotherKey(t *testing.T) {
	// Given a key encrypted under the backend's independent master secret.
	key, err := GenerateKey()
	require.NoError(t, err)
	master := make([]byte, 32)
	_, err = rand.Read(master)
	require.NoError(t, err)
	ciphertext, err := Encrypt(master, "one", key)
	require.NoError(t, err)
	// When authenticated key identity is changed.
	_, err = Decrypt(master, "two", ciphertext)
	// Then swapping database ciphertext is rejected.
	require.ErrorIs(t, err, ErrInvalid)
}
func TestEncryption_when_RestoredWithCorrectMaster(t *testing.T) {
	// Given encrypted private material.
	key, err := GenerateKey()
	require.NoError(t, err)
	master := make([]byte, 32)
	_, err = rand.Read(master)
	require.NoError(t, err)
	ciphertext, err := Encrypt(master, "one", key)
	require.NoError(t, err)
	// When the backend decrypts it with the separate master key.
	restored, err := Decrypt(master, "one", ciphertext)
	// Then the same signing identity is recovered.
	require.NoError(t, err)
	require.True(t, key.Equal(restored))
}

func TestSign_when_RequestWouldProduceUnverifiableToken(t *testing.T) {
	// Given a legal redirect URL shorter than the Edge's 8192-byte URL limit.
	key, err := GenerateKey()
	require.NoError(t, err)
	claims := Claims{Method: "GET", URL: "https://example.com/" + strings.Repeat("a", 7000), Platform: Cloudflare, IssuedAt: 1700000000, ExpiresAt: 1700000300, ID: "unique-request"}
	// When signing would exceed the verifier and request-header token limit.
	token, err := Sign(key, "key-1", claims)
	// Then signing fails before the caller can reserve dispatch quota.
	require.ErrorIs(t, err, ErrInvalid)
	require.Empty(t, token)
}
