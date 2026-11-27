//go:build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"heyblog-api/internal/domain/site"
	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/publicview"
	"heyblog-api/internal/features/siteaudit"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func TestSiteAnnotationWorkflow(t *testing.T) {
	fixture := newCredentialFixture(t)
	ctx := t.Context()
	queries := dbgen.New(fixture.pool)
	repo := siteaudit.NewRepository(fixture.pool)
	audits := siteaudit.NewService(siteaudit.Dependencies{Repository: repo, NewShortID: site.NewShortID})
	public := publicview.New(queries, nil)
	siteaudit.RegisterAnnotationRoutes(fixture.router.API, repo, fixture.auth, credentialWebToken)

	const shortID = "Annot0001"
	const host = "annotation.example.test"
	const root = "/api/web/management/site-annotations"
	const detailPath = root + "/" + shortID
	id := insertSite(ctx, t, fixture.pool, shortID, "Unpublished site name", host)
	source := insertBlogSite(ctx, t, fixture.pool, "Annot0002", "Incoming source", "incoming.example.test")
	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
		PSourceSiteID: source, PTargetUrl: "https://" + host + "/", PTargetHost: host, PStatus: "ACTIVE",
	}); err != nil {
		t.Fatal(err)
	}
	if res := fixture.request(http.MethodGet, root, ""); res.Code != 200 || !strings.Contains(res.Body.String(), shortID) {
		t.Fatalf("system administrator listing = %d %s", res.Code, res.Body)
	}
	if _, err := fixture.pool.Exec(ctx, "UPDATE identity.users SET role='ADMIN' WHERE id=$1", fixture.actor); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, "INSERT INTO identity.user_management_permissions(user_id,permission_key) VALUES($1,'site_audit.review')", fixture.actor); err != nil {
		t.Fatal(err)
	}
	login := func() {
		t.Helper()
		_, tokens, err := fixture.auth.Login(ctx, "credential_admin", "correct-password")
		if err != nil {
			t.Fatal(err)
		}
		fixture.access = tokens[0]
	}
	login()
	if res := fixture.request(http.MethodGet, detailPath, ""); res.Code != http.StatusForbidden {
		t.Fatalf("review-only annotation access = %d %s", res.Code, res.Body)
	}
	if _, err := fixture.pool.Exec(ctx, "UPDATE identity.user_management_permissions SET permission_key='site_annotation.manage' WHERE user_id=$1", fixture.actor); err != nil {
		t.Fatal(err)
	}

	login()

	cascades, err := queries.ListEnabledSiteTagCascades(ctx)
	if err != nil || len(cascades) == 0 {
		t.Fatalf("annotation cascades: %v", err)
	}
	program, err := queries.GetSoftwareComponentByNormalizedName(ctx, "其他")
	if err != nil {
		t.Fatal(err)
	}
	taxonomy := siteauditConflictTaxonomy{
		Level1ID: integrationUUIDText(t, cascades[0].Level1ID), Level2ID: integrationUUIDText(t, cascades[0].Level2ID),
	}
	programID := integrationUUIDText(t, program.ID)
	application := siteauditConflictSubmission("https://"+host+"/", taxonomy, programID)
	availability, err := audits.CheckSiteAvailability(ctx, application.Site.URL)
	if err != nil || !availability.Available || availability.ExistingSite != nil {
		t.Fatalf("recorded site was exposed by address availability: %#v, %v", availability, err)
	}
	submitted, err := audits.Submit(ctx, siteaudit.ActionCreate, "", application)
	if err != nil || submitted.Action != siteaudit.ActionCreate || submitted.ShortID != "" {
		t.Fatalf("existing-host application = %#v, %v", submitted, err)
	}
	lookup, err := audits.Query(ctx, submitted.LookupToken)
	if err != nil || lookup.Action != siteaudit.ActionCreate || lookup.ShortID != "" {
		t.Fatalf("lookup exposed recorded site identity: %#v, %v", lookup, err)
	}

	var detail siteaudit.AnnotationDetail
	res := fixture.request(http.MethodGet, detailPath, "")
	if res.Code != http.StatusOK || res.Header().Get("Cache-Control") != "private, no-store" ||
		json.Unmarshal(res.Body.Bytes(), &detail) != nil || detail.BlogStatus != site.BlogUnreviewed ||
		detail.PendingAuditID != submitted.AuditID || len(detail.Incoming) != 1 || detail.Incoming[0].SourceShortID != "Annot0002" {
		t.Fatalf("annotation detail = %d %s", res.Code, res.Body)
	}
	name, excluded, cascadeID := "Annotated directory entry", site.BlogExcluded, integrationUUIDText(t, cascades[0].ID)
	patch := siteaudit.AnnotationPatch{
		ExpectedRevision: detail.Revision, FriendVersion: detail.FriendVersion, Name: &name,
		BlogStatus: &excluded, TagCascadeID: &cascadeID, Reason: "站点类型与资料核查",
		Components: []siteaudit.AnnotationSelection{{ID: programID, Role: "SITE_PROGRAM"}},
		Feeds: []siteaudit.AnnotationFeed{
			{FeedInput: siteaudit.FeedInput{Name: "RSS", URL: "https://" + host + "/feed.xml", Format: "RSS", IsDefault: true}, IsEnabled: true},
			{FeedInput: siteaudit.FeedInput{Name: "Archived feed", URL: "https://" + host + "/archived.xml", Format: "RSS"}},
		},
		Resources: []siteaudit.ResourceInput{{Kind: "SITEMAP", URL: "https://" + host + "/sitemap.xml"}},
		Outgoing:  []siteaudit.AnnotationFriendInput{{TargetURL: "https://external.example.test/", LinkStatus: "ACTIVE"}},
	}
	save := func(request siteaudit.AnnotationPatch) *http.Response {
		t.Helper()
		body, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		return fixture.request(http.MethodPatch, detailPath, string(body)).Result()
	}
	response := save(patch)
	defer func() { _ = response.Body.Close() }()
	if err := json.NewDecoder(response.Body).Decode(&detail); response.StatusCode != http.StatusOK || err != nil {
		t.Fatalf("annotation save = %d, %v", response.StatusCode, err)
	}
	if detail.BlogStatus != excluded || detail.Name != name || detail.Revision <= patch.ExpectedRevision || len(detail.Tags) < 2 ||
		len(detail.Components) != 1 || len(detail.Feeds) != 2 || len(detail.Resources) != 1 || len(detail.Outgoing) != 1 {
		t.Fatalf("saved data missing: %#v", detail)
	}
	history := fixture.request(http.MethodGet, detailPath+"/history", "")
	var entries struct {
		Items []siteaudit.AnnotationHistory `json:"items"`
	}
	if err := json.Unmarshal(history.Body.Bytes(), &entries); history.Code != 200 || err != nil || len(entries.Items) != 1 ||
		entries.Items[0].ActorName == "" || entries.Items[0].Revision != detail.Revision ||
		!strings.Contains(string(entries.Items[0].Before), "UNREVIEWED") || !strings.Contains(string(entries.Items[0].After), "NON_BLOG") {
		t.Fatalf("annotation history: %d %s", history.Code, history.Body)
	}
	response = save(patch)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("stale annotation status = %d", response.StatusCode)
	}

	reviewer := auth.User{ID: fixture.actor, Role: auth.RoleSysAdmin}
	_, err = audits.Review(ctx, reviewer, siteaudit.ReviewInput{
		AuditID: submitted.AuditID, Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: detail.Revision,
	})
	assertSiteAuditServiceError(t, err, "site_classified_after_submission", http.StatusConflict)
	identifier := publicview.SiteIdentifier{Kind: publicview.IdentifierShortID, Value: shortID}
	if _, err := public.SiteGraphByIdentifier(ctx, identifier); err == nil {
		t.Fatal("non-blog returned a public graph")
	}
	_, err = audits.Review(ctx, reviewer, siteaudit.ReviewInput{
		AuditID: submitted.AuditID, Decision: siteaudit.DecisionReject, ReviewerComment: "站点已标注",
	})
	if err != nil {
		t.Fatal(err)
	}
	later, err := audits.Submit(ctx, siteaudit.ActionCreate, "", application)
	if err != nil {
		t.Fatal(err)
	}
	reviewed, err := audits.Review(ctx, reviewer, siteaudit.ReviewInput{
		AuditID: later.AuditID, Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: detail.Revision,
	})
	if err != nil || reviewed.SiteID != id.String() {
		t.Fatalf("later application changed canonical identity: %s, %v", reviewed.SiteID, err)
	}
	if _, err := public.SiteGraphByIdentifier(ctx, identifier); err != nil {
		t.Fatalf("approved blog missing from graph: %v", err)
	}
	feeds, err := queries.ListSiteFeeds(ctx, id)
	if err != nil || len(feeds) != 1 || feeds[0].Name != "Archived feed" || feeds[0].IsEnabled {
		t.Fatalf("approval lost the disabled feed: %#v, %v", feeds, err)
	}
}

