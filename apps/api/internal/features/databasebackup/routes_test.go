package databasebackup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/publicview"
	"heyblog-api/internal/platform/config"
	"heyblog-api/internal/platform/httpapi"
)

type routeAuth struct {
	user auth.User
	err  error
}

func (authenticator routeAuth) Current(context.Context, *http.Request) (auth.User, error) {
	return authenticator.user, authenticator.err
}

type routeOperation struct {
	calls    int
	filePath string
}

func (operation *routeOperation) Export(context.Context) (*os.File, error) {
	operation.calls++
	file, err := os.CreateTemp("", "backup-route-test-*")
	if err != nil {
		return nil, err
	}
	operation.filePath = file.Name()
	if _, err = file.WriteString(`{"format":"synthetic"}`); err != nil {
		return nil, errorsWithCleanup(err, file)
	}
	_, err = file.Seek(0, io.SeekStart)
	return file, err
}
func (operation *routeOperation) Inspect(_ context.Context, file *os.File, actor string) (Inspection, error) {
	operation.calls++
	operation.filePath = file.Name()
	return Inspection{RetainedSystemAdminID: actor, Issues: []Issue{}, Datasets: []Dataset{}}, nil
}
func (operation *routeOperation) Restore(_ context.Context, file *os.File, actor, checksum string) (Restoration, error) {
	operation.calls++
	operation.filePath = file.Name()
	return Restoration{Status: "restored", SHA256: checksum, AdminMapping: AdminMapping{TargetID: actor}}, nil
}

func backupRouter(t *testing.T, operation Operations, authenticator Authenticator) *httpapi.Router {
	t.Helper()
	router, err := httpapi.NewRouter(httpapi.Options{Mode: config.ModeDevelopment, HTTP: config.HTTPConfig{MaxBodyBytes: 1024, TrustedProxies: []string{}}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), WebToken: "backup-test-web-token", HealthcheckToken: "health-test-token", PublicViews: publicview.New(nil, nil), BodyLimitOverrides: BodyLimitOverrides()})
	if err != nil {
		t.Fatal(err)
	}
	RegisterRoutes(router.API, operation, authenticator, "backup-test-web-token", nil)
	return router
}

func TestBackupRoutesRequireBothWebAndActiveAdministrator(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, token string
		user        auth.User
		err         error
		status      int
	}{
		{"missing web token", "", auth.User{Role: auth.RoleSysAdmin, Active: true}, nil, 401},
		{"missing session", "backup-test-web-token", auth.User{}, errors.New("no session"), 401},
		{"delegated admin", "backup-test-web-token", auth.User{Role: auth.RoleAdmin, Active: true}, nil, 403},
		{"inactive admin", "backup-test-web-token", auth.User{Role: auth.RoleSysAdmin}, nil, 403},
	} {
		t.Run(test.name, func(t *testing.T) {
			operation := &routeOperation{}
			router := backupRouter(t, operation, routeAuth{user: test.user, err: test.err})
			for _, action := range []string{"export", "inspect", "restore"} {
				request := httptest.NewRequest(http.MethodPost, Path+"/"+action, strings.NewReader("invalid"))
				request.Header.Set(httpapi.WebTokenHeader, test.token)
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				if response.Code != test.status || operation.calls != 0 {
					t.Fatalf("status %d calls %d", response.Code, operation.calls)
				}
			}
		})
	}
}

func TestBackupRoutesStreamFilesAndRemovePrivateTemporaries(t *testing.T) {
	t.Parallel()
	operation := &routeOperation{}
	router := backupRouter(t, operation, routeAuth{user: auth.User{ID: "actor", Role: auth.RoleSysAdmin, Active: true}})
	for _, action := range []string{"export", "inspect", "restore"} {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", "backup.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = io.WriteString(part, `{"synthetic":true}`); err != nil {
			t.Fatal(err)
		}
		if err = writer.WriteField("sha256", strings.Repeat("a", 64)); err != nil {
			t.Fatal(err)
		}
		if err = writer.WriteField("confirmation", "RESTORE"); err != nil {
			t.Fatal(err)
		}
		if err = writer.Close(); err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, Path+"/"+action, &body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.Header.Set(httpapi.WebTokenHeader, "backup-test-web-token")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != 200 {
			t.Fatalf("%s status=%d body=%s", action, response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("missing no-store")
		}
		if _, err = os.Stat(operation.filePath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("temporary retained: %v", err)
		}
		if action == "export" && (!strings.Contains(response.Header().Get("Content-Disposition"), "attachment") || response.Body.String() != `{"format":"synthetic"}`) {
			t.Fatal("invalid downloaded file")
		}
	}
}
