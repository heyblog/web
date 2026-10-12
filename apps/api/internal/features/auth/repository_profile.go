package auth

import (
	"context"
	"errors"

	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5/pgconn"
)

type profileMutation struct {
	AuthVersion                   int32
	UserID, Username, DisplayName string
	Confirm                       bool
}

func (repo *repository) createOAuthUser(ctx context.Context, username, email, name string) (dbUser, error) {
	row, err := repo.queries.CreateOAuthUser(ctx, dbgen.CreateOAuthUserParams{Email: &email, Username: username, DisplayName: name})
	return mapUser(row), identityError(err)
}

func (repo *repository) changeProfile(ctx context.Context, change profileMutation) (dbUser, bool, error) {
	id, err := parseUUID(change.UserID)
	if err != nil {
		return dbUser{}, false, err
	}
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return dbUser{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := repo.queries.WithTx(tx)
	row, err := queries.GetUserForUpdate(ctx, id)
	if err != nil {
		return dbUser{}, false, err
	}
	user := mapUser(row)
	if user.AuthVersion != change.AuthVersion {
		return dbUser{}, false, newAuthError("unauthenticated", 401, "authentication is required")
	}
	if !isActive(user) || user.DeletionRequestedAt != nil {
		return dbUser{}, false, newAuthError("account_unavailable", 409, "account is unavailable")
	}
	username, name := user.Username, user.DisplayName
	if change.Username != "" {
		username = change.Username
	}
	if change.DisplayName != "" {
		name = change.DisplayName
	}
	renamed := username != user.Username
	state, used := user.ProfileConfirmationStatus, user.UsernameChangesUsed
	if change.Confirm {
		if state != "PENDING" {
			if state == "CONFIRMED" && username == user.Username && name == user.DisplayName {
				return user, false, nil
			}
			return dbUser{}, false, newAuthError("profile_already_confirmed", 409, "profile confirmation is complete")
		}
		state = "CONFIRMED"
	} else {
		if state == "PENDING" {
			return dbUser{}, false, newAuthError("profile_confirmation_required", 403, "confirm your account profile to continue")
		}
		if renamed {
			if used >= 1 {
				return dbUser{}, false, newAuthError("username_changes_exhausted", 409, "username change allowance has been used")
			}
			used++
		}
	}
	if username == user.Username && name == user.DisplayName && state == user.ProfileConfirmationStatus {
		return user, false, nil
	}
	updated, err := queries.SetAccountProfile(ctx, dbgen.SetAccountProfileParams{ID: id, Username: username, DisplayName: name, ProfileConfirmationStatus: state, UsernameChangesUsed: used})
	if err != nil {
		return dbUser{}, false, identityError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return dbUser{}, false, err
	}
	return mapUser(updated), renamed, nil
}

func identityError(err error) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		switch databaseError.ConstraintName {
		case "users_username_unique_idx":
			return newAuthError("username_taken", 409, "username is already in use")
		case "users_email_unique_idx":
			return newAuthError("email_taken", 409, "email is already in use")
		}
	}
	return err
}
