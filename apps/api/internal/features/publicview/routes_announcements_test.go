package publicview

import (
	"context"
	"encoding/json"
	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/httpapi"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnnouncementPublicReadsReturnEmptyAndNotFoundNormally(t *testing.T) {
	for _, test := range []struct {
		path   string
		status int
	}{{"/api/web/public/announcements/banner", 200}, {"/api/web/public/announcements", 200}, {"/api/web/public/announcements/missing", 404}, {"/api/web/public/announcements?page=0", 422}, {"/api/web/public/announcements?pageSize=101", 422}} {
		t.Run(test.path, func(t *testing.T) {
			// Given a Web-authorized reader with no public announcements.
			router := newTestRouter(t, nil)
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			request.Header.Set(httpapi.WebTokenHeader, testWebToken)
			response := httptest.NewRecorder()
			// When the public route is requested.
			router.ServeHTTP(response, request)
			// Then empty content is a normal result and invalid requests retain their status.
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if response.Code == 200 && response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("public announcement response is cacheable")
			}
			if test.path == "/api/web/public/announcements/banner" {
				var body announcementPublicBody
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Announcement != nil {
					t.Fatal("empty banner is not null")
				}
			}
		})
	}
}

func (stub publicViewReaderStub) Banner(context.Context) (*Announcement, error) {
	return nil, nil
}
func (stub publicViewReaderStub) AnnouncementArchive(context.Context, int32, int32) (AnnouncementArchive, error) {
	return AnnouncementArchive{Announcements: []Announcement{}, Page: 1, PageSize: 20}, nil
}
func (stub publicViewReaderStub) AnnouncementByID(context.Context, string) (*Announcement, error) {
	return nil, apperror.New(apperror.KindNotFound, "not_found", "announcement was not found")
}
