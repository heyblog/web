package dataimport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"heyblog-api/internal/application/publicview"
	"heyblog-api/internal/config"
	"heyblog-api/internal/httpapi"
)

const testImportToken = "test-temp-import-token-0123456789abcdef"

func TestRouteAuthenticatesBeforeReadingBody(t *testing.T) {
	t.Parallel()

	operation := &recordingOperation{}
	router := newImportTestRouter(t, operation)
	body := &countingReader{Reader: strings.NewReader("private body")}
	request := httptest.NewRequest(http.MethodPost, Path, body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if body.reads != 0 || operation.calls != 0 {
		t.Fatalf("body reads and operation calls = (%d, %d), want authentication first", body.reads, operation.calls)
	}
	if got := response.Header().Get("WWW-Authenticate"); got != `Bearer realm="heyblog-temp-import"` {
		t.Fatalf("WWW-Authenticate = %q, want temporary import challenge", got)
	}
}

func TestRouteAuthenticatesDeclaredOversizedBodyBeforeBodyLimit(t *testing.T) {
	t.Parallel()

	operation := &recordingOperation{}
	router := newImportTestRouter(t, operation)
	body := &countingReader{Reader: strings.NewReader("private body")}
	request := httptest.NewRequest(http.MethodPost, Path, body)
	request.ContentLength = TotalBodyLimit + 1
	request.Header.Set("Content-Type", "multipart/form-data; boundary=private")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if body.reads != 0 || operation.calls != 0 {
		t.Fatalf("body reads and operation calls = (%d, %d), want authentication first", body.reads, operation.calls)
	}
}

func TestRouteImportsExactlyTwoCleanedFilesWithExtendedDeadlines(t *testing.T) {
	t.Parallel()

	blogs, graph := testBundleJSON()
	wantCounts := Counts{Sites: 2, FriendLinks: 1}
	operation := &recordingOperation{counts: wantCounts}
	router := newImportTestRouter(t, operation)
	request := multipartImportRequest(t, blogs, graph)
	request.Header.Set("Authorization", "Bearer "+testImportToken)
	recorder := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	started := time.Now()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("response = (%d, %q), want successful import", recorder.Code, recorder.Body.String())
	}
	if operation.calls != 1 || operation.bundles.Blogs.Count != 2 {
		t.Fatalf("operation = (%d, %#v), want one decoded import", operation.calls, operation.bundles)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", recorder.Header().Get("Cache-Control"))
	}
	if recorder.readDeadline.Before(started.Add(89*time.Minute+59*time.Second)) ||
		recorder.writeDeadline.Before(started.Add(89*time.Minute+59*time.Second)) {
		t.Fatalf("deadlines = (%s, %s), want approximately ninety minutes", recorder.readDeadline, recorder.writeDeadline)
	}
	var response struct {
		Status string `json:"status"`
		Hashes struct {
			Blogs string `json:"blogs"`
			Graph string `json:"graph"`
		} `json:"hashes"`
		Counts Counts `json:"counts"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "imported" || response.Counts != wantCounts {
		t.Fatalf("response = %#v, want imported counts", response)
	}
	if response.Hashes.Blogs != hexSHA256(blogs) || response.Hashes.Graph != hexSHA256(graph) {
		t.Fatalf("hashes = %#v, want uploaded file hashes", response.Hashes)
	}
}

func TestRouteRejectsMalformedMultipartRawJSONAndOversizedPart(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		request    func(*testing.T) *http.Request
		wantStatus int
	}{
		"raw JSON": {
			request: func(*testing.T) *http.Request {
				request := httptest.NewRequest(http.MethodPost, Path, strings.NewReader(`{"format":"zhblogs.blogs"}`))
				request.Header.Set("Content-Type", "application/json")
				return request
			},
			wantStatus: http.StatusBadRequest,
		},
		"oversized graph": {
			request: func(t *testing.T) *http.Request {
				blogs, _ := testBundleJSON()
				return multipartImportRequest(t, blogs, bytes.Repeat([]byte("x"), int(GraphFileLimit+1)))
			},
			wantStatus: http.StatusRequestEntityTooLarge,
		},
		"missing file": {
			request: func(t *testing.T) *http.Request {
				blogs, _ := testBundleJSON()
				return multipartRequest(t, map[string][]byte{"blogs": blogs})
			},
			wantStatus: http.StatusBadRequest,
		},
		"streamed total body too large": {
			request: func(t *testing.T) *http.Request {
				request := multipartImportRequest(
					t,
					bytes.Repeat([]byte("b"), int(BlogsFileLimit)),
					bytes.Repeat([]byte("g"), int(GraphFileLimit)),
				)
				request.ContentLength = -1
				return request
			},
			wantStatus: http.StatusRequestEntityTooLarge,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			operation := &recordingOperation{}
			router := newImportTestRouter(t, operation)
			request := test.request(t)
			request.Header.Set("Authorization", "Bearer "+testImportToken)
			response := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("response = (%d, %q), want %d", response.Code, response.Body.String(), test.wantStatus)
			}
			if operation.calls != 0 {
				t.Fatalf("operation calls = %d, want 0", operation.calls)
			}
		})
	}
}

func TestRouteMapsImportFailuresWithoutLeakingDiagnostics(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err        error
		wantStatus int
	}{
		"already running":        {err: ErrImportRunning, wantStatus: http.StatusConflict},
		"directory not empty":    {err: ErrDirectoryNotEmpty, wantStatus: http.StatusConflict},
		"invalid bundle":         {err: ErrInvalidBundle, wantStatus: http.StatusUnprocessableEntity},
		"dependency unavailable": {err: ErrDependencyUnavailable, wantStatus: http.StatusServiceUnavailable},
		"unexpected failure":     {err: errors.New("private database detail"), wantStatus: http.StatusInternalServerError},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			blogs, graph := testBundleJSON()
			operation := &recordingOperation{err: test.err}
			router := newImportTestRouter(t, operation)
			request := multipartImportRequest(t, blogs, graph)
			request.Header.Set("Authorization", "Bearer "+testImportToken)
			response := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
			router.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("response = (%d, %q), want %d", response.Code, response.Body.String(), test.wantStatus)
			}
			if strings.Contains(response.Body.String(), "private database detail") {
				t.Fatalf("response leaked internal diagnostics: %q", response.Body.String())
			}
		})
	}
}

func newImportTestRouter(t *testing.T, operation ImportOperation) *gin.Engine {
	t.Helper()
	router, err := httpapi.NewRouter(httpapi.Options{
		Mode:               config.ModeDevelopment,
		HTTP:               config.HTTPConfig{MaxBodyBytes: 1024, TrustedProxies: []string{}, CORS: config.CORSConfig{}},
		Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		HealthcheckToken:   "test-healthcheck-token-0123456789abcdef",
		WebToken:           "test-web-service-token-0123456789abcdef",
		PublicViews:        importTestPublicViews{},
		BodyLimitOverrides: BodyLimitOverrides(),
	})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	RegisterRoutes(router, operation, testImportToken, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return router
}

func multipartImportRequest(t *testing.T, blogs, graph []byte) *http.Request {
	t.Helper()
	return multipartRequest(t, map[string][]byte{"blogs": blogs, "graph": graph})
}

func multipartRequest(t *testing.T, files map[string][]byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, name := range []string{"blogs", "graph"} {
		contents, exists := files[name]
		if !exists {
			continue
		}
		part, err := writer.CreateFormFile(name, name+".json")
		if err != nil {
			t.Fatalf("CreateFormFile(%q): %v", name, err)
		}
		if _, err := part.Write(contents); err != nil {
			t.Fatalf("write multipart %q: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, Path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func hexSHA256(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

type recordingOperation struct {
	calls   int
	bundles Bundles
	counts  Counts
	err     error
}

type importTestPublicViews struct{}

func (importTestPublicViews) Home(context.Context) (publicview.Home, error) {
	return publicview.Home{}, nil
}

func (importTestPublicViews) Directory(
	context.Context,
	publicview.DirectoryQuery,
) (publicview.DirectoryView, error) {
	return publicview.DirectoryView{}, nil
}

func (importTestPublicViews) DirectoryOptions(context.Context) (publicview.DirectoryOptions, error) {
	return publicview.DirectoryOptions{}, nil
}

func (importTestPublicViews) SiteByIdentifier(
	context.Context,
	publicview.SiteIdentifier,
) (publicview.SiteProfile, error) {
	return publicview.SiteProfile{}, nil
}

func (importTestPublicViews) SiteByCustomID(context.Context, string) (publicview.SiteProfile, error) {
	return publicview.SiteProfile{}, nil
}

func (operation *recordingOperation) Import(_ context.Context, bundles Bundles) (Counts, error) {
	operation.calls++
	operation.bundles = bundles
	return operation.counts, operation.err
}

type countingReader struct {
	io.Reader
	reads int
}

func (reader *countingReader) Read(buffer []byte) (int, error) {
	reader.reads++
	return reader.Reader.Read(buffer)
}

type deadlineRecorder struct {
	*httptest.ResponseRecorder
	readDeadline  time.Time
	writeDeadline time.Time
}

func (recorder *deadlineRecorder) SetReadDeadline(deadline time.Time) error {
	recorder.readDeadline = deadline
	return nil
}

func (recorder *deadlineRecorder) SetWriteDeadline(deadline time.Time) error {
	recorder.writeDeadline = deadline
	return nil
}
