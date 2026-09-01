package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "heyblog-api/internal/database/gen"
)

type dbUser struct {
	ID           string
	Email        *string
	Username     string
	DisplayName  string
	PasswordHash *string
	AvatarURL    *string
	Role         string
	AccessStatus string
	VerifiedAt   *time.Time
	AuthVersion  int32
	CreatedAt    time.Time
	LastLoginAt  *time.Time
}

type dbOAuth struct {
	ID, UserID, ProviderID string
	ProviderLogin          *string
}

type repository struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
}

func newRepository(pool *pgxpool.Pool) *repository {
	return &repository{pool: pool, queries: dbgen.New(pool)}
}

func (repo *repository) userByID(ctx context.Context, id string) (dbUser, error) {
	parsed, err := parseUUID(id)
	if err != nil {
		return dbUser{}, err
	}
	user, err := repo.queries.GetUserByID(ctx, parsed)
	return mapUser(user), err
}

func (repo *repository) userByEmail(ctx context.Context, email string) (dbUser, error) {
	user, err := repo.queries.GetUserByEmail(ctx, email)
	return mapUser(user), err
}
func (repo *repository) userByUsername(ctx context.Context, username string) (dbUser, error) {
	user, err := repo.queries.GetUserByUsername(ctx, username)
	return mapUser(user), err
}

func (repo *repository) createUser(ctx context.Context, username, email, displayName string, passwordHash *string) (dbUser, error) {
	user, err := repo.queries.CreateUser(ctx, dbgen.CreateUserParams{Email: email, Username: username, DisplayName: displayName, PasswordHash: passwordHash})
	return mapUser(user), err
}

func (repo *repository) recordLogin(ctx context.Context, id string) error {
	parsed, err := parseUUID(id)
	if err != nil {
		return err
	}
	_, err = repo.queries.RecordUserLogin(ctx, parsed)
	return err
}
func (repo *repository) verifyEmail(ctx context.Context, id string) error {
	parsed, err := parseUUID(id)
	if err != nil {
		return err
	}
	return repo.queries.SetUserEmailVerified(ctx, parsed)
}
func (repo *repository) updatePassword(ctx context.Context, id, hash string) error {
	parsed, err := parseUUID(id)
	if err != nil {
		return err
	}
	return repo.queries.SetUserPassword(ctx, dbgen.SetUserPasswordParams{ID: parsed, PasswordHash: hash})
}

func (repo *repository) createVerificationCode(ctx context.Context, userID, email, hash string, expires time.Time) error {
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
	if err := queries.DeleteEmailVerificationCodes(ctx, parsed); err != nil {
		return err
	}
	if err := queries.CreateEmailVerificationCode(ctx, dbgen.CreateEmailVerificationCodeParams{UserID: parsed, Email: email, CodeHash: hash, ExpiresAt: timestamp(expires)}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repo *repository) verifyEmailWithCode(ctx context.Context, email, hash string, maxAttempts int) error {
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := repo.queries.WithTx(tx)
	row, err := queries.GetLatestEmailVerificationCode(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return newAuthError("invalid_verification_code", 400, "verification code is invalid")
	}
	if err != nil {
		return err
	}
	if !row.ExpiresAt.Valid || row.ExpiresAt.Time.Before(time.Now()) {
		return newAuthError("expired_verification_code", 400, "verification code has expired")
	}
	if int(row.AttemptCount) >= maxAttempts {
		return newAuthError("verification_attempts_exceeded", 400, "verification code is invalid")
	}
	if row.CodeHash != hash {
		if err := queries.IncrementEmailVerificationAttempts(ctx, row.ID); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return newAuthError("invalid_verification_code", 400, "verification code is invalid")
	}
	if err := queries.ConsumeEmailVerificationCode(ctx, row.ID); err != nil {
		return err
	}
	if err := queries.SetUserEmailVerified(ctx, row.UserID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repo *repository) createResetToken(ctx context.Context, userID, email, hash string, expires time.Time) error {
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
	if err := queries.DeletePasswordResetTokens(ctx, parsed); err != nil {
		return err
	}
	if err := queries.CreatePasswordResetToken(ctx, dbgen.CreatePasswordResetTokenParams{UserID: parsed, Email: email, TokenHash: hash, ExpiresAt: timestamp(expires)}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repo *repository) resetPasswordWithToken(ctx context.Context, tokenHash, passwordHash string) error {
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := repo.queries.WithTx(tx)
	row, err := queries.GetPasswordResetToken(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return newAuthError("invalid_password_reset_token", 400, "password reset token is invalid")
	}
	if err != nil {
		return err
	}
	if !row.ExpiresAt.Valid || row.ExpiresAt.Time.Before(time.Now()) {
		return newAuthError("expired_password_reset_token", 400, "password reset token has expired")
	}
	if err := queries.ConsumePasswordResetToken(ctx, row.ID); err != nil {
		return err
	}
	if err := queries.SetUserPassword(ctx, dbgen.SetUserPasswordParams{ID: row.UserID, PasswordHash: passwordHash}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func mapUser(row dbgen.IdentityUser) dbUser {
	var verified, lastLogin *time.Time
	if row.EmailVerifiedAt.Valid {
		verified = &row.EmailVerifiedAt.Time
	}
	if row.LastLoginAt.Valid {
		lastLogin = &row.LastLoginAt.Time
	}
	return dbUser{ID: row.ID.String(), Email: row.Email, Username: row.Username, DisplayName: row.DisplayName, PasswordHash: row.PasswordHash, Role: row.Role, AccessStatus: row.AccessStatus, VerifiedAt: verified, AuthVersion: row.AuthVersion, CreatedAt: row.CreatedAt.Time, LastLoginAt: lastLogin}
}
func parseUUID(value string) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(value); err != nil {
		return pgtype.UUID{}, fmt.Errorf("parse user id: %w", err)
	}
	return parsed, nil
}
func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
func isNotFound(err error) bool                         { return errors.Is(err, pgx.ErrNoRows) }
func repositoryError(operation string, err error) error { return fmt.Errorf("%s: %w", operation, err) }
