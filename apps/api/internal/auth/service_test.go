package auth

import "testing"

func TestValidEmailAcceptsNormalizedMailboxOnly(t *testing.T) {
	if !validEmail("reader@example.test") {
		t.Fatal("normalized mailbox was rejected")
	}
	for _, value := range []string{"", "not-an-email", "Reader <reader@example.test>", "用户@example.test"} {
		if validEmail(value) {
			t.Fatalf("validEmail(%q) = true, want false", value)
		}
	}
}
