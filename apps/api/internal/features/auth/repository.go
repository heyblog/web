package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type dbUser struct {
	ProfileConfirmationStatus string
	UsernameChangesUsed       int32
	UpdatedAt                 time.Time
	DeletionRequestedAt       *time.Time
	ID                        string
	Email                     *string
	Username                  string
	DisplayName               string
	PasswordHash              *string
	AvatarURL                 *string
	Role                      string
	AccessStatus              string
	VerifiedAt                *time.Time
	AuthVersion               int32
	CreatedAt                 time.Time
	LastLoginAt               *time.Time
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

func (repo *repository) recordLogin(ctx context.Context, id string) (dbUser, error) {
	parsed, err := parseUUID(id)
	if err != nil {
		return dbUser{}, err
	}
	row, err := repo.queries.RecordUserLogin(ctx, parsed)
	return mapUser(row), err
}
func (repo *repository) updatePassword(ctx context.Context, id, hash string) error {
	parsed, err := parseUUID(id)
	if err != nil {
		return err
	}
	return repo.queries.SetUserPassword(ctx, dbgen.SetUserPasswordParams{ID: parsed, PasswordHash: hash})
}

func mapUser(row dbgen.IdentityUser) dbUser {
	var verified, lastLogin *time.Time
	if row.EmailVerifiedAt.Valid {
		verified = &row.EmailVerifiedAt.Time
	}
	if row.LastLoginAt.Valid {
		lastLogin = &row.LastLoginAt.Time
	}
	var deletion *time.Time
	if row.DeletionRequestedAt.Valid {
		deletion = &row.DeletionRequestedAt.Time
	}
	return dbUser{ProfileConfirmationStatus: row.ProfileConfirmationStatus, UsernameChangesUsed: row.UsernameChangesUsed, UpdatedAt: row.UpdatedAt.Time, DeletionRequestedAt: deletion, ID: row.ID.String(), Email: row.Email, Username: row.Username, DisplayName: row.DisplayName, PasswordHash: row.PasswordHash, Role: row.Role, AccessStatus: row.AccessStatus, VerifiedAt: verified, AuthVersion: row.AuthVersion, CreatedAt: row.CreatedAt.Time, LastLoginAt: lastLogin}
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
