//go:build integration

package integration_test

import (
	"reflect"
	"testing"

	"heyblog-api/internal/features/publicview"
	"heyblog-api/internal/features/taxonomy"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func TestTaxonomyCatalogCacheTracksCommittedInputs(t *testing.T) {
	f := newAuditMigrationFixture(t)
	ctx := t.Context()
	if _, err := f.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	service := taxonomy.NewService(f.pool, nil)
	before, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	read := func() taxonomy.Catalog {
		t.Helper()
		result, err := service.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	if !reflect.DeepEqual(before, read()) {
		t.Fatal("cache changed catalog")
	}
	// A caller cannot mutate the shared cache through returned slices.
	copy := read()
	copy.Tags[0].Name = "not persisted"
	if !reflect.DeepEqual(before, read()) {
		t.Fatal("returned catalog aliases cache")
	}
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "UPDATE directory.tags SET description='uncommitted' WHERE id=$1::uuid", before.Tags[0].ID); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, read()) {
		t.Fatal("uncommitted changes invalidated catalog")
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, read()) {
		t.Fatal("rollback changed catalog")
	}
	if _, err = f.pool.Exec(ctx, "UPDATE directory.tags SET description='committed' WHERE id=$1::uuid", before.Tags[0].ID); err != nil {
		t.Fatal(err)
	}
	after := read()
	if after.Revision == before.Revision || after.Tags[0].Description != "committed" {
		t.Fatal("committed change left stale catalog")
	}
	// Object insertion changes usage as well as the optimistic mutation revision.
	insertSite(ctx, t, f.pool, "CacHe1234", "Catalog cache", "catalog-cache.example.test")
	withSite := read()
	if withSite.Revision == after.Revision {
		t.Fatal("site insertion left stale revision")
	}
	fresh, err := taxonomy.NewService(f.pool, nil).List(ctx)
	if err != nil || !reflect.DeepEqual(withSite, fresh) {
		t.Fatalf("cached and fresh catalogs differ: %v", err)
	}
}

func TestDirectoryOptionsCacheTracksClassificationNamesAndCounts(t *testing.T) {
	f := newAuditMigrationFixture(t)
	ctx := t.Context()
	if _, err := f.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	service := publicview.New(dbgen.New(f.pool), nil)
	read := func() publicview.DirectoryOptions {
		t.Helper()
		options, err := service.DirectoryOptions(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return options
	}
	before := read()
	classification := before.Classifications[0]
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "UPDATE directory.tags SET name='缓存分类' WHERE id=$1::uuid", classification.ID); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, read()) {
		t.Fatal("options changed before commit")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	after := read()
	found := false
	for _, option := range after.Classifications {
		if option.ID == classification.ID {
			found = option.Label == "缓存分类"
		}
	}
	if !found {
		t.Fatal("committed classification name not returned")
	}
	insertSite(ctx, t, f.pool, "OpTio1234", "Directory options", "directory-options.example.test")
	if _, err := f.pool.Exec(ctx, `UPDATE directory.sites SET tag_cascade_id=(
	 SELECT id FROM directory.tag_cascades WHERE level1_tag_id=$1::uuid AND scope='SITE' LIMIT 1
	) WHERE short_id='OpTio1234'`, classification.ID); err != nil {
		t.Fatal(err)
	}
	withSite := read()
	for _, option := range withSite.Classifications {
		if option.ID == classification.ID && option.NormalCount != classification.NormalCount+1 {
			t.Fatalf("count = %d", option.NormalCount)
		}
	}
	fresh, err := publicview.New(dbgen.New(f.pool), nil).DirectoryOptions(ctx)
	if err != nil || !reflect.DeepEqual(withSite, fresh) {
		t.Fatalf("cached and fresh options differ: %v", err)
	}
}
