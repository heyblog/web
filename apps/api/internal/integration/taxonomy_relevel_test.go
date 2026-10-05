//go:build integration

package integration_test

import (
	"heyblog-api/internal/features/taxonomy"
	"testing"
)

func TestTaxonomyPathChangesAndPreviewConcurrency(t *testing.T) {
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
	var fallback taxonomy.Cascade
	for _, c := range catalog.Cascades {
		if c.Scope == "SITE" && c.Key == "other/topic-other-other" {
			fallback = c
		}
	}
	if fallback.PrimaryID != fallback.SecondaryID {
		t.Fatal("fallback must be a self relation")
	}
	catalog, err = service.Create(ctx, taxonomy.CreateInput{Name: "path-test", Slug: "path-test", ExpectedRevision: catalog.Revision})
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, tag := range catalog.Tags {
		if tag.Slug == "path-test" {
			id = tag.ID
		}
	}
	catalog, err = service.CreateCascade(ctx, taxonomy.CreateCascadeInput{Scope: "SITE", PrimaryID: id, SecondaryID: id, Key: "test/self", ExpectedRevision: catalog.Revision})
	if err != nil {
		t.Fatal(err)
	}
	var pathID string
	for _, c := range catalog.Cascades {
		if c.Key == "test/self" {
			pathID = c.ID
		}
	}
	input := taxonomy.ChangeInput{Kind: "path_merge", SourceID: pathID, TargetID: fallback.ID, ExpectedRevision: catalog.Revision}
	preview, err := service.Preview(ctx, input)
	if err != nil || len(preview.Blockers) > 0 {
		t.Fatal(err, preview.Blockers)
	}
	input.Fingerprint = preview.Fingerprint
	if _, err = f.pool.Exec(ctx, `INSERT INTO directory.sites(short_id,name,scheme,normalized_host,base_path,summary) VALUES('PreV12345','Concurrent','https','preview.example.test','/','')`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Apply(ctx, input); err == nil {
		t.Fatal("reference change did not invalidate preview")
	}
	catalog, err = service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	input.ExpectedRevision = catalog.Revision
	preview, err = service.Preview(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	input.Fingerprint = preview.Fingerprint
	catalog, err = service.Apply(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	preview, err = service.Preview(ctx, taxonomy.ChangeInput{Kind: "path_update", SourceID: fallback.ID, PrimaryID: id, SecondaryID: id, ExpectedRevision: catalog.Revision})
	if err != nil || len(preview.Blockers) != 1 || preview.Blockers[0] != "fallback_protected" {
		t.Fatal("fallback was mutable", err, preview)
	}
	catalog, err = service.Create(ctx, taxonomy.CreateInput{Name: "deletable", Slug: "deletable", ExpectedRevision: catalog.Revision})
	if err != nil {
		t.Fatal(err)
	}
	var disposable string
	for _, tag := range catalog.Tags {
		if tag.Slug == "deletable" {
			disposable = tag.ID
		}
	}
	if _, err = service.Delete(ctx, disposable, taxonomy.DeleteInput{ExpectedRevision: catalog.Revision}); err != nil {
		t.Fatal(err)
	}
}
