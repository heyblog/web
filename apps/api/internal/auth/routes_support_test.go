package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/mail"
)

func TestAuthErrorStatusMapping(t *testing.T) {
	if got := statusKind(http.StatusUnprocessableEntity); got != "validation" {
		t.Fatalf("422 kind = %q", got)
	}
	if got := statusKind(http.StatusBadGateway); got != "unavailable" {
		t.Fatalf("502 kind = %q", got)
	}
}

func TestMapErrorClassifiesMailDeliveryFailureAsUnavailable(t *testing.T) {
	got := mapError(fmt.Errorf("send verification code: %w", mail.ErrDeliveryUnavailable))
	var applicationError *apperror.Error
	if !errors.As(got, &applicationError) {
		t.Fatalf("mapError() = %T, want application error", got)
	}
	if applicationError.Kind() != apperror.KindUnavailable || applicationError.Code() != "mail_unavailable" {
		t.Fatalf("mapped error = (%q, %q), want unavailable/mail_unavailable", applicationError.Kind(), applicationError.Code())
	}
	if !errors.Is(got, mail.ErrDeliveryUnavailable) {
		t.Fatalf("mapped error = %v, want delivery cause preserved", got)
	}
}

func TestAuthCookieUsesConfiguredLifetimeAndDomain(t *testing.T) {
	expires := time.Now().Add(time.Hour)
	cookie := authCookie(Config{WebBaseURL: "https://web.example.test", CookieDomain: ".example.test"}, "token", "value", "/", 3600, expires)
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Domain != ".example.test" || cookie.MaxAge != 3600 || !cookie.Expires.Equal(expires) {
		t.Fatalf("authCookie() = %#v", cookie)
	}
}

func TestGithubStateCookieNameIsScopedToState(t *testing.T) {
	first := githubStateCookieName("first-state")
	second := githubStateCookieName("second-state")

	if first == second {
		t.Fatalf("state cookie names collide: %q", first)
	}
	if first != "heyblog_github_state_first-state" || second != "heyblog_github_state_second-state" {
		t.Fatalf("state cookie names = (%q, %q), want state-scoped names", first, second)
	}
}

func TestReadGithubStateCookiePrefersMatchingStateCookie(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=first-state", nil)
	request.Header.Set("Cookie", githubStateCookieName("first-state")+"=first-state; "+githubStateCookieName("second-state")+"=second-state")

	value, legacy := readGithubStateCookie(request, "first-state")

	if value != "first-state" || legacy {
		t.Fatalf("readGithubStateCookie() = (%q, %t), want matching state cookie", value, legacy)
	}
}

func TestReadGithubStateCookieFallsBackToLegacyCookie(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=state", nil)
	request.Header.Set("Cookie", legacyGithubStateCookieName+"=state")

	value, legacy := readGithubStateCookie(request, "state")

	if value != "state" || !legacy {
		t.Fatalf("readGithubStateCookie() = (%q, %t), want legacy cookie", value, legacy)
	}
}
