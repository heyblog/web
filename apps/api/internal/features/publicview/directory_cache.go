package publicview

import (
	"context"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

// Only taxonomy rows share this generation. Software counts are read separately
// because component changes do not invalidate the taxonomy catalog.
type cachedDirectoryTags struct {
	revision int64
	tags     []dbgen.ListDirectoryTagOptionsRow
	cascades []dbgen.ListPublicSiteTagCascadesRow
}

func (service *Service) directoryTags(ctx context.Context) (*cachedDirectoryTags, error) {
	revision, err := service.options.GetDirectoryTaxonomyRevision(ctx)
	if err != nil {
		return nil, internalError(err, "read directory taxonomy generation")
	}
	if snapshot := service.optionsSnapshot.Load(); snapshot != nil && snapshot.revision == revision {
		return snapshot, nil
	}
	service.optionsOnce.Do(func() { service.optionsGate = make(chan struct{}, 1) })
	select {
	case service.optionsGate <- struct{}{}:
		defer func() { <-service.optionsGate }()
	case <-ctx.Done():
		return nil, internalError(ctx.Err(), "load directory taxonomy")
	}
	// Recheck after waiting for a refresh. A commit during the reads is observed
	// by the next request, including when it changes counts without changing tags.
	revision, err = service.options.GetDirectoryTaxonomyRevision(ctx)
	if err != nil {
		return nil, internalError(err, "read directory taxonomy generation")
	}
	if snapshot := service.optionsSnapshot.Load(); snapshot != nil && snapshot.revision == revision {
		return snapshot, nil
	}
	tags, err := service.options.ListDirectoryTagOptions(ctx)
	if err != nil {
		return nil, internalError(err, "list directory tag options")
	}
	cascades, err := service.options.ListPublicSiteTagCascades(ctx)
	if err != nil {
		return nil, internalError(err, "list directory classifications")
	}
	snapshot := &cachedDirectoryTags{revision: revision, tags: tags, cascades: cascades}
	service.optionsSnapshot.Store(snapshot)
	return snapshot, nil
}
