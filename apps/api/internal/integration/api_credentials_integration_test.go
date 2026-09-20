//go:build integration

package integration_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"heyblog-api/internal/apikey"
	"heyblog-api/internal/exampleapi"
)

func TestAPICredentials(t *testing.T) {
	fixture := newCredentialFixture(t)
	t.Run("both audiences call every example method", func(t *testing.T) {
		for _, audience := range []apikey.Audience{apikey.AudienceInternal, apikey.AudienceExternal} {
			t.Run(string(audience), func(t *testing.T) {
				// Given a real issued key persisted in the database.
				credential := fixture.create(t, audience)
				for _, method := range strings.Split(exampleapi.AllowedMethods, ", ") {
					request := httptest.NewRequest(method, exampleapi.Path, nil)
					request.Header.Set("Authorization", "Bearer "+credential.Token)
					response := httptest.NewRecorder()
					// When
					fixture.router.ServeHTTP(response, request)
					// Then
					if response.Code != 200 && response.Code != 204 {
						t.Fatalf("%s: status %d", method, response.Code)
					}
				}
				importPolicy := apikey.AccessPolicy{Audiences: []apikey.Audience{apikey.AudienceInternal}, Scope: apikey.ScopeDataImportWrite}
				if _, err := fixture.service.Authenticate(t.Context(), credential.Token, importPolicy); !errors.Is(err, apikey.ErrInsufficientScope) {
					t.Fatalf("example-only key imported: %v", err)
				}
			})
		}
	})
	t.Run("external scopes reject data import", func(t *testing.T) {
		credential := fixture.create(t, apikey.AudienceExternal)
		_, err := fixture.service.UpdateClient(t.Context(), apikey.UpdateClientRecord{ID: credential.Client.ID, Name: credential.Client.Name, Enabled: true, Scopes: []apikey.Scope{apikey.ScopeDataImportWrite}})
		if !errors.Is(err, apikey.ErrInvalidScope) {
			t.Fatalf("external update: %v", err)
		}
	})
	t.Run("internal scopes control operations", func(t *testing.T) {
		// Given an INTERNAL client granted only data import after an update.
		credential := fixture.create(t, apikey.AudienceInternal)
		if _, err := fixture.service.UpdateClient(t.Context(), apikey.UpdateClientRecord{ID: credential.Client.ID, Name: credential.Client.Name, Enabled: true, Scopes: []apikey.Scope{apikey.ScopeDataImportWrite}}); err != nil {
			t.Fatal(err)
		}
		// When the key authenticates against each operation's policy.
		_, importErr := fixture.service.Authenticate(t.Context(), credential.Token, apikey.AccessPolicy{Audiences: []apikey.Audience{apikey.AudienceInternal}, Scope: apikey.ScopeDataImportWrite})
		request := httptest.NewRequest(http.MethodGet, exampleapi.Path, nil)
		request.Header.Set("Authorization", "Bearer "+credential.Token)
		response := httptest.NewRecorder()
		fixture.router.ServeHTTP(response, request)
		// Then scope changes take effect immediately without replacing the key.
		if importErr != nil {
			t.Fatal(importErr)
		}
		if response.Code != 403 || !strings.Contains(response.Body.String(), `"insufficient_scope"`) {
			t.Fatalf("scope removal status %d", response.Code)
		}
	})
	t.Run("database rejects retired scope", func(t *testing.T) {
		credential := fixture.create(t, apikey.AudienceExternal)
		_, err := fixture.pool.Exec(t.Context(), "INSERT INTO identity.api_client_scopes (client_id, scope) VALUES ($1, 'sites.read')", credential.Client.ID)
		if err == nil {
			t.Fatal("database accepted retired scope")
		}
	})
	t.Run("issue lifecycle via management HTTP", func(t *testing.T) { testIssueLifecycle(t, fixture) })
	t.Run("concurrent issuance creates one key", func(t *testing.T) { testConcurrentIssuance(t, fixture) })
	t.Run("expired keys allow issuance", func(t *testing.T) {
		// Given a clock beyond the existing key's expiration.
		credential := fixture.create(t, apikey.AudienceExternal)
		now := credential.Key.ExpiresAt.Add(time.Second)
		service := apikey.NewService(apikey.NewRepository(fixture.pool), func() time.Time { return now })
		// When
		issued, err := service.IssueKey(t.Context(), apikey.IssueKeyRequest{ClientID: credential.Client.ID, ActorID: fixture.actor, NeverExpires: true})
		// Then
		if err != nil {
			t.Fatal(err)
		}
		if issued.Key.RotatedFrom != nil || issued.Key.ExpiresAt != nil {
			t.Fatal("issuance changed expiration or rotation semantics")
		}
	})
	t.Run("disabled caller cannot issue", func(t *testing.T) {
		credential := fixture.create(t, apikey.AudienceExternal)
		if _, err := fixture.service.UpdateClient(t.Context(), apikey.UpdateClientRecord{ID: credential.Client.ID, Name: credential.Client.Name, Scopes: credential.Client.Scopes, Enabled: false}); err != nil {
			t.Fatal(err)
		}
		response := fixture.request(http.MethodPost, "/management/api-clients/"+credential.Client.ID+"/keys", `{"never_expires":true}`)
		if response.Code != 409 || !strings.Contains(response.Body.String(), `"api_client_disabled"`) {
			t.Fatalf("disabled issuance status %d", response.Code)
		}
	})
	t.Run("internal issuance enforces expiration", func(t *testing.T) {
		credential := fixture.create(t, apikey.AudienceInternal)
		if err := fixture.service.RevokeKey(t.Context(), credential.Key.ID, fixture.actor); err != nil {
			t.Fatal(err)
		}
		for _, body := range []string{`{"never_expires":true}`, `{"never_expires":false}`, `{"expires_at":"2099-01-01T00:00:00Z","never_expires":false}`} {
			response := fixture.request(http.MethodPost, "/management/api-clients/"+credential.Client.ID+"/keys", body)
			if response.Code != 422 {
				t.Fatalf("invalid expiration status %d", response.Code)
			}
		}
	})
	t.Run("management requires web and administrator authentication", func(t *testing.T) { testIssuanceAuthorization(t, fixture) })
}

