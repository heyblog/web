package taskcenter

import (
	"context"
	"errors"
	"time"

	"heyblog-api/internal/platform/apperror"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db  taskDatabase
	now func() time.Time
}
type taskDatabase interface {
	Begin(context.Context) (pgx.Tx, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func NewRepository(pool *pgxpool.Pool, now func() time.Time) *Repository {
	return &Repository{db: pool, now: now}
}
func conflict() *apperror.Error {
	return apperror.New(apperror.KindConflict, "task_conflict", "task state changed; refresh and retry")
}
func invalid(err error) error {
	return apperror.New(apperror.KindValidation, "invalid_task", err.Error())
}
func (r *Repository) transaction(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func mutate[T any](ctx context.Context, r *Repository, request mutation[T], execute func(*Repository) (T, error)) (T, error) {
	var result T
	err := r.transaction(ctx, func(tx pgx.Tx) error {
		var err error
		result, err = idempotent(ctx, tx, request, func() (T, error) {
			return execute(&Repository{db: tx, now: r.now})
		})
		return err
	})
	return result, err
}

func taskLock(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(73921408)`)
	return err
}

func requireUpdated(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return conflict()
	}
	return nil
}

func noRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
