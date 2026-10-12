//go:build integration

package integration_test

import (
	"reflect"
	"testing"

	"heyblog-api/internal/features/taxonomy"
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
