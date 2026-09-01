package auth

import (
	"context"

	dbgen "heyblog-api/internal/database/gen"
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

func (repo *repository) listUsers(ctx context.Context) ([]dbUser, error) {
	rows, err := repo.queries.ListUsersForManagement(ctx)
	if err != nil {
		return nil, err
	}
	users := make([]dbUser, len(rows))
	for index, row := range rows {
		users[index] = mapUser(row)
	}
	return users, nil
}

func (repo *repository) setRole(ctx context.Context, userID, role string) error {
	parsed, err := parseUUID(userID)
	if err != nil {
		return err
	}
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := repo.queries.WithTx(tx)
	if err := queries.SetUserRole(ctx, dbgen.SetUserRoleParams{ID: parsed, Role: role}); err != nil {
		return err
	}
	if role == string(RoleUser) {
		if err := queries.DeleteUserManagementPermissions(ctx, parsed); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (repo *repository) replacePermissions(ctx context.Context, userID, actorID string, permissions []Permission) error {
	userUUID, err := parseUUID(userID)
	if err != nil {
		return err
	}
	actorUUID, err := parseUUID(actorID)
	if err != nil {
		return err
	}
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := repo.queries.WithTx(tx)
	if err := queries.DeleteUserManagementPermissions(ctx, userUUID); err != nil {
		return err
	}
	for _, permission := range permissions {
		if err := queries.CreateUserManagementPermission(ctx, dbgen.CreateUserManagementPermissionParams{UserID: userUUID, PermissionKey: string(permission), GrantedBy: actorUUID}); err != nil {
			return err
		}
	}
	if err := queries.BumpUserAuthVersion(ctx, userUUID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func mapOAuth(row dbgen.IdentityOauthIdentity) dbOAuth {
	return dbOAuth{ID: row.ID.String(), UserID: row.UserID.String(), ProviderID: row.ProviderUserID, ProviderLogin: row.ProviderLogin}
}
