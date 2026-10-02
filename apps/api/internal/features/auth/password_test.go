package auth

import (
	"strings"
	"testing"
)

func TestPasswordHashVerifiesOnlyMatchingPassword(t *testing.T) {
	// Given
	hash, err := hashPassword("correct-password")
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}

	// When / Then
	if !verifyPassword("correct-password", hash) {
		t.Fatal("matching password was rejected")
	}
	if verifyPassword("wrong-password", hash) {
		t.Fatal("wrong password was accepted")
	}
}

func TestPasswordHashRejectsUntrustedParameters(t *testing.T) {
	hash, err := hashPassword("correct-password")
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}
	if verifyPassword("correct-password", strings.Replace(hash, "m=65536", "m=1048576", 1)) {
		t.Fatal("password hash with untrusted memory cost was accepted")
	}
}
