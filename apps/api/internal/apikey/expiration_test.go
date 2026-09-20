package apikey

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCreateClientExpirationBoundaries(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 20, 10, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name      string
		audience  Audience
		expires   *time.Time
		permanent bool
		valid     bool
	}{
		{name: "internal ninety days", audience: AudienceInternal, expires: timePointer(now.Add(maximumInternalKeyLife)), valid: true},
		{name: "internal permanent", audience: AudienceInternal, permanent: true},
		{name: "internal missing expiry", audience: AudienceInternal},
		{name: "internal expired", audience: AudienceInternal, expires: &now},
		{name: "external expired", audience: AudienceExternal, expires: &now},
		{name: "external missing expiry", audience: AudienceExternal},
		{name: "external conflicting expiry", audience: AudienceExternal, expires: timePointer(now.Add(time.Hour)), permanent: true},
		{name: "external one year", audience: AudienceExternal, expires: timePointer(now.Add(365 * 24 * time.Hour)), valid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			service := NewService(&fakeStore{}, func() time.Time { return now })
			// When
			_, err := service.CreateClient(context.Background(), CreateClientRequest{Name: "consumer", Audience: test.audience, Scopes: []Scope{ScopeExampleCall}, ExpiresAt: test.expires, NeverExpires: test.permanent})
			// Then
			if test.valid && err != nil {
				t.Fatal(err)
			}
			if !test.valid && !errors.Is(err, ErrInvalidExpiration) {
				t.Fatalf("expiration error %v", err)
			}
		})
	}
}

func TestCreateClientRejectsExternalDataImport(t *testing.T) {
	t.Parallel()
	// Given
	service := NewService(&fakeStore{}, time.Now)
	// When
	_, err := service.CreateClient(context.Background(), CreateClientRequest{Name: "consumer", Audience: AudienceExternal, Scopes: []Scope{ScopeDataImportWrite}, NeverExpires: true})
	// Then
	if !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("scope error %v", err)
	}
}
