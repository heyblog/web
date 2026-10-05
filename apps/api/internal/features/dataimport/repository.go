package dataimport

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

const (
	importLockName            = "heyblog:data-import:v2"
	minimumImportLockCapacity = 512
	friendLinkBatchSize       = 1000
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) Import(ctx context.Context, plan Plan) (Counts, error) {
	if repository.pool == nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, errors.New("database pool is unavailable"))
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, fmt.Errorf("begin import transaction: %w", err))
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		rollbackContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackContext)
	}()

	queries := dbgen.New(tx)
	if err := queries.LockTaxonomy(ctx); err != nil {
		return Counts{}, err
	}
	locked, err := queries.TryAcquireImportLock(ctx, importLockName)
	if err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, fmt.Errorf("acquire import lock: %w", err))
	}
	if !locked {
		return Counts{}, ErrImportRunning
	}
	lockCapacity, err := queries.ImportLockCapacity(ctx)
	if err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, fmt.Errorf("check database lock capacity: %w", err))
	}
	if lockCapacity < minimumImportLockCapacity {
		return Counts{}, errors.Join(
			ErrDependencyUnavailable,
			fmt.Errorf("max_locks_per_transaction is %d; data import requires at least %d", lockCapacity, minimumImportLockCapacity),
		)
	}
	empty, err := queries.DirectoryIsEmpty(ctx)
	if err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, fmt.Errorf("check directory state: %w", err))
	}
	if !empty {
		return Counts{}, ErrDirectoryNotEmpty
	}
	if err := insertPlan(ctx, queries, plan); err != nil {
		return Counts{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, fmt.Errorf("commit import transaction: %w", err))
	}
	committed = true
	return plan.Counts(), nil
}
