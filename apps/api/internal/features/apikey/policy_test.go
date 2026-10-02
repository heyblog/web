package apikey

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestClientChangesRejectDuplicateAndRetiredScopes(t *testing.T) {
	t.Parallel()
	for _, scopes := range [][]Scope{{ScopeExampleCall, ScopeExampleCall}, {"sites.read"}} {
		t.Run(strings.Join(scopeStrings(scopes), ","), func(t *testing.T) {
			// Given
			service := NewService(&fakeStore{}, time.Now)
			// When
			_, createErr := service.CreateClient(context.Background(), CreateClientRequest{
				Name: "consumer", Audience: AudienceExternal, Scopes: scopes, NeverExpires: true,
			})
			_, updateErr := service.UpdateClient(context.Background(), UpdateClientRecord{Name: "consumer", Scopes: scopes})
			// Then
			if !errors.Is(createErr, ErrInvalidScope) || !errors.Is(updateErr, ErrInvalidScope) {
				t.Fatalf("scope validation: create = %v, update = %v", createErr, updateErr)
			}
		})
	}
}

func TestParseTokenPreservesUnderscoresInPublicID(t *testing.T) {
	t.Parallel()
	// Given
	publicID := "abc_def_ghij"
	secret := strings.Repeat("s", 43)
	// When
	gotID, gotSecret, valid := parseToken("hbk_" + publicID + "_" + secret)
	// Then
	if !valid || gotID != publicID || gotSecret != secret {
		t.Fatal("issued token did not round trip")
	}
}

func scopeStrings(scopes []Scope) []string {
	result := make([]string, len(scopes))
	for index, scope := range scopes {
		result[index] = string(scope)
	}
	return result
}