func TestSiteAnnotationClassificationBackfill(t *testing.T) {
	fixture := newAuditMigrationFixture(t)
	ctx := t.Context()
	if _, err := fixture.provider.UpTo(ctx, 16); err != nil {
		t.Fatal(err)
	}
	pending := fixture.pendingCreate(t, "pending-annotation.example.test")
	fixture.addReviewHistory(t, pending.AuditID)
	createSite := insertSite(ctx, t, fixture.pool, "AnnotCr01", "Historical create", "historical-create.example.test")
	insertSite(ctx, t, fixture.pool, "AnnotNo01", "Directory only", "directory-only.example.test")
	if _, err := fixture.pool.Exec(ctx, `
		INSERT INTO directory.site_audits
			(lookup_secret_hash, action, status, site_id, proposed_snapshot, final_snapshot, request_reason, reviewed_by, reviewed_at)
		SELECT decode(repeat('cd', 32), 'hex'), 'CREATE', 'APPROVED', $2,
		       proposed_snapshot, proposed_snapshot, '', review_draft_updated_by, now()
		FROM directory.site_audits WHERE id=$1
	`, pending.AuditID, createSite); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	queries := dbgen.New(fixture.pool)
	for shortID, expected := range map[string]site.BlogStatus{"4Fjw77W5U": site.BlogConfirmed, "AnnotCr01": site.BlogConfirmed, "AnnotNo01": site.BlogUnreviewed} {
		record, err := queries.GetSiteByShortID(ctx, shortID)
		if err != nil || site.BlogStatus(record.BlogStatus) != expected || record.Revision != 1 {
			t.Errorf("backfill %s = (%q, revision %d), %v", shortID, record.BlogStatus, record.Revision, err)
		}
	}
	if _, err := fixture.provider.Down(ctx); err == nil || !strings.Contains(err.Error(), "restore the previous database backup") {
		t.Fatalf("downgrade must refuse to discard classifications: %v", err)
	}
}
