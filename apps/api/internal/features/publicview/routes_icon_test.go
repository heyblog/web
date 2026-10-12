package publicview

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/httpapi"
)

func (stub publicViewReaderStub) SiteIconByIdentifier(context.Context, SiteIdentifier) (SiteIcon, error) {
	return SiteIcon{}, apperror.New(apperror.KindNotFound, apperror.CodeNotFound, "icon was not found")
}

type iconViewReader struct {
	publicViewReaderStub
	read func(context.Context, SiteIdentifier) (SiteIcon, error)
}

func (reader iconViewReader) SiteIconByIdentifier(ctx context.Context, identifier SiteIdentifier) (SiteIcon, error) {
	return reader.read(ctx, identifier)
}

func TestSiteIconRoute(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, identifier, token string
		want                    int
		kind                    IdentifierKind
		err                     error
	}{
		{name: "short id", identifier: "A1b2C3d4E", token: testWebToken, want: 200, kind: IdentifierShortID},
		{name: "uuid", identifier: "018f3f5f-8f2b-7c1a-8b4a-1d2e3f4a5b6c", token: testWebToken, want: 200, kind: IdentifierUUID},
		{name: "invalid", identifier: "custom-name", token: testWebToken, want: 400},
		{name: "unauthorized", identifier: "A1b2C3d4E", want: 401},
		{name: "missing", identifier: "A1b2C3d4E", token: testWebToken, want: 404, err: apperror.New(apperror.KindNotFound, apperror.CodeNotFound, "icon was not found")},
		{name: "unavailable", identifier: "A1b2C3d4E", token: testWebToken, want: 503, err: apperror.New(apperror.KindUnavailable, apperror.CodeServiceUnavailable, "icon unavailable")},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given the authenticated application icon boundary.
			calls := 0
			body := []byte("\x89PNG\r\n\x1a\n")
			router := newRouterWithViews(t, testHTTPConfig(), iconViewReader{read: func(_ context.Context, id SiteIdentifier) (SiteIcon, error) {
				calls++
				if test.kind != 0 && (id.Kind != test.kind || id.Value != test.identifier) {
					t.Fatalf("identifier=%+v", id)
				}
				return SiteIcon{Content: body, Hash: "abc123"}, test.err
			}})
			request := httptest.NewRequest(http.MethodGet, "/api/web/public/sites/id/"+test.identifier+"/icon", nil)
			request.Header.Set(httpapi.WebTokenHeader, test.token)
			response := httptest.NewRecorder()
			// When requesting a cached icon.
			router.ServeHTTP(response, request)
			// Then errors remain typed and success is raw PNG, not JSON/base64.
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
			if test.want == 400 || test.want == 401 {
				if calls != 0 {
					t.Fatalf("unauthorized or invalid request made %d reads", calls)
				}
				return
			}
			if test.want == 200 && (!bytes.Equal(response.Body.Bytes(), body) || response.Header().Get("Content-Type") != "image/png" || response.Header().Get("ETag") != "\"abc123\"" || response.Header().Get("Cache-Control") != "no-store") {
				t.Fatalf("headers=%v body=%q", response.Header(), response.Body)
			}
		})
	}
}

func (stub publicViewReaderStub) SiteMetadataByIdentifier(ctx context.Context, identifier SiteIdentifier) (SiteProfile, error) {
	return stub.SiteByIdentifier(ctx, identifier)
}
