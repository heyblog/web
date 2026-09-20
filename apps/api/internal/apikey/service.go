package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"time"
)

const (
	keyPrefix              = "hbk_"
	maximumInternalKeyLife = 90 * 24 * time.Hour
	maximumRotationOverlap = 24 * time.Hour
	usageTouchInterval     = 15 * time.Minute
)

var (
	ErrInvalidAudience   = errors.New("invalid API client audience")
	ErrInvalidExpiration = errors.New("invalid API key expiration")
	ErrInvalidName       = errors.New("invalid API client name")
	ErrInvalidOverlap    = errors.New("invalid API key rotation overlap")
	ErrInvalidScope      = errors.New("invalid API client scope")
	ErrInvalidToken      = errors.New("invalid API key")
	ErrInsufficientScope = errors.New("insufficient API key scope")
)

type Store interface {
	CreateClient(context.Context, CreateClientRecord) (Credential, error)
	ListClients(context.Context) ([]Client, error)
	UpdateClient(context.Context, UpdateClientRecord) (Client, error)
	Rotate(context.Context, RotateRecord) (Credential, error)
	IssueKey(context.Context, IssueKeyRecord) (Credential, error)
	RevokeKey(context.Context, RevokeKeyRecord) error
	FindCredential(context.Context, string) (StoredCredential, error)
	TouchKey(context.Context, string, time.Time) error
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, now: now}
}

func (service *Service) CreateClient(ctx context.Context, request CreateClientRequest) (Credential, error) {
	now := service.now().UTC()
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	if err := validateCreateRequest(request, now); err != nil {
		return Credential{}, err
	}
	publicID, secret, token, err := newKeyMaterial()
	if err != nil {
		return Credential{}, err
	}
	digest := secretDigest(secret)
	credential, err := service.store.CreateClient(ctx, CreateClientRecord{
		CreateClientRequest: request,
		PublicID:            publicID,
		Prefix:              token[:min(len(token), 20)],
		SecretHash:          digest[:],
	})
	if err != nil {
		return Credential{}, err
	}
	credential.Token = token
	return credential, nil
}

func (service *Service) Rotate(ctx context.Context, request RotateRequest) (Credential, error) {
	if request.Overlap < 0 || request.Overlap > maximumRotationOverlap {
		return Credential{}, ErrInvalidOverlap
	}
	publicID, secret, token, err := newKeyMaterial()
	if err != nil {
		return Credential{}, err
	}
	digest := secretDigest(secret)
	credential, err := service.store.Rotate(ctx, RotateRecord{
		RotateRequest: request,
		Now:           service.now().UTC(),
		PublicID:      publicID,
		Prefix:        token[:min(len(token), 20)],
		SecretHash:    digest[:],
	})
	if err != nil {
		return Credential{}, err
	}
	credential.Token = token
	return credential, nil
}

func (service *Service) Authenticate(ctx context.Context, token string, policy AccessPolicy) (Principal, error) {
	publicID, secret, ok := parseToken(token)
	if !ok {
		return Principal{}, ErrInvalidToken
	}
	stored, err := service.store.FindCredential(ctx, publicID)
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	digest := secretDigest(secret)
	now := service.now().UTC()
	if subtle.ConstantTimeCompare(digest[:], stored.SecretHash) != 1 ||
		stored.RevokedAt != nil || stored.ClientDisabledAt != nil ||
		(stored.ExpiresAt != nil && !stored.ExpiresAt.After(now)) {
		return Principal{}, ErrInvalidToken
	}
	if !slices.Contains(policy.Audiences, stored.Audience) || !containsScope(stored.Scopes, policy.Scope) {
		return Principal{}, ErrInsufficientScope
	}
	if stored.LastUsedAt == nil || stored.LastUsedAt.Before(now.Add(-usageTouchInterval)) {
		_ = service.store.TouchKey(ctx, stored.KeyID, now)
	}
	return stored.Principal, nil
}

func (service *Service) ListClients(ctx context.Context) ([]Client, error) {
	return service.store.ListClients(ctx)
}

func (service *Service) UpdateClient(ctx context.Context, record UpdateClientRecord) (Client, error) {
	record.Name = strings.TrimSpace(record.Name)
	record.Description = strings.TrimSpace(record.Description)
	if record.Name == "" || len(record.Name) > 128 || len(record.Description) > 512 {
		return Client{}, ErrInvalidName
	}
	if !validScopes(record.Scopes) {
		return Client{}, ErrInvalidScope
	}
	return service.store.UpdateClient(ctx, record)
}

func (service *Service) RevokeKey(ctx context.Context, keyID, actorID string) error {
	return service.store.RevokeKey(ctx, RevokeKeyRecord{KeyID: keyID, ActorID: actorID, Now: service.now().UTC()})
}

func validateCreateRequest(request CreateClientRequest, now time.Time) error {
	if request.Name == "" || len(request.Name) > 128 || len(request.Description) > 512 {
		return ErrInvalidName
	}
	if request.Audience != AudienceInternal && request.Audience != AudienceExternal {
		return ErrInvalidAudience
	}
	if !validScopes(request.Scopes) {
		return ErrInvalidScope
	}
	for _, scope := range request.Scopes {
		if !scopeAllowed(request.Audience, scope) {
			return ErrInvalidScope
		}
	}
	return validateExpiration(request.Audience, request.ExpiresAt, request.NeverExpires, now)
}

func validateExpiration(audience Audience, expiresAt *time.Time, neverExpires bool, now time.Time) error {
	if audience == AudienceInternal {
		if neverExpires || expiresAt == nil || !expiresAt.After(now) || expiresAt.After(now.Add(maximumInternalKeyLife)) {
			return ErrInvalidExpiration
		}
		return nil
	}
	if neverExpires {
		if expiresAt != nil {
			return ErrInvalidExpiration
		}
		return nil
	}
	if expiresAt == nil || !expiresAt.After(now) {
		return ErrInvalidExpiration
	}
	return nil
}

func scopeAllowed(audience Audience, scope Scope) bool {
	switch audience {
	case AudienceInternal:
		return scope == ScopeDataImportWrite || scope == ScopeExampleCall
	case AudienceExternal:
		return scope == ScopeExampleCall
	default:
		return false
	}
}

func validScopes(scopes []Scope) bool {
	if len(scopes) == 0 {
		return false
	}
	seen := make(map[Scope]bool, len(scopes))
	for _, scope := range scopes {
		if (scope != ScopeDataImportWrite && scope != ScopeExampleCall) || seen[scope] {
			return false
		}
		seen[scope] = true
	}
	return true
}

func containsScope(scopes []Scope, required Scope) bool {
	for _, scope := range scopes {
		if scope == required {
			return true
		}
	}
	return false
}

func newKeyMaterial() (string, string, string, error) {
	publicBytes := make([]byte, 9)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(publicBytes); err != nil {
		return "", "", "", err
	}
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", "", err
	}
	publicID := base64.RawURLEncoding.EncodeToString(publicBytes)
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)
	return publicID, secret, keyPrefix + publicID + "_" + secret, nil
}

func parseToken(token string) (string, string, bool) {
	if !strings.HasPrefix(token, keyPrefix) {
		return "", "", false
	}
	material := strings.TrimPrefix(token, keyPrefix)
	if len(material) != 56 || material[12] != '_' {
		return "", "", false
	}
	return material[:12], material[13:], true
}

func secretDigest(secret string) [sha256.Size]byte { return sha256.Sum256([]byte(secret)) }
