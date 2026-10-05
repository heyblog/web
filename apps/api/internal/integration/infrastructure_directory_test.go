//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func verifyDirectoryQueries(ctx context.Context, t *testing.T, connection *pgxpool.Pool) {
	t.Helper()
	queries := dbgen.New(connection)
	for index := range 26 {
		insertSite(
			ctx,
			t,
			connection,
			fmt.Sprintf("D%08d", index),
			fmt.Sprintf("Directory Fixture %02d", index),
			fmt.Sprintf("directory-%02d.example.com", index),
		)
	}

	filters := dbgen.CountDirectorySitesByStatusParams{
		QueryText: "directory fixture", Level1TagSlug: "", Level2TagSlug: "", TertiaryTagSlugs: []string{},
		WarningSlugs: []string{}, TechnologyNames: []string{}, AccessScopes: []string{"ALL"},
		FeedMode: "without",
	}
	counts, err := queries.CountDirectorySitesByStatus(ctx, filters)
	if err != nil {
		t.Fatalf("count directory fixtures: %v", err)
	}
	if counts.NormalCount != 26 || counts.AbnormalCount != 0 {
		t.Fatalf("directory fixture counts = %#v, want normal=26 abnormal=0", counts)
	}

	base := dbgen.ListDirectorySitesParams{
		SiteVisibility: "VISIBLE", QueryText: filters.QueryText,
		Level1TagSlug: filters.Level1TagSlug, Level2TagSlug: filters.Level2TagSlug,
		TertiaryTagSlugs: filters.TertiaryTagSlugs,
		WarningSlugs:     filters.WarningSlugs, TechnologyNames: filters.TechnologyNames,
		AccessScopes: filters.AccessScopes, FeedMode: filters.FeedMode,
		SortMode: "random", Seed: "site-directory:integration", SortOrder: "desc", PageLimit: 24,
	}
	firstPage, err := queries.ListDirectorySites(ctx, base)
	if err != nil {
		t.Fatalf("list first stable directory page: %v", err)
	}
	repeatedPage, err := queries.ListDirectorySites(ctx, base)
	if err != nil {
		t.Fatalf("repeat first stable directory page: %v", err)
	}
	if len(firstPage) != 24 || !slices.EqualFunc(firstPage, repeatedPage, func(left, right dbgen.DirectorySite) bool {
		return left.ID == right.ID
	}) {
		t.Fatalf("stable directory page changed between identical queries")
	}
	base.PageOffset = 24
	secondPage, err := queries.ListDirectorySites(ctx, base)
	if err != nil {
		t.Fatalf("list second stable directory page: %v", err)
	}
	if len(secondPage) != 2 {
		t.Fatalf("second directory page length = %d, want 2", len(secondPage))
	}
	seen := make(map[pgtype.UUID]struct{}, len(firstPage))
	for _, row := range firstPage {
		seen[row.ID] = struct{}{}
	}
	for _, row := range secondPage {
		if _, exists := seen[row.ID]; exists {
			t.Fatalf("stable directory pages repeated site %s", row.ShortID)
		}
	}

	cascades, err := queries.ListEnabledSiteTagCascades(ctx)
	if err != nil || len(cascades) < 2 {
		t.Fatalf("list directory cascade fixtures: %v / %d", err, len(cascades))
	}
	firstCascade := cascades[0]
	secondCascade := cascades[1]
	tertiaryOne, err := queries.CreateTag(ctx, dbgen.CreateTagParams{
		Name: "Directory Tertiary One", NormalizedName: "directory tertiary one",
		Slug: "directory-tertiary-one", Description: "integration fixture",
	})
	if err != nil {
		t.Fatalf("create first directory tertiary tag: %v", err)
	}
	tertiaryTwo, err := queries.CreateTag(ctx, dbgen.CreateTagParams{
		Name: "Directory Tertiary Two", NormalizedName: "directory tertiary two",
		Slug: "directory-tertiary-two", Description: "integration fixture",
	})
	if err != nil {
		t.Fatalf("create second directory tertiary tag: %v", err)
	}
	visibleBoth := insertSite(ctx, t, connection, "F00000001", "Role Fixture Visible Both", "role-visible-both.example.com")
	visiblePartial := insertSite(ctx, t, connection, "F00000002", "Role Fixture Visible Partial", "role-visible-partial.example.com")
	hiddenBoth := insertSite(ctx, t, connection, "F00000003", "Role Fixture Hidden Both", "role-hidden-both.example.com")
	removedBoth := insertSite(ctx, t, connection, "F00000004", "Role Fixture Removed Both", "role-removed-both.example.com")
	if _, err := connection.Exec(ctx, `UPDATE directory.sites SET visibility = 'HIDDEN', visibility_reason = 'fixture' WHERE id = $1`, hiddenBoth); err != nil {
		t.Fatalf("hide directory role fixture: %v", err)
	}
	if _, err := connection.Exec(ctx, `UPDATE directory.sites SET visibility = 'REMOVED', visibility_reason = 'fixture' WHERE id = $1`, removedBoth); err != nil {
		t.Fatalf("remove directory role fixture: %v", err)
	}
	for _, siteID := range []pgtype.UUID{visibleBoth, hiddenBoth, removedBoth} {
		if _, updateErr := connection.Exec(ctx, `UPDATE directory.sites SET tag_cascade_id = $2 WHERE id = $1`, siteID, firstCascade.ID); updateErr != nil {
			t.Fatalf("assign first directory cascade: %v", updateErr)
		}
	}
	if _, err := connection.Exec(ctx, `UPDATE directory.sites SET tag_cascade_id = $2 WHERE id = $1`, visiblePartial, secondCascade.ID); err != nil {
		t.Fatalf("assign second directory cascade: %v", err)
	}
	assign := func(siteID, tagID pgtype.UUID, position int16) {
		t.Helper()
		if _, assignErr := queries.AssignSiteTag(ctx, dbgen.AssignSiteTagParams{
			SiteID: siteID, TagID: tagID, Role: "TERTIARY", AssignmentSource: "SYSTEM", Position: &position,
		}); assignErr != nil {
			t.Fatalf("assign tertiary directory fixture: %v", assignErr)
		}
	}
	for _, siteID := range []pgtype.UUID{visibleBoth, hiddenBoth, removedBoth} {
		assign(siteID, tertiaryOne.ID, 1)
		assign(siteID, tertiaryTwo.ID, 2)
	}
	assign(visiblePartial, tertiaryOne.ID, 1)

	roleFilters := dbgen.CountDirectorySitesByStatusParams{
		QueryText: "role fixture", Level1TagSlug: firstCascade.Level1Slug,
		Level2TagSlug: firstCascade.Level2Slug, TertiaryTagSlugs: []string{tertiaryOne.Slug}, WarningSlugs: []string{},
		TechnologyNames: []string{}, AccessScopes: []string{}, FeedMode: "any",
	}
	roleCounts, err := queries.CountDirectorySitesByStatus(ctx, roleFilters)
	if err != nil {
		t.Fatalf("count cascade directory fixtures: %v", err)
	}
	if roleCounts.NormalCount != 1 || roleCounts.AbnormalCount != 1 {
		t.Fatalf("cascade status counts = %#v, want normal=1 abnormal=1", roleCounts)
	}
	randomSite, err := queries.PickRandomVisibleSite(ctx, dbgen.PickRandomVisibleSiteParams{
		Level1TagName: firstCascade.Level1Name,
		Level2TagName: firstCascade.Level2Name,
	})
	if err != nil || randomSite.Visibility != "VISIBLE" || randomSite.TagCascadeID != firstCascade.ID {
		t.Fatalf("random classified site = (%#v, %v), want visible first cascade", randomSite, err)
	}
	_, err = queries.PickRandomVisibleSite(ctx, dbgen.PickRandomVisibleSiteParams{
		Level1TagName: "不存在的分类",
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("random missing classification error = %v, want no rows", err)
	}
	roleFilters.TertiaryTagSlugs = []string{tertiaryOne.Slug, tertiaryTwo.Slug}
	roleCounts, err = queries.CountDirectorySitesByStatus(ctx, roleFilters)
	if err != nil {
		t.Fatalf("count tertiary AND directory fixtures: %v", err)
	}
	if roleCounts.NormalCount != 1 || roleCounts.AbnormalCount != 1 {
		t.Fatalf("secondary AND status counts = %#v, want normal=1 abnormal=1", roleCounts)
	}
	hiddenRows, err := queries.ListDirectorySites(ctx, dbgen.ListDirectorySitesParams{
		SiteVisibility: "HIDDEN", QueryText: roleFilters.QueryText,
		Level1TagSlug: roleFilters.Level1TagSlug, Level2TagSlug: roleFilters.Level2TagSlug,
		TertiaryTagSlugs: roleFilters.TertiaryTagSlugs,
		WarningSlugs:     []string{}, TechnologyNames: []string{}, AccessScopes: []string{},
		FeedMode: "any", SortMode: "joined", Seed: "integration", SortOrder: "desc", PageLimit: 24,
	})
	if err != nil {
		t.Fatalf("list hidden directory fixtures: %v", err)
	}
	if len(hiddenRows) != 1 || hiddenRows[0].ID != hiddenBoth {
		t.Fatalf("hidden directory rows = %#v, want only hidden fixture", hiddenRows)
	}
	optionRows, err := queries.ListDirectoryTagOptions(ctx)
	if err != nil {
		t.Fatalf("list directory tag options: %v", err)
	}
	var tertiaryOption *dbgen.ListDirectoryTagOptionsRow
	for index := range optionRows {
		row := &optionRows[index]
		if row.Slug == tertiaryTwo.Slug && row.Role == "TERTIARY" {
			tertiaryOption = row
		}
	}
	if tertiaryOption == nil || tertiaryOption.NormalCount != 1 || tertiaryOption.AbnormalCount != 1 {
		t.Fatalf("tertiary directory option = %#v, want normal=1 abnormal=1", tertiaryOption)
	}
}

