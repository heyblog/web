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
	fixture.router.ServeHTTP(missingToken, httptest.NewRequest("GET", "/account/sites", nil))
	if missingToken.Code != 401 {
		t.Fatalf("missing web token = %d", missingToken.Code)
	}
	request := httptest.NewRequest("GET", "/account/sites", nil)
	request.Header.Set("X-HeyBlog-Web-Token", credentialWebToken)
	missingSession := httptest.NewRecorder()
	fixture.router.ServeHTTP(missingSession, request)
	if missingSession.Code != 401 {
		t.Fatalf("missing session = %d", missingSession.Code)
	}
	forbidden := fixture.request("PUT", "/account/sites/HTTPown01/friend-links/HTTPfri01", "")
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("unverified edit = %d / %s", forbidden.Code, forbidden.Body.String())
	}
	created := fixture.request("POST", "/account/site-claims", `{"short_id":"HTTPown01","method":"META"}`)
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
	checked := fixture.request("POST", "/account/site-claims/"+challenge.Claim.ID+"/check", "")
	if checked.Code != http.StatusOK {
		t.Fatalf("check certification = %d / %s", checked.Code, checked.Body.String())
	}
	replay := fixture.request("POST", "/account/site-claims/"+challenge.Claim.ID+"/check", "")
	if replay.Code != http.StatusConflict {
		t.Fatalf("replay check = %d / %s", replay.Code, replay.Body.String())
	}
	listed := fixture.request("GET", "/account/sites", "")
	if listed.Code != 200 || !strings.Contains(listed.Body.String(), "HTTPown01") || listed.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("owned site list = %d / %s", listed.Code, listed.Body.String())
	}
	linked := fixture.request("PUT", "/account/sites/HTTPown01/friend-links/HTTPfri01", "")
	if linked.Code != 200 {
		t.Fatalf("add friend = %d / %s", linked.Code, linked.Body.String())
	}
	friends := fixture.request("GET", "/account/sites/HTTPown01/friend-links", "")
	if friends.Code != 200 || !strings.Contains(friends.Body.String(), "HTTPfri01") {
		t.Fatalf("read friend = %d / %s", friends.Code, friends.Body.String())
	}
	removed := fixture.request("DELETE", "/account/sites/HTTPown01/friend-links/by-host/http-friend.example.test", "")
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
	input.Site.AccessScope = "CN_ONLY"
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	submitted := fixture.request("POST", "/account/sites/HTTPown01/updates", string(body))
	if submitted.Code != http.StatusCreated {
		t.Fatalf("submit owner update = %d / %s", submitted.Code, submitted.Body.String())
	}
	var submission siteaudit.SubmissionResult
	if err := json.Unmarshal(submitted.Body.Bytes(), &submission); err != nil || submission.LookupToken != "" {
		t.Fatalf("account update result = %#v / %v", submission, err)
	}
	detail := fixture.request("GET", "/account/site-submissions/"+submission.AuditID, "")
	if detail.Code != 200 || !strings.Contains(detail.Body.String(), "OWNER_UPDATE") || !strings.Contains(detail.Body.String(), "CN_ONLY") {
		t.Fatalf("account update detail = %d / %s", detail.Code, detail.Body.String())
	}
	canonical := fixture.request("GET", "/account/sites/HTTPown01", "")
	if canonical.Code != 200 || strings.Contains(canonical.Body.String(), "Updated HTTP owner") {
		t.Fatalf("pending edit became canonical = %d / %s", canonical.Code, canonical.Body.String())
	}
}
