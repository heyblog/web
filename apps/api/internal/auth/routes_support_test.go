package auth

import (
	"errors"
	"fmt"
	"net/http"
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
