package auth

import (
	"testing"
	"time"
)

func TestNewServiceAllowsSlowGitHubResponses(t *testing.T) {
	service := NewService(Dependencies{})
	if service.httpClient.Timeout != 15*time.Second {
		t.Fatalf("GitHub HTTP timeout = %s, want 15s", service.httpClient.Timeout)
	}
}

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