func verifySiteAuditReviewDraftQueries(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	queries := dbgen.New(pool)
	reviewer, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email: "audit-reviewer@example.test", Username: "audit_reviewer", DisplayName: "Audit Reviewer",
	})
	if err != nil {
		t.Fatalf("create audit reviewer: %v", err)
	}
	audit, err := queries.CreateSiteAudit(ctx, dbgen.CreateSiteAuditParams{
		LookupSecretHash: make([]byte, 32), Action: "CREATE",
		ProposedSnapshot: []byte(`{"name":"Submitted"}`), RequestReason: "",
	})
	if err != nil {
		t.Fatalf("create review draft audit: %v", err)
	}
	saved, err := queries.SaveSiteAuditReviewDraft(ctx, dbgen.SaveSiteAuditReviewDraftParams{
		ReviewDraftSnapshot: []byte(`{"name":"Corrected"}`), ReviewDraftUpdatedBy: reviewer.ID,
		ID: audit.ID, ExpectedReviewDraftRevision: 0,
	})
	if err != nil {
		t.Fatalf("save review draft: %v", err)
	}
	var savedDraft map[string]string
	if err := json.Unmarshal(saved.ReviewDraftSnapshot, &savedDraft); err != nil {
		t.Fatalf("decode saved review draft: %v", err)
	}
	if saved.ReviewDraftRevision != 1 || savedDraft["name"] != "Corrected" {
		t.Fatalf("saved review draft = (revision:%d snapshot:%s)", saved.ReviewDraftRevision, saved.ReviewDraftSnapshot)
	}
	if _, err := queries.SaveSiteAuditReviewDraft(ctx, dbgen.SaveSiteAuditReviewDraftParams{
		ReviewDraftSnapshot: []byte(`{"name":"Stale"}`), ReviewDraftUpdatedBy: reviewer.ID,
		ID: audit.ID, ExpectedReviewDraftRevision: 0,
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("stale review draft error = %v, want pgx.ErrNoRows", err)
	}
	discarded, err := queries.DiscardSiteAuditReviewDraft(ctx, dbgen.DiscardSiteAuditReviewDraftParams{
		ReviewDraftUpdatedBy: reviewer.ID, ID: audit.ID, ExpectedReviewDraftRevision: 1,
	})
	if err != nil {
		t.Fatalf("discard review draft: %v", err)
	}
	if discarded.ReviewDraftSnapshot != nil || discarded.ReviewDraftRevision != 2 {
		t.Fatalf("discarded review draft = (revision:%d snapshot:%s)", discarded.ReviewDraftRevision, discarded.ReviewDraftSnapshot)
	}
}

