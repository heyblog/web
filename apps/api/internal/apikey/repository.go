package apikey

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "heyblog-api/internal/database/gen"
)

var (
	ErrClientDisabled     = errors.New("API client is disabled")
	ErrClientNameConflict = errors.New("API client name is already in use")
	ErrNotFound           = errors.New("API client or key was not found")
)

type repository struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
}

func NewRepository(pool *pgxpool.Pool) Store {
	return &repository{pool: pool, queries: dbgen.New(pool)}
}

func (repo *repository) CreateClient(ctx context.Context, record CreateClientRecord) (Credential, error) {
	actorID, err := parseUUID(record.CreatedBy)
	if err != nil {
		return Credential{}, err
	}
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return Credential{}, fmt.Errorf("begin API client creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := repo.queries.WithTx(tx)
	clientRow, err := queries.CreateAPIClient(ctx, dbgen.CreateAPIClientParams{
		Name: record.Name, Description: record.Description, Audience: string(record.Audience), CreatedBy: actorID,
	})
	if isClientNameConflict(err) {
		return Credential{}, ErrClientNameConflict
	}
	if err != nil {
		return Credential{}, fmt.Errorf("create API client: %w", err)
	}
	for _, scope := range record.Scopes {
		if err := queries.CreateAPIClientScope(ctx, dbgen.CreateAPIClientScopeParams{ClientID: clientRow.ID, Scope: string(scope)}); err != nil {
			return Credential{}, fmt.Errorf("grant API client scope: %w", err)
		}
	}
	keyRow, err := queries.CreateAPIKey(ctx, dbgen.CreateAPIKeyParams{
		ClientID: clientRow.ID, PublicID: record.PublicID, KeyPrefix: record.Prefix,
		SecretHash: record.SecretHash, ExpiresAt: optionalTime(record.ExpiresAt), CreatedBy: actorID,
	})
	if err != nil {
		return Credential{}, fmt.Errorf("create API key: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Credential{}, fmt.Errorf("commit API client creation: %w", err)
	}
	client := mapClient(clientRow, record.Scopes, []Key{mapKey(keyRow)})
	return Credential{Client: client, Key: client.Keys[0]}, nil
}

func (repo *repository) ListClients(ctx context.Context) ([]Client, error) {
	clientRows, err := repo.queries.ListAPIClients(ctx)
	if err != nil {
		return nil, fmt.Errorf("list API clients: %w", err)
	}
	scopeRows, err := repo.queries.ListAPIClientScopes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list API client scopes: %w", err)
	}
	keyRows, err := repo.queries.ListAPIKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("list API keys: %w", err)
	}
	scopesByClient := make(map[string][]Scope)
	for _, row := range scopeRows {
		id := row.ClientID.String()
		scopesByClient[id] = append(scopesByClient[id], Scope(row.Scope))
	}
	keysByClient := make(map[string][]Key)
	for _, row := range keyRows {
		id := row.ClientID.String()
		keysByClient[id] = append(keysByClient[id], mapListedKey(row))
	}
	clients := make([]Client, len(clientRows))
	for index, row := range clientRows {
		id := row.ID.String()
		clients[index] = mapClient(row, scopesByClient[id], keysByClient[id])
	}
	return clients, nil
}

