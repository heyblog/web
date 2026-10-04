//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"heyblog-api/internal/features/announcement"
	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/publicview"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/platform/config"
	"heyblog-api/internal/platform/httpapi"

	"github.com/jackc/pgx/v5/pgxpool"
)

type announcementActor struct{ user auth.User }

func TestAnnouncementManagementHTTPWithDatabase(t *testing.T) {
	fixture := newAuditMigrationFixture(t)
	if _, err := fixture.provider.Up(t.Context()); err != nil {
		t.Fatal(err)
	}
	verifyAnnouncementManagement(t.Context(), t, fixture.pool)
}

func (actor announcementActor) Current(context.Context, *http.Request) (auth.User, error) {
	return actor.user, nil
}

func verifyAnnouncementManagement(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	actor, err := dbgen.New(pool).CreateUser(ctx, dbgen.CreateUserParams{Email: "announcement-management@example.com", Username: "announcement_management", DisplayName: "Announcement manager"})
	if err != nil {
		t.Fatal(err)
	}
	router, err := httpapi.NewRouter(httpapi.Options{WebToken: "announcement-test", HealthcheckToken: "health-test", PublicViews: publicview.New(dbgen.New(pool)), HTTP: config.HTTPConfig{MaxBodyBytes: 1 << 20}})
	if err != nil {
		t.Fatal(err)
	}
	service := announcement.NewService(announcement.NewRepository(pool), announcementActor{auth.User{ID: actor.ID.String(), Role: auth.RoleSysAdmin}})
	if err := announcement.RegisterRoutes(router.API, service, "announcement-test"); err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body string) *httptest.ResponseRecorder {
		value := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		value.Header.Set(httpapi.WebTokenHeader, "announcement-test")
		value.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, value)
		return response
	}
	decode := func(response *httptest.ResponseRecorder) announcement.ManagedAnnouncement {
		t.Helper()
		var output struct {
			Announcement announcement.ManagedAnnouncement `json:"announcement"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &output); err != nil {
			t.Fatalf("decode announcement: %v body=%s", err, response.Body.String())
		}
		return output.Announcement
	}
	requireStatus := func(response *httptest.ResponseRecorder, status int) {
		t.Helper()
		if response.Code != status {
			t.Fatalf("response status = %d, want %d body=%s", response.Code, status, response.Body.String())
		}
	}
	versionBody := func(version string) string { return `{"rowVersion":"` + version + `"}` }

	// Given a manager creates a valid draft through the real HTTP adapter.
	response := request(http.MethodPost, "/management/announcements", `{"kind":"MAIN","title":"Management lifecycle","bodyMarkdown":"**Published body**","priority":3,"actionType":"NONE","actionLabel":null,"actionPath":null,"actionExternalUrl":null,"startsAt":null,"endsAt":null}`)
	requireStatus(response, http.StatusCreated)
	draft := decode(response)
	path := "/management/announcements/" + draft.ID
	requireStatus(request(http.MethodGet, "/announcements/"+draft.ID, ""), http.StatusNotFound)

	// When it is published, edited and permanently archived.
	response = request(http.MethodPost, path+"/publish", versionBody(draft.RowVersion))
	requireStatus(response, http.StatusOK)
	published := decode(response)
	public := request(http.MethodGet, "/announcements/"+draft.ID, "")
	requireStatus(public, http.StatusOK)
	if strings.Contains(public.Body.String(), "rowVersion") || strings.Contains(public.Body.String(), "createdBy") {
		t.Fatal("public response exposes management fields")
	}
	editedInput := announcement.EditInput{Input: published.Input, RowVersion: published.RowVersion}
	editedInput.Title = "Updated published title"
	body, err := json.Marshal(editedInput)
	if err != nil {
		t.Fatal(err)
	}
	response = request(http.MethodPut, path, string(body))
	requireStatus(response, http.StatusOK)
	edited := decode(response)
	requireStatus(request(http.MethodPut, path, string(body)), http.StatusConflict)
	revisions := request(http.MethodGet, path+"/revisions", "")
	requireStatus(revisions, http.StatusOK)
	var history struct {
		Revisions []announcement.Revision `json:"revisions"`
	}
	if err := json.Unmarshal(revisions.Body.Bytes(), &history); err != nil {
		t.Fatal(err)
	}
	if len(history.Revisions) != 1 || history.Revisions[0].Revision != published.RowVersion || history.Revisions[0].Title != published.Title {
		t.Fatalf("revisions = %#v", history.Revisions)
	}
	requireStatus(request(http.MethodDelete, path, versionBody(edited.RowVersion)), http.StatusConflict)
	response = request(http.MethodPost, path+"/archive", versionBody(edited.RowVersion))
	requireStatus(response, http.StatusOK)
	archived := decode(response)
	editedInput.RowVersion = archived.RowVersion
	body, err = json.Marshal(editedInput)
	if err != nil {
		t.Fatal(err)
	}
	requireStatus(request(http.MethodPut, path, string(body)), http.StatusConflict)
	requireStatus(request(http.MethodGet, "/announcements/"+draft.ID, ""), http.StatusOK)

	// Then concurrent banner publication permits exactly one overlapping interval.
	drafts := make([]announcement.ManagedAnnouncement, 2)
	for index := range drafts {
		response = request(http.MethodPost, "/management/announcements", `{"kind":"BANNER","title":"Concurrent banner","bodyMarkdown":null,"priority":0,"actionType":"NONE","actionLabel":null,"actionPath":null,"actionExternalUrl":null,"startsAt":null,"endsAt":null}`)
		requireStatus(response, http.StatusCreated)
		drafts[index] = decode(response)
	}
	var group sync.WaitGroup
	results := make([]*httptest.ResponseRecorder, 2)
	for index, draft := range drafts {
		group.Go(func() {
			results[index] = request(http.MethodPost, "/management/announcements/"+draft.ID+"/publish", `{"rowVersion":"`+draft.RowVersion+`","startsAt":"2050-01-01T00:00:00Z","endsAt":"2050-01-02T00:00:00Z"}`)
		})
	}
	group.Wait()
	successes, conflicts := 0, 0
	for index, result := range results {
		switch result.Code {
		case http.StatusOK:
			successes++
			view := decode(result)
			requireStatus(request(http.MethodPost, "/management/announcements/"+drafts[index].ID+"/archive", versionBody(view.RowVersion)), http.StatusOK)
			requireStatus(request(http.MethodGet, "/announcements/"+view.ID, ""), http.StatusNotFound)
		case http.StatusConflict:
			conflicts++
			if !strings.Contains(result.Body.String(), `"banner_window_conflict"`) {
				t.Fatalf("unexpected banner conflict = %s", result.Body.String())
			}
			requireStatus(request(http.MethodDelete, "/management/announcements/"+drafts[index].ID, versionBody(drafts[index].RowVersion)), http.StatusNoContent)
		default:
			t.Fatalf("concurrent banner status = %d body=%s", result.Code, result.Body.String())
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("banner results = %d successful, %d conflicts", successes, conflicts)
	}
}
