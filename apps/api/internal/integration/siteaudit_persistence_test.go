//go:build integration

package integration_test

import (
	"net/http"
	"testing"
	"time"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/monitoring"
	"heyblog-api/internal/features/siteaudit"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type auditPersistenceFixture struct {
	auditMigrationFixture
	service  *siteaudit.Service
	reviewer auth.User
	input    siteaudit.SubmissionInput
	site     dbgen.DirectorySite
}

func newAuditPersistenceFixture(t *testing.T) auditPersistenceFixture {
	t.Helper()
	f := auditPersistenceFixture{auditMigrationFixture: newAuditMigrationFixture(t)}
	ctx := t.Context()
	if _, err := f.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	queries := dbgen.New(f.pool)
	cascades, err := queries.ListEnabledSiteTagCascades(ctx)
	if err != nil || len(cascades) == 0 {
		t.Fatalf("load taxonomy: %v", err)
	}
	program, err := queries.GetSoftwareComponentByNormalizedName(ctx, "其他")
	if err != nil {
		t.Fatal(err)
	}
	user, err := queries.CreateUser(ctx, dbgen.CreateUserParams{Email: "persistence@example.test", Username: "persistence", DisplayName: "Reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	f.reviewer = auth.User{ID: integrationUUIDText(t, user.ID), Role: auth.RoleSysAdmin}
	f.input = siteauditConflictSubmission("https://persistence.example.test", siteauditConflictTaxonomy{
		Level1ID: integrationUUIDText(t, cascades[0].Level1ID), Level2ID: integrationUUIDText(t, cascades[0].Level2ID),
	}, integrationUUIDText(t, program.ID))
	f.input.Site.Feeds = []siteaudit.FeedInput{
		{Name: "First", URL: "/first.xml", Format: "RSS", IsDefault: true},
		{Name: "Second", URL: "/second.xml", Format: "RSS"},
		{Name: "Removed", URL: "/removed.xml", Format: "RSS"},
	}
	f.service = siteaudit.NewService(siteaudit.Dependencies{Repository: siteaudit.NewRepository(f.pool), NewShortID: func() (string, error) { return "Persist01", nil }})
	created, err := f.service.Submit(ctx, siteaudit.ActionCreate, "", f.input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.Review(ctx, f.reviewer, siteaudit.ReviewInput{AuditID: created.AuditID, Decision: siteaudit.DecisionApprove}); err != nil {
		t.Fatal(err)
	}
	f.site, err = queries.GetSiteByShortID(ctx, "Persist01")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestSiteAuditPreservesRSSHistoryForRetainedSources(t *testing.T) {
	// Given three sources with previously fetched articles and conditional validators.
	f := newAuditPersistenceFixture(t)
	ctx := t.Context()
	queries := dbgen.New(f.pool)
	before, err := queries.ListSiteFeeds(ctx, f.site.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, feed := range before {
		if _, err := f.pool.Exec(ctx, `INSERT INTO operations.feed_documents(feed_id,etag) VALUES($1,'"retained"')`, feed.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.pool.Exec(ctx, `INSERT INTO operations.feed_items(feed_id,item_key,title,url,published) VALUES($1,'old','Archived article','https://persistence.example.test/old','2020-01-01')`, feed.ID); err != nil {
			t.Fatal(err)
		}
	}
	f.input.Site.Summary = "New summary"
	f.input.Site.Feeds = f.input.Site.Feeds[:2]
	f.input.Site.Feeds[0].Name = "Renamed"
	f.input.Site.Feeds[0].IsDefault = false
	f.input.Site.Feeds[1].IsDefault = true
	pending, err := f.service.Submit(ctx, siteaudit.ActionUpdate, f.site.ShortID, f.input)
	if err != nil {
		t.Fatal(err)
	}

	// When an ordinary review renames retained sources, switches the default and removes one.
	if _, err := f.service.Review(ctx, f.reviewer, siteaudit.ReviewInput{AuditID: pending.AuditID, Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: f.site.Revision}); err != nil {
		t.Fatal(err)
	}

	// Then retained identities and articles survive; only the removed source is cascaded away.
	after, err := queries.ListSiteFeeds(ctx, f.site.ID)
	if err != nil || len(after) != 2 {
		t.Fatalf("feeds after review: %v / %v", after, err)
	}
	for _, original := range before {
		var articles, documents int
		if err := f.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM operations.feed_items WHERE feed_id=$1),(SELECT count(*) FROM operations.feed_documents WHERE feed_id=$1 AND etag='"retained"')`, original.ID).Scan(&articles, &documents); err != nil {
			t.Fatal(err)
		}
		want := 1
		if original.UrlKey == "/removed.xml" {
			want = 0
		}
		if articles != want || documents != want {
			t.Errorf("source %s lost retained data: articles=%d documents=%d want=%d", original.UrlKey, articles, documents, want)
		}
	}
	if after[0].UrlKey != "/second.xml" || !after[0].IsDefault {
		t.Fatalf("default source was not switched: %+v", after)
	}
}

func TestSiteAuditAssociationOnlyUpdatesFenceStaleRevisions(t *testing.T) {
	// Given an existing site whose only proposed change is feed metadata.
	f := newAuditPersistenceFixture(t)
	ctx := t.Context()
	f.input.Site.Feeds[0].Name = "Renamed feed"
	pending, err := f.service.Submit(ctx, siteaudit.ActionUpdate, f.site.ShortID, f.input)
	if err != nil {
		t.Fatal(err)
	}

	// When the association-only update is approved.
	approved, err := f.service.Review(ctx, f.reviewer, siteaudit.ReviewInput{AuditID: pending.AuditID, Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: f.site.Revision})
	if err != nil {
		t.Fatal(err)
	}

	// Then the content version advances once and later reviews reject the old version.
	current, err := dbgen.New(f.pool).GetSiteByShortID(ctx, f.site.ShortID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Revision != f.site.Revision+1 || approved.FinalSnapshot.Revision != current.Revision || !current.UpdatedAt.Time.After(f.site.UpdatedAt.Time) {
		t.Errorf("association edit did not advance content version: before=%d after=%d final=%d", f.site.Revision, current.Revision, approved.FinalSnapshot.Revision)
	}
	f.input.Site.Summary = "Next review"
	next, err := f.service.Submit(ctx, siteaudit.ActionUpdate, f.site.ShortID, f.input)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.service.Review(ctx, f.reviewer, siteaudit.ReviewInput{AuditID: next.AuditID, Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: f.site.Revision})
	assertSiteAuditServiceError(t, err, "site_revision_changed", http.StatusConflict)

	// Monitoring remains outside the content revision and timestamp contract.
	if err := monitoring.New(f.pool).Record(ctx, monitoring.Round{SiteID: integrationUUIDText(t, current.ID), Generation: current.MonitoringGeneration, ID: "after-review", StartedAt: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), CN: monitoring.OK, GL: monitoring.OK}); err != nil {
		t.Fatal(err)
	}
	monitored, err := dbgen.New(f.pool).GetSiteByShortID(ctx, f.site.ShortID)
	if err != nil || monitored.Revision != current.Revision || !monitored.UpdatedAt.Time.Equal(current.UpdatedAt.Time) {
		t.Fatalf("monitoring changed content metadata: %v / %v", monitored, err)
	}
}