func verifyDirectoryConstraints(ctx context.Context, t *testing.T, connection *pgxpool.Pool) {
	t.Helper()

	siteID := insertSite(ctx, t, connection, "0Aa1Bb2Cc", "Example Blog", "example.com")
	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_feeds (
			site_id, name, location_type, url_ref, url_key, is_default
		) VALUES ($1, 'Default', 'RELATIVE', '/feed.xml', '/feed.xml', true)
	`, siteID); err != nil {
		t.Fatalf("insert default feed: %v", err)
	}

	feedlessSiteID := insertSite(ctx, t, connection, "1Aa2Bb3Cc", "Feed Constraint", "feed.example.com")
	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_feeds (
			site_id, name, location_type, url_ref, url_key, is_default
		) VALUES ($1, 'Not Default', 'RELATIVE', '/feed.xml', '/feed.xml', false)
	`, feedlessSiteID); err == nil {
		t.Fatal("enabled feed without a default unexpectedly succeeded")
	}

	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.sites (short_id, custom_id, name, normalized_host)
		VALUES ('2Aa3Bb4Cc', 'bad--id', 'Invalid Custom ID', 'invalid-custom.example.com')
	`); err == nil {
		t.Fatal("invalid custom ID unexpectedly succeeded")
	}

	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.sites (short_id, name, normalized_host)
		VALUES ('3Aa4Bb5Cc', 'Duplicate Host', 'example.com')
	`); err == nil {
		t.Fatal("duplicate normalized host unexpectedly succeeded")
	}

}

