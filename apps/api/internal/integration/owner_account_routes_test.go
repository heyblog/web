//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"heyblog-api/internal/domain/site"
	"heyblog-api/internal/features/siteaudit"
	"heyblog-api/internal/features/sitemanagement"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type publishedProof struct{}

func (publishedProof) Verify(context.Context, sitemanagement.Claim) error { return nil }

func TestOwnerAccountHTTPWorkflow(t *testing.T) {
	fixture := newCredentialFixture(t)
	ctx := t.Context()
	repository := siteaudit.NewRepository(fixture.pool)
	audits := siteaudit.NewService(siteaudit.Dependencies{Repository: repository, Auth: fixture.auth, NewShortID: site.NewShortID})
	if err := siteaudit.RegisterRoutes(fixture.router.API, audits, credentialWebToken, fixture.redis); err != nil {
		t.Fatal(err)
	}
	siteaudit.RegisterAccountRoutes(fixture.router.API, siteaudit.NewAccountService(repository, audits), credentialWebToken, fixture.redis)
	claims := sitemanagement.NewService(sitemanagement.NewRepository(fixture.pool), fixture.auth, publishedProof{})
	if err := sitemanagement.RegisterRoutes(fixture.router.API, claims, credentialWebToken, fixture.redis); err != nil {
		t.Fatal(err)
	}
	insertSite(ctx, t, fixture.pool, "HTTPown01", "HTTP owned site", "http-owner.example.test")
	insertSite(ctx, t, fixture.pool, "HTTPfri01", "HTTP friend", "http-friend.example.test")
	missingToken := httptest.NewRecorder()
	fixture.router.ServeHTTP(missingToken, httptest.NewRequest("GET", "/api/web/account/sites", nil))
	if missingToken.Code != 401 {
		t.Fatalf("missing web token = %d", missingToken.Code)
	}
	request := httptest.NewRequest("GET", "/api/web/account/sites", nil)
	request.Header.Set("X-HeyBlog-Web-Token", credentialWebToken)
	missingSession := httptest.NewRecorder()
	fixture.router.ServeHTTP(missingSession, request)
	if missingSession.Code != 401 {
		t.Fatalf("missing session = %d", missingSession.Code)
	}
	forbidden := fixture.request("PUT", "/api/web/account/sites/HTTPown01/friend-links/HTTPfri01", "")
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("unverified edit = %d / %s", forbidden.Code, forbidden.Body.String())
	}
	created := fixture.request("POST", "/api/web/account/site-claims", `{"short_id":"HTTPown01","method":"META"}`)
	if created.Code != http.StatusOK {
		t.Fatalf("create certification = %d / %s", created.Code, created.Body.String())
	}
	var challenge sitemanagement.CreateResult
	if err := json.Unmarshal(created.Body.Bytes(), &challenge); err != nil || challenge.Token == "" {
		t.Fatalf("challenge = %#v / %v", challenge, err)
	}
	if strings.Contains(created.Body.String(), "token_hash") {
		t.Fatal("claim DTO disclosed stored digest")
	}
	checked := fixture.request("POST", "/api/web/account/site-claims/"+challenge.Claim.ID+"/check", "")
	if checked.Code != http.StatusOK {
		t.Fatalf("check certification = %d / %s", checked.Code, checked.Body.String())
	}
	replay := fixture.request("POST", "/api/web/account/site-claims/"+challenge.Claim.ID+"/check", "")
	if replay.Code != http.StatusConflict {
		t.Fatalf("replay check = %d / %s", replay.Code, replay.Body.String())
	}
	listed := fixture.request("GET", "/api/web/account/sites", "")
	if listed.Code != 200 || !strings.Contains(listed.Body.String(), "HTTPown01") || listed.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("owned site list = %d / %s", listed.Code, listed.Body.String())
	}
	linked := fixture.request("PUT", "/api/web/account/sites/HTTPown01/friend-links/HTTPfri01", "")
	if linked.Code != 200 {
		t.Fatalf("add friend = %d / %s", linked.Code, linked.Body.String())
	}
	friends := fixture.request("GET", "/api/web/account/sites/HTTPown01/friend-links", "")
	if friends.Code != 200 || !strings.Contains(friends.Body.String(), "HTTPfri01") {
		t.Fatalf("read friend = %d / %s", friends.Code, friends.Body.String())
	}
	removed := fixture.request("DELETE", "/api/web/account/sites/HTTPown01/friend-links/by-host/http-friend.example.test", "")
	if removed.Code != http.StatusOK {
		t.Fatalf("remove friend by host = %d / %s", removed.Code, removed.Body.String())
	}
	queries := dbgen.New(fixture.pool)
	cascades, err := queries.ListEnabledSiteTagCascades(ctx)
	if err != nil || len(cascades) == 0 {
		t.Fatal(err)
	}
	program, err := queries.GetSoftwareComponentByNormalizedName(ctx, "其他")
	if err != nil {
		t.Fatal(err)
	}
	input := siteauditConflictSubmission("https://http-owner.example.test/", siteauditConflictTaxonomy{Level1ID: integrationUUIDText(t, cascades[0].Level1ID), Level2ID: integrationUUIDText(t, cascades[0].Level2ID)}, integrationUUIDText(t, program.ID))
	input.Site.Name = "Updated HTTP owner"
	if _, err := fixture.pool.Exec(ctx, "UPDATE directory.sites SET access_scope = 'CN_ONLY' WHERE short_id = 'HTTPown01'"); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/web/account/site-submissions", "/api/web/account/sites/HTTPown01/updates", "/api/web/account/sites/HTTPown01/friend-link-submissions"} {
		attemptedScope := strings.Replace(string(body), `"site":{`, `"site":{"access_scope":"ALL",`, 1)
		rejected := fixture.request("POST", path, attemptedScope)
		if rejected.Code != http.StatusUnprocessableEntity || !strings.Contains(rejected.Body.String(), "access_scope") {
			t.Fatalf("applicant access scope %s = %d / %s", path, rejected.Code, rejected.Body.String())
		}
	}
	submitted := fixture.request("POST", "/api/web/account/sites/HTTPown01/updates", string(body))
	if submitted.Code != http.StatusCreated {
		t.Fatalf("submit owner update = %d / %s", submitted.Code, submitted.Body.String())
	}
	var submission siteaudit.SubmissionResult
	if err := json.Unmarshal(submitted.Body.Bytes(), &submission); err != nil || submission.LookupToken != "" {
		t.Fatalf("account update result = %#v / %v", submission, err)
	}
	detail := fixture.request("GET", "/api/web/account/site-submissions/"+submission.AuditID, "")
	if detail.Code != 200 || !strings.Contains(detail.Body.String(), "OWNER_UPDATE") || !strings.Contains(detail.Body.String(), "CN_ONLY") {
		t.Fatalf("account update detail = %d / %s", detail.Code, detail.Body.String())
	}
	canonical := fixture.request("GET", "/api/web/account/sites/HTTPown01", "")
	if canonical.Code != 200 || strings.Contains(canonical.Body.String(), "Updated HTTP owner") {
		t.Fatalf("pending edit became canonical = %d / %s", canonical.Code, canonical.Body.String())
	}
	var snapshot siteaudit.Snapshot
	if err := json.Unmarshal(canonical.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	draftBody, err := json.Marshal(siteaudit.ReviewDraftInput{Site: siteaudit.ReviewSiteInput{SiteInput: input.Site, AccessScope: "GLOBAL_ONLY"}, ExpectedSiteRevision: snapshot.Revision})
	if err != nil {
		t.Fatal(err)
	}
	draft := fixture.request("PUT", "/api/web/management/site-audits/"+submission.AuditID+"/review-draft", string(draftBody))
	if draft.Code != http.StatusOK {
		t.Fatalf("admin scope draft = %d / %s", draft.Code, draft.Body.String())
	}
	decisionBody, err := json.Marshal(siteaudit.ReviewInput{Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: snapshot.Revision, ExpectedReviewDraftRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	approved := fixture.request("POST", "/api/web/management/site-audits/"+submission.AuditID+"/review", string(decisionBody))
	var audit siteaudit.Audit
	if err := json.Unmarshal(approved.Body.Bytes(), &audit); err != nil {
		t.Fatal(err)
	}
	if approved.Code != http.StatusOK || audit.FinalSnapshot.AccessScope != "GLOBAL_ONLY" {
		t.Fatalf("admin scope approval = %d / %s", approved.Code, approved.Body.String())
	}
}
