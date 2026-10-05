package dataimport

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func (repository *Repository) ImportIncremental(ctx context.Context, plan Plan, generate func() (string, error)) (Counts, error) {
	if repository.pool == nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, errors.New("database pool is unavailable"))
	}
	if err := validatePlan(plan); err != nil {
		return Counts{}, errors.Join(ErrInvalidBundle, err)
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, fmt.Errorf("begin incremental import: %w", err))
	}
	defer rollbackIncremental(tx)
	queries := dbgen.New(tx)
	if err := queries.LockTaxonomy(ctx); err != nil {
		return Counts{}, fmt.Errorf("acquire incremental taxonomy lock: %w", err)
	}
	if err := lockIncremental(ctx, queries, plan); err != nil {
		return Counts{}, err
	}
	stored, err := queries.ListIncrementalSites(ctx)
	if err != nil {
		return Counts{}, fmt.Errorf("read registered import sites: %w", err)
	}
	pairs, err := queries.ListIncrementalFriendPairs(ctx)
	if err != nil {
		return Counts{}, fmt.Errorf("read existing import edges: %w", err)
	}
	resolved, counts, err := resolveIncremental(plan, stored, pairs, generate)
	if err != nil {
		return Counts{}, errors.Join(ErrInvalidBundle, err)
	}
	if err := insertProfiles(ctx, queries, resolved); err != nil {
		return Counts{}, err
	}
	if err := insertIncrementalOrigins(ctx, queries, resolved, &counts); err != nil {
		return Counts{}, err
	}
	if err := insertFriendLinks(ctx, queries, resolved); err != nil {
		return Counts{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, fmt.Errorf("commit incremental import: %w", err))
	}
	return counts, nil
}

func rollbackIncremental(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}

func lockIncremental(ctx context.Context, queries *dbgen.Queries, plan Plan) error {
	locked, err := queries.TryAcquireImportLock(ctx, importLockName)
	if err != nil {
		return fmt.Errorf("acquire incremental import lock: %w", err)
	}
	if !locked {
		return ErrImportRunning
	}
	capacity, err := queries.ImportLockCapacity(ctx)
	if err != nil || capacity < minimumImportLockCapacity {
		return errors.Join(ErrDependencyUnavailable, err, fmt.Errorf("incremental import requires lock capacity of at least %d", minimumImportLockCapacity))
	}
	if err := queries.LockIncrementalSites(ctx); err != nil {
		return fmt.Errorf("lock incremental import sites: %w", err)
	}
	hosts := make([]string, 0, len(plan.Sites))
	for _, row := range plan.Sites {
		hosts = append(hosts, row.NormalizedHost)
	}
	sort.Strings(hosts)
	if err := queries.LockIncrementalHosts(ctx, hosts); err != nil {
		return fmt.Errorf("lock incremental import hosts: %w", err)
	}
	return nil
}

func insertIncrementalOrigins(ctx context.Context, queries *dbgen.Queries, plan Plan, counts *Counts) error {
	for _, source := range plan.Sources {
		inserted, err := queries.InsertIncrementalSource(ctx, dbgen.InsertIncrementalSourceParams{SourceKey: source.Key, Name: source.Name})
		if err != nil {
			return fmt.Errorf("insert incremental source: %w", err)
		}
		counts.Sources += int(inserted)
		stored, err := queries.GetIncrementalSource(ctx, source.Key)
		if err != nil {
			return fmt.Errorf("resolve incremental source: %w", err)
		}
		if !stored.IsEnabled {
			return errors.Join(ErrInvalidBundle, errors.New("incremental provenance source is disabled"))
		}
		for _, origin := range plan.Origins {
			if origin.SourceKey != source.Key {
				continue
			}
			inserted, err := queries.InsertIncrementalOrigin(ctx, dbgen.InsertIncrementalOriginParams{
				SiteID: mustUUID(origin.SiteID), SourceID: stored.ID,
				ExternalReference: nullableText(origin.ExternalReference), FirstDiscoveredAt: timestamp(origin.FirstDiscoveredAt), Metadata: origin.Metadata,
			})
			if err != nil {
				return fmt.Errorf("insert incremental origin: %w", err)
			}
			counts.Origins += int(inserted)
		}
	}
	return nil
}
