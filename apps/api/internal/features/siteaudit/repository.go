package siteaudit

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type Repository struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: dbgen.New(pool)}
}

func (repository *Repository) InTransaction(
	ctx context.Context,
	operation func(AuditTransaction) error,
) error {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin site audit transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if err := dbgen.New(transaction).LockTaxonomy(ctx); err != nil {
		return fmt.Errorf("lock audit taxonomy: %w", err)
	}
	if err := operation(&auditTransaction{queries: repository.queries.WithTx(transaction)}); err != nil {
		return err
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit site audit transaction: %w", err)
	}
	return nil
}
