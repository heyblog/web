package httpapi

import (
	"context"
	"encoding/json"
	"heyblog-api/internal/features/publicview"
	"heyblog-api/internal/platform/apperror"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnnouncementPublicReadsReturnEmptyAndNotFoundNormally(t *testing.T) {
	for _, test := range []struct {
		path   string
		status int
	}{{"/announcements/banner", 200}, {"/announcements", 200}, {"/announcements/missing", 404}, {"/announcements?page=0", 422}, {"/announcements?pageSize=101", 422}} {
		t.Run(test.path, func(t *testing.T) {
			// Given a Web-authorized reader with no public announcements.
			router := newTestRouter(t, nil)
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			request.Header.Set(WebTokenHeader, testWebToken)
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
			if test.path == "/announcements/banner" {
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

func (stub publicViewReaderStub) Banner(context.Context) (*publicview.Announcement, error) {
	return nil, nil
}
func (stub publicViewReaderStub) AnnouncementArchive(context.Context, int32, int32) (publicview.AnnouncementArchive, error) {
	return publicview.AnnouncementArchive{Announcements: []publicview.Announcement{}, Page: 1, PageSize: 20}, nil
}
func (stub publicViewReaderStub) AnnouncementByID(context.Context, string) (*publicview.Announcement, error) {
	return nil, apperror.New(apperror.KindNotFound, "not_found", "announcement was not found")
}