func (repo *repository) UpdateClient(ctx context.Context, record UpdateClientRecord) (Client, error) {
	clientID, err := parseUUID(record.ID)
	if err != nil {
		return Client{}, ErrNotFound
	}
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return Client{}, fmt.Errorf("begin API client update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := repo.queries.WithTx(tx)
	current, err := queries.GetAPIClient(ctx, clientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Client{}, ErrNotFound
	}
	if err != nil {
		return Client{}, fmt.Errorf("get API client: %w", err)
	}
	for _, scope := range record.Scopes {
		if !scopeAllowed(Audience(current.Audience), scope) {
			return Client{}, ErrInvalidScope
		}
	}
	updated, err := queries.UpdateAPIClient(ctx, dbgen.UpdateAPIClientParams{
		ID: clientID, Name: record.Name, Description: record.Description, Enabled: record.Enabled,
	})
	if isClientNameConflict(err) {
		return Client{}, ErrClientNameConflict
	}
	if err != nil {
		return Client{}, fmt.Errorf("update API client: %w", err)
	}
	if err := queries.DeleteAPIClientScopes(ctx, clientID); err != nil {
		return Client{}, fmt.Errorf("clear API client scopes: %w", err)
	}
	for _, scope := range record.Scopes {
		if err := queries.CreateAPIClientScope(ctx, dbgen.CreateAPIClientScopeParams{ClientID: clientID, Scope: string(scope)}); err != nil {
			return Client{}, fmt.Errorf("replace API client scope: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Client{}, fmt.Errorf("commit API client update: %w", err)
	}
	return mapClient(updated, record.Scopes, []Key{}), nil
}

func (repo *repository) Rotate(ctx context.Context, record RotateRecord) (Credential, error) {
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
		return Credential{}, fmt.Errorf("begin API key rotation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := repo.queries.WithTx(tx)
	clientRow, err := queries.GetAPIClient(ctx, clientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Credential{}, ErrNotFound
	}
	if err != nil {
		return Credential{}, fmt.Errorf("get API client for rotation: %w", err)
	}
	if clientRow.DisabledAt.Valid {
		return Credential{}, ErrClientDisabled
	}
	previous, err := queries.GetLatestActiveAPIKey(ctx, dbgen.GetLatestActiveAPIKeyParams{ClientID: clientID, Now: requiredTime(record.Now)})
	if errors.Is(err, pgx.ErrNoRows) {
		return Credential{}, ErrNotFound
	}
	if err != nil {
		return Credential{}, fmt.Errorf("get active API key: %w", err)
	}
	expiresAt := rotatedExpiration(Audience(clientRow.Audience), previous, record.Now)
	keyRow, err := queries.CreateAPIKey(ctx, dbgen.CreateAPIKeyParams{
		ClientID: clientID, PublicID: record.PublicID, KeyPrefix: record.Prefix,
		SecretHash: record.SecretHash, ExpiresAt: expiresAt, RotatedFromID: previous.ID, CreatedBy: actorID,
	})
	if err != nil {
		return Credential{}, fmt.Errorf("create rotated API key: %w", err)
	}
	if err := queries.RetireAPIKeyForRotation(ctx, dbgen.RetireAPIKeyForRotationParams{
		OverlapUntil: requiredTime(record.Now.Add(record.Overlap)), Now: requiredTime(record.Now), ActorID: actorID, ID: previous.ID,
	}); err != nil {
		return Credential{}, fmt.Errorf("retire previous API key: %w", err)
	}
	scopeRows, err := queries.ListAPIClientScopesByClient(ctx, clientID)
	if err != nil {
		return Credential{}, fmt.Errorf("list rotated API client scopes: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Credential{}, fmt.Errorf("commit API key rotation: %w", err)
	}
	scopes := make([]Scope, len(scopeRows))
	for index, scope := range scopeRows {
		scopes[index] = Scope(scope)
	}
	client := mapClient(clientRow, scopes, []Key{mapKey(keyRow)})
	return Credential{Client: client, Key: client.Keys[0]}, nil
}

func (repo *repository) RevokeKey(ctx context.Context, record RevokeKeyRecord) error {
	keyID, err := parseUUID(record.KeyID)
	if err != nil {
		return ErrNotFound
	}
	actorID, err := parseUUID(record.ActorID)
	if err != nil {
		return err
	}
	rows, err := repo.queries.RevokeAPIKey(ctx, dbgen.RevokeAPIKeyParams{Now: requiredTime(record.Now), ActorID: actorID, ID: keyID})
	if err != nil {
		return fmt.Errorf("revoke API key: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (repo *repository) FindCredential(ctx context.Context, publicID string) (StoredCredential, error) {
	row, err := repo.queries.FindAPIKeyCredential(ctx, publicID)
	if err != nil {
		return StoredCredential{}, err
	}
	scopes := make([]Scope, len(row.Scopes))
	for index, scope := range row.Scopes {
		scopes[index] = Scope(scope)
	}
	return StoredCredential{
		Principal:  Principal{ClientID: row.ClientID.String(), KeyID: row.KeyID.String(), Audience: Audience(row.Audience), Scopes: scopes},
		SecretHash: row.SecretHash, ExpiresAt: timeValue(row.ExpiresAt), RevokedAt: timeValue(row.RevokedAt),
		ClientDisabledAt: timeValue(row.ClientDisabledAt), LastUsedAt: timeValue(row.LastUsedAt),
	}, nil
}

func (repo *repository) TouchKey(ctx context.Context, keyID string, usedAt time.Time) error {
	parsed, err := parseUUID(keyID)
	if err != nil {
		return err
	}
	return repo.queries.TouchAPIKeyUsage(ctx, dbgen.TouchAPIKeyUsageParams{UsedAt: requiredTime(usedAt), ID: parsed})
}
