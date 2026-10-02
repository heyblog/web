package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	passwordMemory      = 64 * 1024
	passwordIterations  = 3
	passwordParallelism = 4
	passwordKeyLength   = 32
	passwordSaltLength  = 16
)

func hashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	digest := argon2.IDKey([]byte(password), salt, passwordIterations, passwordMemory, passwordParallelism, passwordKeyLength)
	return fmt.Sprintf("argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", passwordMemory, passwordIterations, passwordParallelism,
		base64.RawURLEncoding.EncodeToString(salt), base64.RawURLEncoding.EncodeToString(digest)), nil
}

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" || parts[1] != "v=19" {
		return false
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	saltEncoded, digestEncoded := parts[3], parts[4]
	salt, err := base64.RawURLEncoding.DecodeString(saltEncoded)
	if err != nil || len(salt) != passwordSaltLength || memory != passwordMemory ||
		iterations != passwordIterations || parallelism != passwordParallelism {
		return false
	}
	expected, err := base64.RawURLEncoding.DecodeString(digestEncoded)
	if err != nil || len(expected) != passwordKeyLength {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, passwordKeyLength)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func validPassword(password string) bool { return len(password) >= 8 && len(password) <= 128 }
