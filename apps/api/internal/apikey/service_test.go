package apikey

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCreateClientRejectsInternalKeyBeyondNinetyDays(t *testing.T) {
	t.Parallel()

	// Given
	now := time.Date(2026, time.September, 19, 10, 0, 0, 0, time.UTC)
	service := NewService(&fakeStore{}, func() time.Time { return now })

	// When
	_, err := service.CreateClient(context.Background(), CreateClientRequest{
		Name:      "content importer",
		Audience:  AudienceInternal,
		Scopes:    []Scope{ScopeDataImportWrite},
		ExpiresAt: timePointer(now.Add(90*24*time.Hour + time.Second)),
		CreatedBy: "00000000-0000-0000-0000-000000000001",
	})

	// Then
	if !errors.Is(err, ErrInvalidExpiration) {
		t.Fatalf("CreateClient() error = %v, want ErrInvalidExpiration", err)
	}
}

func TestCreateClientAllowsPermanentExternalKeyWhenExplicit(t *testing.T) {
	t.Parallel()

	// Given
	now := time.Date(2026, time.September, 19, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{createResult: Credential{}}
	service := NewService(store, func() time.Time { return now })

	// When
	credential, err := service.CreateClient(context.Background(), CreateClientRequest{
		Name:         "public catalog",
		Audience:     AudienceExternal,
		Scopes:       []Scope{ScopeExampleCall},
		NeverExpires: true,
		CreatedBy:    "00000000-0000-0000-0000-000000000001",
	})

	// Then
	if err != nil {
		t.Fatalf("CreateClient() error = %v", err)
	}
	if credential.Token == "" {
		t.Fatal("CreateClient() token is empty")
	}
	if store.createInput.ExpiresAt != nil {
		t.Fatalf("CreateClient() expiresAt = %v, want nil", store.createInput.ExpiresAt)
	}
	if len(store.createInput.SecretHash) != 32 {
		t.Fatalf("CreateClient() secret hash length = %d, want 32", len(store.createInput.SecretHash))
	}
}

func TestCreateClientAllowsExampleScopeForExternalClient(t *testing.T) {
	t.Parallel()

	// Given
	store := &fakeStore{}
	service := NewService(store, time.Now)

	// When
	_, err := service.CreateClient(context.Background(), CreateClientRequest{
		Name:         "example consumer",
		Audience:     AudienceExternal,
		Scopes:       []Scope{ScopeExampleCall},
		NeverExpires: true,
		CreatedBy:    "00000000-0000-0000-0000-000000000001",
	})

	// Then
	if err != nil {
		t.Fatalf("CreateClient() error = %v", err)
	}
}

func TestCreateClientAllowsExampleScopeForInternalClient(t *testing.T) {
	t.Parallel()

	// Given
	now := time.Date(2026, time.September, 19, 10, 0, 0, 0, time.UTC)
	service := NewService(&fakeStore{}, func() time.Time { return now })

	// When
	_, err := service.CreateClient(context.Background(), CreateClientRequest{
		Name:      "internal consumer",
		Audience:  AudienceInternal,
		Scopes:    []Scope{ScopeExampleCall},
		ExpiresAt: timePointer(now.Add(24 * time.Hour)),
		CreatedBy: "00000000-0000-0000-0000-000000000001",
	})

	// Then
	if err != nil {
		t.Fatalf("CreateClient() error = %v", err)
	}
}

func TestRotateRejectsOverlapBeyondTwentyFourHours(t *testing.T) {
	t.Parallel()

	// Given
	now := time.Date(2026, time.September, 19, 10, 0, 0, 0, time.UTC)
	service := NewService(&fakeStore{}, func() time.Time { return now })

	// When
	_, err := service.Rotate(context.Background(), RotateRequest{
		ClientID: "00000000-0000-0000-0000-000000000002",
		Overlap:  24*time.Hour + time.Second,
		ActorID:  "00000000-0000-0000-0000-000000000001",
	})

	// Then
	if !errors.Is(err, ErrInvalidOverlap) {
		t.Fatalf("Rotate() error = %v, want ErrInvalidOverlap", err)
	}
}

func TestUpdateClientRejectsEmptyScopes(t *testing.T) {
	t.Parallel()

	// Given
	store := &fakeStore{}
	service := NewService(store, time.Now)

	// When
	_, err := service.UpdateClient(context.Background(), UpdateClientRecord{
		ID:      "00000000-0000-0000-0000-000000000002",
		Name:    "content importer",
		Enabled: true,
	})

	// Then
	if !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("UpdateClient() error = %v, want ErrInvalidScope", err)
	}
	if store.updateCalled {
		t.Fatal("UpdateClient() called the store for an invalid request")
	}
}

