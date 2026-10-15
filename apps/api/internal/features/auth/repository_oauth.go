package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func (repo *repository) loginOAuth(ctx context.Context, provider OAuthProvider, identity oauthIdentity) (dbUser, error) {
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return dbUser{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := repo.queries.WithTx(tx)
	if err := q.LockOAuthMutation(ctx); err != nil {
		return dbUser{}, err
	}
	existing, err := q.GetOAuthIdentity(ctx, dbgen.GetOAuthIdentityParams{Provider: string(provider), ProviderUserID: identity.Subject})
	var record dbgen.IdentityUser
	if err == nil {
		record, err = q.GetUserForUpdate(ctx, existing.UserID)
	} else if isNotFound(err) {
		record, err = q.GetUserByEmail(ctx, identity.Email)
		if err == nil {
			record, err = q.GetUserForUpdate(ctx, record.ID)
		}
		if isNotFound(err) {
			username, usernameErr := availableOAuthUsername(ctx, q, identity.Login, identity.Subject)
			if usernameErr != nil {
				return dbUser{}, usernameErr
			}
			record, err = q.CreateOAuthUser(ctx, dbgen.CreateOAuthUserParams{Email: &identity.Email, Username: username, DisplayName: oauthDisplayName(identity.Name)})
		}
	}
	if err != nil {
		return dbUser{}, oauthIdentityError(provider, err)
	}
	user := mapUser(record)
	if !isActive(user) || user.DeletionRequestedAt != nil {
		return dbUser{}, newAuthError("account_unavailable", 409, "Account is unavailable")
	}
	if user.VerifiedAt == nil && user.Email != nil && normalizeEmail(*user.Email) == identity.Email {
		if err := q.SetUserEmailVerified(ctx, record.ID); err != nil {
			return dbUser{}, err
		}
	}
	if err := upsertOAuthIdentity(ctx, q, user.ID, provider, identity); err != nil {
		return dbUser{}, err
	}
	record, err = q.RecordUserLogin(ctx, record.ID)
	if err != nil {
		return dbUser{}, err
	}
	return mapUser(record), tx.Commit(ctx)
}

func (repo *repository) bindOAuth(ctx context.Context, actor User, provider OAuthProvider, identity oauthIdentity) error {
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := repo.queries.WithTx(tx)
	if err := q.LockOAuthMutation(ctx); err != nil {
		return err
	}
	current, err := lockOAuthActor(ctx, q, actor)
	if err != nil {
		return err
	}
	if current.Email == nil || normalizeEmail(*current.Email) != identity.Email {
		return newAuthError(provider.slug()+"_bind_email_mismatch", 409, "Provider email must match the current account email")
	}
	if err := upsertOAuthIdentity(ctx, q, actor.ID, provider, identity); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func lockOAuthActor(ctx context.Context, q *dbgen.Queries, actor User) (dbUser, error) {
	id, err := parseUUID(actor.ID)
	if err != nil {
		return dbUser{}, err
	}
	row, err := q.GetUserForUpdate(ctx, id)
	if err != nil {
		return dbUser{}, err
	}
	current := mapUser(row)
	if !isActive(current) || current.DeletionRequestedAt != nil || current.AuthVersion != actor.AuthVersion || current.ProfileConfirmationStatus == "PENDING" {
		return dbUser{}, newAuthError("oauth_actor_changed", 409, "The account changed; restart authorization")
	}
	return current, nil
}

func upsertOAuthIdentity(ctx context.Context, q *dbgen.Queries, userID string, provider OAuthProvider, identity oauthIdentity) error {
	id, err := parseUUID(userID)
	if err != nil {
		return err
	}
	existing, err := q.GetUserProviderIdentity(ctx, dbgen.GetUserProviderIdentityParams{UserID: id, Provider: string(provider)})
	if err == nil && existing.ProviderUserID != identity.Subject {
		return newAuthError(provider.slug()+"_already_bound", 409, "The account already has a different provider identity")
	}
	if err != nil && !isNotFound(err) {
		return err
	}
	_, err = q.UpsertOAuthIdentity(ctx, dbgen.UpsertOAuthIdentityParams{UserID: id, Provider: string(provider), ProviderUserID: identity.Subject, ProviderLogin: &identity.Login, Profile: identity.Profile})
	if isNotFound(err) {
		return newAuthError(provider.slug()+"_account_conflict", 409, "The provider identity is linked to another account")
	}
	return oauthIdentityError(provider, err)
}

func oauthIdentityError(provider OAuthProvider, err error) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		switch databaseError.ConstraintName {
		case "oauth_identities_user_provider_unique":
			return newAuthError(provider.slug()+"_already_bound", 409, "The account already has a provider identity")
		case "oauth_identities_provider_subject_unique":
			return newAuthError(provider.slug()+"_account_conflict", 409, "The provider identity is linked to another account")
		}
	}
	return identityError(err)
}

func availableOAuthUsername(ctx context.Context, q *dbgen.Queries, login, subject string) (string, error) {
	base := oauthUsername(login, subject)
	for attempt := 1; attempt <= 100; attempt++ {
		candidate := base
		if attempt > 1 {
			suffix := fmt.Sprintf("_%d", attempt)
			candidate = strings.TrimRight(base[:min(len(base), 32-len(suffix))], "_") + suffix
		}
		if _, err := q.GetUserByUsername(ctx, candidate); isNotFound(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", newAuthError("oauth_username_unavailable", 409, "A local username could not be allocated")
}

func (service *Service) OAuthUnbind(ctx context.Context, request *http.Request, provider OAuthProvider) (User, []string, error) {
	actor, err := service.Current(ctx, request)
	if err != nil {
		return User{}, nil, err
	}
	if !slices.Contains(actor.OAuthProviders, provider) {
		return User{}, nil, newAuthError(provider.slug()+"_not_bound", 404, "The provider identity is not linked")
	}
	tx, err := service.repo.pool.Begin(ctx)
	if err != nil {
		return User{}, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := service.repo.queries.WithTx(tx)
	if err := q.LockOAuthMutation(ctx); err != nil {
		return User{}, nil, err
	}
	current, err := lockOAuthActor(ctx, q, actor)
	if err != nil {
		return User{}, nil, err
	}
	if current.PasswordHash == nil {
		return User{}, nil, newAuthError("password_required", 409, "Set a password before unlinking an OAuth identity")
	}
	id, err := parseUUID(actor.ID)
	if err != nil {
		return User{}, nil, err
	}
	if err := q.UnlinkOAuthIdentity(ctx, dbgen.UnlinkOAuthIdentityParams{UserID: id, Provider: string(provider)}); err != nil {
		return User{}, nil, err
	}
	row, err := q.GetUserByID(ctx, id)
	if err != nil {
		return User{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, nil, err
	}
	access, refresh, err := service.issueSession(ctx, mapUser(row))
	if err != nil {
		return User{}, nil, err
	}
	user, err := service.toUser(ctx, mapUser(row))
	return user, []string{access, refresh}, err
}
