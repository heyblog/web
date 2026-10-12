package taxonomy

import (
	"context"
	"slices"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type cachedCatalog struct {
	generation int64
	value      Catalog
}

// List reuses the catalog only while every input to its revision is unchanged.
// The existing taxonomy lock serializes refreshes with authorized mutations.
func (s *Service) List(ctx context.Context) (Catalog, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Catalog{}, databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = dbgen.New(tx).LockTaxonomy(ctx); err != nil {
		return Catalog{}, databaseError(err)
	}
	generation, err := readCatalogGeneration(ctx, tx)
	if err != nil {
		return Catalog{}, databaseError(err)
	}
	s.catalogMu.Lock()
	cached := s.catalog
	s.catalogMu.Unlock()
	if cached == nil || cached.generation != generation {
		state, readErr := readGraph(ctx, tx)
		if readErr != nil {
			return Catalog{}, databaseError(readErr)
		}
		cached = &cachedCatalog{generation: generation, value: state.Catalog}
	}
	if err = tx.Commit(ctx); err != nil {
		return Catalog{}, databaseError(err)
	}
	s.catalogMu.Lock()
	s.catalog = cached
	s.catalogMu.Unlock()
	result := cached.value
	result.Tags = slices.Clone(result.Tags)
	for i := range result.Tags {
		result.Tags[i].Roles = slices.Clone(result.Tags[i].Roles)
	}
	result.Cascades = slices.Clone(result.Cascades)
	return result, nil
}
