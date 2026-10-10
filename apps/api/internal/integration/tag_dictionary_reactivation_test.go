//go:build integration

package integration_test

import (
	"heyblog-api/internal/features/taxonomy"
	"testing"
)

func TestDictionaryReactivationReusesIdentityAndName(t *testing.T) {
	f := newAuditMigrationFixture(t)
	ctx := t.Context()
	if _, err := f.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	service := taxonomy.NewService(f.pool, nil)
	catalog, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err = service.Create(ctx, taxonomy.CreateInput{Name: "Reusable", ExpectedRevision: catalog.Revision})
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, tag := range catalog.Tags {
		if tag.Name == "Reusable" {
			id = tag.ID
		}
	}
	catalog, err = service.Update(ctx, id, taxonomy.UpdateInput{Name: "Reusable", Enabled: false, ExpectedRevision: catalog.Revision})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err = service.Create(ctx, taxonomy.CreateInput{Name: "  REUSABLE  ", ExpectedRevision: catalog.Revision})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tag := range catalog.Tags {
		if tag.ID == id {
			found = tag.Enabled && tag.Name == "Reusable"
		}
	}
	if !found {
		t.Fatal("canonical identity not reactivated")
	}
	if _, err = service.Create(ctx, taxonomy.CreateInput{Name: "reusable", ExpectedRevision: catalog.Revision}); err == nil {
		t.Fatal("enabled duplicate accepted")
	}
}
