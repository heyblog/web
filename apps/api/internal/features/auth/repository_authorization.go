package auth

import (
	"context"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func (repo *repository) oauthByProviderID(ctx context.Context, id string) (dbOAuth, error) {
	row, err := repo.queries.GetGitHubIdentity(ctx, id)
	return mapOAuth(row), err
}

func (repo *repository) oauthByUser(ctx context.Context, userID string) (dbOAuth, error) {
	parsed, err := parseUUID(userID)
	if err != nil {
		return dbOAuth{}, err
	}
	row, err := repo.queries.GetUserGitHubIdentity(ctx, parsed)
	return mapOAuth(row), err
}

func (repo *repository) upsertOAuth(ctx context.Context, userID, providerID, login string, profile []byte) error {
	parsed, err := parseUUID(userID)
	if err != nil {
		return err
	}
	_, err = repo.queries.UpsertGitHubIdentity(ctx, dbgen.UpsertGitHubIdentityParams{ProviderUserID: providerID, ProviderLogin: &login, Profile: profile, UserID: parsed})
	return err
}

func (repo *repository) unlinkOAuth(ctx context.Context, userID string) error {
	parsed, err := parseUUID(userID)
	if err != nil {
		return err
	}
	return repo.queries.UnlinkOAuthIdentity(ctx, dbgen.UnlinkOAuthIdentityParams{UserID: parsed, Provider: "GITHUB"})
}

func (repo *repository) permissions(ctx context.Context, userID string) ([]Permission, error) {
	parsed, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := repo.queries.ListUserManagementPermissions(ctx, parsed)
	if err != nil {
		return nil, err
	}
	result := make([]Permission, len(rows))
	for index, key := range rows {
		result[index] = Permission(key)
	}
	return result, nil
}

func mapOAuth(row dbgen.IdentityOauthIdentity) dbOAuth {
	return dbOAuth{ID: row.ID.String(), UserID: row.UserID.String(), ProviderID: row.ProviderUserID, ProviderLogin: row.ProviderLogin}
}