func TestUpdateClientRejectsUnknownScope(t *testing.T) {
	t.Parallel()

	// Given
	store := &fakeStore{}
	service := NewService(store, time.Now)

	// When
	_, err := service.UpdateClient(context.Background(), UpdateClientRecord{
		ID:      "00000000-0000-0000-0000-000000000002",
		Name:    "content importer",
		Scopes:  []Scope{"unknown.write"},
		Enabled: true,
	})

	// Then
	if !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("UpdateClient() error = %v, want ErrInvalidScope", err)
	}
	if store.updateCalled {
		t.Fatal("UpdateClient() called the store for an invalid request")
	}
}

func TestUpdateClientAcceptsExampleScope(t *testing.T) {
	t.Parallel()

	// Given
	store := &fakeStore{}
	service := NewService(store, time.Now)

	// When
	_, err := service.UpdateClient(context.Background(), UpdateClientRecord{
		ID:      "00000000-0000-0000-0000-000000000002",
		Name:    "example consumer",
		Scopes:  []Scope{ScopeExampleCall},
		Enabled: true,
	})

	// Then
	if err != nil {
		t.Fatalf("UpdateClient() error = %v", err)
	}
	if !store.updateCalled {
		t.Fatal("UpdateClient() did not call the store")
	}
}

func TestAuthenticateRejectsScopeOutsideAudience(t *testing.T) {
	t.Parallel()

	// Given
	secret := "0123456789012345678901234567890123456789012"
	digest := secretDigest(secret)
	store := &fakeStore{authenticateResult: StoredCredential{
		Principal: Principal{
			ClientID: "00000000-0000-0000-0000-000000000002",
			KeyID:    "00000000-0000-0000-0000-000000000003",
			Audience: AudienceExternal,
			Scopes:   []Scope{ScopeDataImportWrite},
		},
		SecretHash: digest[:],
	}}
	service := NewService(store, time.Now)

	// When
	_, err := service.Authenticate(
		context.Background(),
		"hbk_public123456_"+secret,
		AccessPolicy{Audiences: []Audience{AudienceInternal}, Scope: ScopeDataImportWrite},
	)

	// Then
	if !errors.Is(err, ErrInsufficientScope) {
		t.Fatalf("Authenticate() error = %v, want ErrInsufficientScope", err)
	}
}

type fakeStore struct {
	createInput        CreateClientRecord
	createResult       Credential
	authenticateResult StoredCredential
	updateCalled       bool
}

func (store *fakeStore) CreateClient(_ context.Context, input CreateClientRecord) (Credential, error) {
	store.createInput = input
	return store.createResult, nil
}

func (store *fakeStore) Rotate(context.Context, RotateRecord) (Credential, error) {
	return Credential{}, nil
}

func (store *fakeStore) IssueKey(context.Context, IssueKeyRecord) (Credential, error) {
	return Credential{}, nil
}

func (store *fakeStore) FindCredential(context.Context, string) (StoredCredential, error) {
	return store.authenticateResult, nil
}

func (store *fakeStore) TouchKey(context.Context, string, time.Time) error { return nil }

func (store *fakeStore) ListClients(context.Context) ([]Client, error) { return nil, nil }

func (store *fakeStore) UpdateClient(context.Context, UpdateClientRecord) (Client, error) {
	store.updateCalled = true
	return Client{}, nil
}

func (store *fakeStore) RevokeKey(context.Context, RevokeKeyRecord) error { return nil }

func timePointer(value time.Time) *time.Time { return &value }