func testIssueLifecycle(t *testing.T, fixture credentialFixture) {
	t.Helper()
	// Given an existing external caller.
	credential := fixture.create(t, apikey.AudienceExternal)
	path := "/management/api-clients/" + credential.Client.ID
	response := fixture.request(http.MethodPost, path+"/keys", `{"never_expires":true}`)
	if response.Code != 409 || !strings.Contains(response.Body.String(), `"api_client_has_active_key"`) {
		t.Fatalf("active issuance status %d", response.Code)
	}
	// When the administrator rotates, revokes, then reissues.
	rotatedResponse := fixture.request(http.MethodPost, path+"/rotate", `{"overlap_hours":0}`)
	if rotatedResponse.Code != 200 {
		t.Fatalf("rotation status %d", rotatedResponse.Code)
	}
	rotated := decodeCredential(t, rotatedResponse)
	if rotated.Key.RotatedFrom == nil || *rotated.Key.RotatedFrom != credential.Key.ID {
		t.Fatal("rotation history missing")
	}
	if response := fixture.request(http.MethodPost, "/management/api-keys/"+rotated.Key.ID+"/revoke", ""); response.Code != 204 {
		t.Fatalf("revoke status %d", response.Code)
	}
	issuedResponse := fixture.request(http.MethodPost, path+"/keys", `{"never_expires":true}`)
	if issuedResponse.Code != 201 {
		t.Fatalf("issuance status %d body %s", issuedResponse.Code, issuedResponse.Body)
	}
	issued := decodeCredential(t, issuedResponse)
	// Then the new credential works, old ones fail, and all records survive.
	if issuedResponse.Header().Get("Cache-Control") != "private, no-store" || issued.Key.RotatedFrom != nil {
		t.Fatal("invalid issuance contract")
	}
	policy := apikey.AccessPolicy{Audiences: []apikey.Audience{apikey.AudienceInternal, apikey.AudienceExternal}, Scope: apikey.ScopeExampleCall}
	if _, err := fixture.service.Authenticate(t.Context(), issued.Token, policy); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{credential.Token, rotated.Token} {
		if _, err := fixture.service.Authenticate(t.Context(), token, policy); !errors.Is(err, apikey.ErrInvalidToken) {
			t.Fatalf("old key remains usable: %v", err)
		}
	}
	clients, err := fixture.service.ListClients(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range clients {
		if client.ID == credential.Client.ID && len(client.Keys) != 3 {
			t.Fatalf("key history size %d", len(client.Keys))
		}
	}
}

func testConcurrentIssuance(t *testing.T, fixture credentialFixture) {
	t.Helper()
	// Given one enabled caller whose initial credential is revoked.
	credential := fixture.create(t, apikey.AudienceExternal)
	if err := fixture.service.RevokeKey(t.Context(), credential.Key.ID, fixture.actor); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 8)
	// When eight requests race to issue a key.
	for range 8 {
		go func() {
			<-start
			_, err := fixture.service.IssueKey(t.Context(), apikey.IssueKeyRequest{ClientID: credential.Client.ID, ActorID: fixture.actor, NeverExpires: true})
			results <- err
		}()
	}
	close(start)
	succeeded := 0
	for range 8 {
		err := <-results
		if err == nil {
			succeeded++
		} else if !errors.Is(err, apikey.ErrClientHasActiveKey) {
			t.Error(err)
		}
	}
	// Then the client row lock admits exactly one request.
	if succeeded != 1 {
		t.Fatalf("issuance successes %d", succeeded)
	}
}

func testIssuanceAuthorization(t *testing.T, fixture credentialFixture) {
	t.Helper()
	credential := fixture.create(t, apikey.AudienceExternal)
	path := "/management/api-clients/" + credential.Client.ID + "/keys"
	for _, withWebToken := range []bool{false, true} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"never_expires":true}`))
		request.Header.Set("Content-Type", "application/json")
		if withWebToken {
			request.Header.Set("X-HeyBlog-Web-Token", credentialWebToken)
		}
		response := httptest.NewRecorder()
		fixture.router.ServeHTTP(response, request)
		if response.Code != 401 {
			t.Fatalf("anonymous issuance status %d", response.Code)
		}
	}
	if _, err := fixture.pool.Exec(t.Context(), "UPDATE identity.users SET role = 'ADMIN' WHERE id = $1", fixture.actor); err != nil {
		t.Fatal(err)
	}
	_, tokens, err := fixture.auth.Login(t.Context(), "credential_admin", "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	fixture.access = tokens[0]
	response := fixture.request(http.MethodPost, path, `{"never_expires":true}`)
	if response.Code != 403 || !strings.Contains(response.Body.String(), `"api_key_management_forbidden"`) {
		t.Fatalf("non SYS_ADMIN status %d", response.Code)
	}
}
