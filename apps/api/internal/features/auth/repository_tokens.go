package auth

import (
	"context"
	"errors"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

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
	if err := lockTokenAccount(ctx, queries, parsed, email); err != nil {
		return err
	}
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
	if err := lockTokenAccount(ctx, queries, row.UserID, row.Email); err != nil {
		return err
	}
	row, err = queries.LockEmailVerificationCode(ctx, row.ID)
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
	if err := lockTokenAccount(ctx, queries, parsed, email); err != nil {
		return err
	}
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
	if err := lockTokenAccount(ctx, queries, row.UserID, row.Email); err != nil {
		return err
	}
	row, err = queries.LockPasswordResetToken(ctx, row.ID)
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
	if err := queries.SetUserEmailVerified(ctx, row.UserID); err != nil {
		return err
	}
	if err := queries.SetUserPassword(ctx, dbgen.SetUserPasswordParams{ID: row.UserID, PasswordHash: passwordHash}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func lockTokenAccount(ctx context.Context, queries *dbgen.Queries, id pgtype.UUID, email string) error {
	row, err := queries.GetUserForUpdate(ctx, id)
	if err != nil {
		return err
	}
	user := mapUser(row)
	if !isActive(user) || user.DeletionRequestedAt != nil || *user.Email != email {
		return newAuthError("account_token_invalid", 400, "account email or access state changed")
	}
	return nil
}
