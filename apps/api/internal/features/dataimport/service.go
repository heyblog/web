package dataimport

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrImportRunning         = errors.New("data import is already running")
	ErrDirectoryNotEmpty     = errors.New("directory is not empty")
	ErrInvalidBundle         = errors.New("invalid import bundle")
	ErrDependencyUnavailable = errors.New("import dependency is unavailable")
)

type Store interface {
	Import(context.Context, Plan) (Counts, error)
}

type taxonomyStore interface {
	ImportTaxonomy(context.Context, TagTaxonomyBundle) (Counts, error)
}

type Service struct {
	store           Store
	generateShortID func() (string, error)
	mutex           sync.Mutex
}

func NewService(store Store, generateShortID func() (string, error)) *Service {
	return &Service{store: store, generateShortID: generateShortID}
}

func (service *Service) Import(ctx context.Context, bundles Bundles) (Counts, error) {
	if err := validateImportMode(bundles); err != nil {
		return Counts{}, errors.Join(ErrInvalidBundle, err)
	}
	if !service.mutex.TryLock() {
		return Counts{}, ErrImportRunning
	}
	defer service.mutex.Unlock()
	if bundles.Taxonomy != nil {
		store, ok := service.store.(taxonomyStore)
		if !ok {
			return Counts{}, errors.Join(ErrDependencyUnavailable, errors.New("taxonomy import is unavailable"))
		}
		return store.ImportTaxonomy(ctx, *bundles.Taxonomy)
	}
	plan, err := BuildPlan(bundles, service.generateShortID)
	if err != nil {
		return Counts{}, errors.Join(ErrInvalidBundle, err)
	}
	if bundles.Mode == ImportIncremental {
		store, ok := service.store.(incrementalStore)
		if !ok {
			return Counts{}, errors.Join(ErrDependencyUnavailable, errors.New("incremental import is unavailable"))
		}
		return store.ImportIncremental(ctx, plan, service.generateShortID)
	}
	return service.store.Import(ctx, plan)
}
