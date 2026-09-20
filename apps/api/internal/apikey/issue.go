package apikey

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	dbgen "heyblog-api/internal/database/gen"
)

var ErrClientHasActiveKey = errors.New("API client already has an active key")

func (service *Service) IssueKey(ctx context.Context, request IssueKeyRequest) (Credential, error) {
	publicID, secret, token, err := newKeyMaterial()
	if err != nil {
		return Credential{}, err
	}
	digest := secretDigest(secret)
	credential, err := service.store.IssueKey(ctx, IssueKeyRecord{
		IssueKeyRequest: request, Now: service.now().UTC(),
		PublicID: publicID, Prefix: token[:20], SecretHash: digest[:],
	})
	if err != nil {
		return Credential{}, err
	}
	credential.Token = token
	return credential, nil
}

func (repo *repository) IssueKey(ctx context.Context, record IssueKeyRecord) (Credential, error) {
	clientID, err := parseUUID(record.ClientID)
	if err != nil {
		return Credential{}, ErrNotFound
	}
	actorID, err := parseUUID(record.ActorID)
	if err != nil {
		return Credential{}, err
	}
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return Credential{}, fmt.Errorf("begin API key issuance: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := repo.queries.WithTx(tx)
	clientRow, err := queries.GetAPIClient(ctx, clientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Credential{}, ErrNotFound
	}
	if err != nil {
		return Credential{}, fmt.Errorf("get API client for issuance: %w", err)
	}
	if clientRow.DisabledAt.Valid {
		return Credential{}, ErrClientDisabled
	}
	_, err = queries.GetLatestActiveAPIKey(ctx, dbgen.GetLatestActiveAPIKeyParams{ClientID: clientID, Now: requiredTime(record.Now)})
	if err == nil {
		return Credential{}, ErrClientHasActiveKey
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Credential{}, fmt.Errorf("check active API keys: %w", err)
	}
	if err := validateExpiration(Audience(clientRow.Audience), record.ExpiresAt, record.NeverExpires, record.Now); err != nil {
		return Credential{}, err
	}
	keyRow, err := queries.CreateAPIKey(ctx, dbgen.CreateAPIKeyParams{
		ClientID: clientID, PublicID: record.PublicID, KeyPrefix: record.Prefix,
		SecretHash: record.SecretHash, ExpiresAt: optionalTime(record.ExpiresAt), CreatedBy: actorID,
	})
	if err != nil {
		return Credential{}, fmt.Errorf("create issued API key: %w", err)
	}
	scopeRows, err := queries.ListAPIClientScopesByClient(ctx, clientID)
	if err != nil {
		return Credential{}, fmt.Errorf("list issued API client scopes: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Credential{}, fmt.Errorf("commit API key issuance: %w", err)
	}
	scopes := make([]Scope, len(scopeRows))
	for index, scope := range scopeRows {
		scopes[index] = Scope(scope)
	}
	client := mapClient(clientRow, scopes, []Key{mapKey(keyRow)})
	return Credential{Client: client, Key: client.Keys[0]}, nil
}
