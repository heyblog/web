//go:build integration

package integration_test

import (
	"context"
	"heyblog-api/internal/features/taxonomy"
	"slices"
	"testing"
)

func verifyTaxonomyAuditReferences(ctx context.Context, t *testing.T, f auditMigrationFixture, service *taxonomy.Service) {
	t.Helper()
	pending := f.pendingCreate(t, "taxonomy-pending.example.test")
	var frozen string
	if err := f.pool.QueryRow(ctx, `SELECT proposed_snapshot::text FROM directory.site_audits WHERE id=$1::uuid`, pending.AuditID).Scan(&frozen); err != nil {
		t.Fatal(err)
	}
	catalog, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Renaming an existing classifier must never rewrite submission JSON.
	for _, tag := range catalog.Tags {
		if slices.Contains(tag.Roles, "PRIMARY") && tag.Enabled {
			_, err = service.Update(ctx, tag.ID, taxonomy.UpdateInput{Name: tag.Name + " renamed", Enabled: true, ExpectedRevision: catalog.Revision})
			if err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	var after string
	if err := f.pool.QueryRow(ctx, `SELECT proposed_snapshot::text FROM directory.site_audits WHERE id=$1::uuid`, pending.AuditID).Scan(&after); err != nil || after != frozen {
		t.Fatal("original audit JSON changed", err)
	}
}
