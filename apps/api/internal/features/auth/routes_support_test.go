package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"heyblog-api/internal/infrastructure/mail"
	"heyblog-api/internal/platform/apperror"
)

func TestAuthErrorStatusMapping(t *testing.T) {
	if got := statusKind(http.StatusUnprocessableEntity); got != "validation" {
		t.Fatalf("422 kind = %q", got)
	}
	if got := statusKind(http.StatusBadGateway); got != "unavailable" {
		t.Fatalf("502 kind = %q", got)
	}
	mapped := mapError(sessionUnavailable(context.DeadlineExceeded))
	var unavailable *apperror.Error
	if !errors.As(mapped, &unavailable) || unavailable.Kind() != apperror.KindUnavailable || !errors.Is(mapped, context.DeadlineExceeded) {
		t.Fatalf("session timeout lost service status or cause: %v", mapped)
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

func TestOAuthCookieNamesIsolateProvidersAndAttempts(t *testing.T) {
	// Given concurrent authorization attempts for two providers.
	names := []string{oauthStateCookieName(ProviderGitHub, "first"), oauthStateCookieName(ProviderGitHub, "second"), oauthStateCookieName(ProviderBlogsClub, "first")}
	// When cookies are parsed from the same browser request.
	request := httptest.NewRequest(http.MethodGet, "/auth/blogsclub/callback", nil)
	for _, name := range names {
		request.AddCookie(authCookie(Config{}, name, name, "/", 600, time.Now().Add(10*time.Minute)))
	}
	// Then all attempts remain independently readable.
	for _, name := range names {
		cookie, err := request.Cookie(name)
		if err != nil || cookie.Value != name {
			t.Fatalf("cookie %s: %v", name, err)
		}
	}
}