func verifyPublicViewQueries(ctx context.Context, t *testing.T, connection *pgxpool.Pool) {
	t.Helper()
	queries := dbgen.New(connection)

	countBefore, err := queries.CountVisibleSites(ctx)
	if err != nil {
		t.Fatalf("count visible sites before fixture: %v", err)
	}
	visibleSiteID := insertSite(ctx, t, connection, "6Pv7Qw8Er", "Public Query", "public-query.example.com")
	hiddenSiteID := insertSite(ctx, t, connection, "7Pv8Qw9Er", "Hidden Query", "hidden-query.example.com")
	if _, err := connection.Exec(ctx, `
		UPDATE directory.sites
		   SET visibility = 'HIDDEN', visibility_reason = 'integration fixture'
		 WHERE id = $1
	`, hiddenSiteID); err != nil {
		t.Fatalf("hide public query fixture: %v", err)
	}
	countAfter, err := queries.CountVisibleSites(ctx)
	if err != nil {
		t.Fatalf("count visible sites after fixture: %v", err)
	}
	if countAfter != countBefore+1 {
		t.Fatalf("visible site count = %d, want %d", countAfter, countBefore+1)
	}
	randomSites, err := queries.ListRandomVisibleSites(ctx, int32(countAfter)) // #nosec G115 -- isolated fixtures keep the test count small.
	if err != nil {
		t.Fatalf("list random visible sites: %v", err)
	}
	var foundVisible bool
	for _, candidate := range randomSites {
		if candidate.ID == hiddenSiteID {
			t.Fatal("random visible sites included a hidden site")
		}
		if candidate.ID == visibleSiteID {
			foundVisible = true
		}
	}
	if !foundVisible {
		t.Fatal("random visible sites omitted a visible site within a full-size result")
	}

	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_feeds (
			site_id, name, location_type, url_ref, url_key, format, is_enabled, is_default
		) VALUES ($1, 'Public feed', 'RELATIVE', '/feed.xml', '/feed.xml', 'ATOM', true, true)
	`, visibleSiteID); err != nil {
		t.Fatalf("insert public feed fixture: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_feeds (
			site_id, name, location_type, url_ref, url_key, format, is_enabled, is_default
		) VALUES ($1, 'Disabled feed', 'RELATIVE', '/disabled.xml', '/disabled.xml', 'RSS', false, false)
	`, visibleSiteID); err != nil {
		t.Fatalf("insert disabled feed fixture: %v", err)
	}
	feeds, err := queries.ListPublicSiteFeeds(ctx, visibleSiteID)
	if err != nil {
		t.Fatalf("list public site feeds: %v", err)
	}
	if len(feeds) != 1 || feeds[0].Name != "Public feed" || !feeds[0].IsEnabled {
		t.Fatalf("public site feeds = %#v", feeds)
	}
	batchFeeds, err := queries.ListDefaultPublicSiteFeedsBySiteIDs(
		ctx,
		[]pgtype.UUID{visibleSiteID, hiddenSiteID},
	)
	if err != nil {
		t.Fatalf("list default public feeds by site IDs: %v", err)
	}
	if len(batchFeeds) != 1 || batchFeeds[0].SiteID != visibleSiteID || !batchFeeds[0].IsDefault {
		t.Fatalf("default public site feeds = %#v", batchFeeds)
	}

	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_resources (
			site_id, kind, location_type, url_ref, url_key
		) VALUES ($1, 'SITEMAP', 'RELATIVE', '/sitemap.xml', '/sitemap.xml')
	`, visibleSiteID); err != nil {
		t.Fatalf("insert public sitemap fixture: %v", err)
	}
	sitemaps, err := queries.ListPublicSitemapsBySiteIDs(ctx, []pgtype.UUID{visibleSiteID, hiddenSiteID})
	if err != nil {
		t.Fatalf("list public sitemaps by site IDs: %v", err)
	}
	if len(sitemaps) != 1 || sitemaps[0].SiteID != visibleSiteID || sitemaps[0].Kind != "SITEMAP" {
		t.Fatalf("public site sitemaps = %#v", sitemaps)
	}

	var enabledTagID, disabledTagID, mergedTagID, canonicalTagID pgtype.UUID
	for _, fixture := range []struct {
		name           string
		normalizedName string
		slug           string
		enabled        bool
		id             *pgtype.UUID
	}{
		{name: "Public Topic", normalizedName: "public topic", slug: "public-topic", enabled: true, id: &enabledTagID},
		{name: "Disabled Topic", normalizedName: "disabled topic", slug: "disabled-topic", enabled: false, id: &disabledTagID},
		{name: "Merged Topic", normalizedName: "merged topic", slug: "merged-topic", enabled: true, id: &mergedTagID},
		{name: "Canonical Topic", normalizedName: "canonical topic", slug: "canonical-topic", enabled: true, id: &canonicalTagID},
	} {
		if err := connection.QueryRow(ctx, `
			INSERT INTO directory.tags (name, normalized_name, slug, is_enabled)
			VALUES ($1, $2, $3, $4)
			RETURNING id
		`, fixture.name, fixture.normalizedName, fixture.slug, fixture.enabled).Scan(fixture.id); err != nil {
			t.Fatalf("insert tag fixture %q: %v", fixture.name, err)
		}
	}
	for _, tagID := range []pgtype.UUID{enabledTagID, disabledTagID, mergedTagID} {
		if _, err := connection.Exec(ctx, `
			INSERT INTO directory.site_tags (site_id, tag_id, role)
			VALUES ($1, $2, 'WARNING')
		`, visibleSiteID, tagID); err != nil {
			t.Fatalf("assign public view tag fixture: %v", err)
		}
	}
	if _, err := connection.Exec(ctx, `
		UPDATE directory.site_tags SET tag_id=$2 WHERE tag_id=$1
	`, mergedTagID, canonicalTagID); err != nil {
		t.Fatalf("merge public view tag fixture: %v", err)
	}
	tags, err := queries.ListPublicSiteTags(ctx, visibleSiteID)
	if err != nil {
		t.Fatalf("list public site tags: %v", err)
	}
	if len(tags) != 5 || !containsPublicTag(tags, visibleSiteID, disabledTagID, "WARNING") || !containsPublicTag(tags, visibleSiteID, enabledTagID, "WARNING") ||
		!containsPublicTagRole(tags, visibleSiteID, "PRIMARY") || !containsPublicTagRole(tags, visibleSiteID, "SECONDARY") {
		t.Fatalf("public site tags = %#v", tags)
	}
	batchTags, err := queries.ListPublicSiteTagsBySiteIDs(ctx, []pgtype.UUID{visibleSiteID, hiddenSiteID})
	if err != nil {
		t.Fatalf("list public site tags by site IDs: %v", err)
	}
	batchWarning, batchDisabled, batchPrimary, batchSecondary := false, false, false, false
	for _, row := range batchTags {
		batchWarning = batchWarning || row.SiteID == visibleSiteID && row.TagID == enabledTagID && row.Role == "WARNING"
		batchDisabled = batchDisabled || row.SiteID == visibleSiteID && row.TagID == disabledTagID && row.Role == "WARNING"
		batchPrimary = batchPrimary || row.SiteID == hiddenSiteID && row.Role == "PRIMARY"
		batchSecondary = batchSecondary || row.SiteID == hiddenSiteID && row.Role == "SECONDARY"
	}
	if len(batchTags) != 7 || !batchWarning || !batchDisabled || !batchPrimary || !batchSecondary {
		t.Fatalf("batch public site tags = %#v", batchTags)
	}

	var enabledComponentID, disabledComponentID pgtype.UUID
	for _, fixture := range []struct {
		name           string
		normalizedName string
		enabled        bool
		id             *pgtype.UUID
	}{
		{name: "Public Runtime", normalizedName: "public runtime", enabled: true, id: &enabledComponentID},
		{name: "Disabled Runtime", normalizedName: "disabled runtime", enabled: false, id: &disabledComponentID},
	} {
		if err := connection.QueryRow(ctx, `
			INSERT INTO directory.software_components (name, normalized_name, is_enabled)
			VALUES ($1, $2, $3)
			RETURNING id
		`, fixture.name, fixture.normalizedName, fixture.enabled).Scan(fixture.id); err != nil {
			t.Fatalf("insert software fixture %q: %v", fixture.name, err)
		}
	}
	for _, componentID := range []pgtype.UUID{enabledComponentID, disabledComponentID} {
		if _, err := connection.Exec(ctx, `
			INSERT INTO directory.site_software_components (
				site_id, component_id, role, evidence_source
			) VALUES ($1, $2, 'RUNTIME', 'MANUAL')
		`, visibleSiteID, componentID); err != nil {
			t.Fatalf("assign public view software fixture: %v", err)
		}
	}
	technologies, err := queries.ListPublicSiteSoftwareComponents(ctx, visibleSiteID)
	if err != nil {
		t.Fatalf("list public site software components: %v", err)
	}
	if len(technologies) != 1 || technologies[0].ComponentID != enabledComponentID || technologies[0].Name != "Public Runtime" {
		t.Fatalf("public site software components = %#v", technologies)
	}
}

func containsPublicTag(rows []dbgen.ListPublicSiteTagsRow, siteID, tagID pgtype.UUID, role string) bool {
	for _, row := range rows {
		if row.SiteID == siteID && row.TagID == tagID && row.Role == role {
			return true
		}
	}
	return false
}

func containsPublicTagRole(rows []dbgen.ListPublicSiteTagsRow, siteID pgtype.UUID, role string) bool {
	for _, row := range rows {
		if row.SiteID == siteID && row.Role == role {
			return true
		}
	}
	return false
}

func insertSite(
	ctx context.Context,
	t *testing.T,
	connection *pgxpool.Pool,
	shortID string,
	name string,
	host string,
) pgtype.UUID {
	t.Helper()
	var siteID pgtype.UUID
	if err := connection.QueryRow(ctx, `
		INSERT INTO directory.sites (short_id, name, normalized_host)
		VALUES ($1, $2, $3)
		RETURNING id
	`, shortID, name, host).Scan(&siteID); err != nil {
		t.Fatalf("insert site %q: %v", host, err)
	}
	return siteID
}
