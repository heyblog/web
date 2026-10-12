package publicview

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type directoryCacheQueries struct {
	queryStub
	revision        atomic.Int64
	reads           atomic.Int64
	technologyReads atomic.Int64
	load            func(context.Context) ([]dbgen.ListDirectoryTagOptionsRow, error)
}

func (queries *directoryCacheQueries) GetDirectoryTaxonomyRevision(context.Context) (int64, error) {
	return queries.revision.Load(), nil
}

func (queries *directoryCacheQueries) ListDirectoryTagOptions(ctx context.Context) ([]dbgen.ListDirectoryTagOptionsRow, error) {
	queries.reads.Add(1)
	if queries.load != nil {
		return queries.load(ctx)
	}
	return []dbgen.ListDirectoryTagOptionsRow{{Name: "写作", Role: "TERTIARY", NormalCount: queries.revision.Load() + 1}}, nil
}

func (queries *directoryCacheQueries) ListDirectoryTechnologyOptions(context.Context) ([]dbgen.ListDirectoryTechnologyOptionsRow, error) {
	count := queries.technologyReads.Add(1)
	return []dbgen.ListDirectoryTechnologyOptionsRow{{Name: "Astro", NormalizedName: "astro", NormalCount: count}}, nil
}

func TestDirectoryOptionsReuseTaxonomyAndRefreshCommittedCounts(t *testing.T) {
	t.Parallel()
	queries := &directoryCacheQueries{}
	service := &Service{options: queries}
	first, err := service.DirectoryOptions(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	first.TertiaryTags[0].NormalCount = 999
	second, err := service.DirectoryOptions(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if second.TertiaryTags[0].NormalCount != 1 || queries.reads.Load() != 1 || second.Technologies[0].NormalCount != 2 {
		t.Fatalf("reused options = %#v, reads = %d", second, queries.reads.Load())
	}
	queries.revision.Add(1)
	updated, err := service.DirectoryOptions(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if updated.TertiaryTags[0].NormalCount != 2 || queries.reads.Load() != 2 {
		t.Fatalf("updated options = %#v", updated)
	}
}

func TestDirectoryOptionsConcurrentReadersShareRefresh(t *testing.T) {
	t.Parallel()
	queries := &directoryCacheQueries{}
	service := &Service{options: queries}
	var readers sync.WaitGroup
	for range 20 {
		readers.Go(func() {
			options, err := service.DirectoryOptions(t.Context())
			if err != nil {
				t.Error(err)
				return
			}
			if options.TertiaryTags[0].NormalCount != 1 {
				t.Errorf("options = %#v", options)
			}
		})
	}
	readers.Wait()
	if queries.reads.Load() != 1 {
		t.Fatalf("taxonomy reads = %d", queries.reads.Load())
	}
}

func TestDirectoryOptionsRetryFailedRefresh(t *testing.T) {
	t.Parallel()
	queries := &directoryCacheQueries{}
	service := &Service{options: queries}
	if _, err := service.DirectoryOptions(t.Context()); err != nil {
		t.Fatal(err)
	}
	queries.revision.Add(1)
	failure := errors.New("taxonomy unavailable")
	queries.load = func(context.Context) ([]dbgen.ListDirectoryTagOptionsRow, error) { return nil, failure }
	if _, err := service.DirectoryOptions(t.Context()); !errors.Is(err, failure) {
		t.Fatalf("refresh error = %v", err)
	}
	queries.load = nil
	options, err := service.DirectoryOptions(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if options.TertiaryTags[0].NormalCount != 2 {
		t.Fatalf("retried options = %#v", options)
	}
}

func TestDirectoryOptionsWaiterCanCancel(t *testing.T) {
	t.Parallel()
	started, release := make(chan struct{}), make(chan struct{})
	queries := &directoryCacheQueries{load: func(ctx context.Context) ([]dbgen.ListDirectoryTagOptionsRow, error) {
		close(started)
		select {
		case <-release:
			return []dbgen.ListDirectoryTagOptionsRow{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	service := &Service{options: queries}
	finished := make(chan error, 1)
	go func() { _, err := service.DirectoryOptions(t.Context()); finished <- err }()
	<-started
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := service.DirectoryOptions(ctx)
	close(release)
	if loadErr := <-finished; loadErr != nil {
		t.Fatal(loadErr)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("wait error = %v", err)
	}
}
