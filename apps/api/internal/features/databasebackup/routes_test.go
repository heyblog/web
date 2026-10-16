package databasebackup

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/config"
	"heyblog-api/internal/platform/httpapi"
)

type routeAuth struct {
	user auth.User
	err  error
}

type slowRouteOperation struct{ routeOperation }

func (operation *slowRouteOperation) Inspect(ctx context.Context, file *os.File, actor string) (Inspection, error) {
	select {
	case <-time.After(200 * time.Millisecond):
		return operation.routeOperation.Inspect(ctx, file, actor)
	case <-ctx.Done():
		return Inspection{}, ctx.Err()
	}
}
func TestBackupInspectionOutlivesServerWriteTimeout(t *testing.T) {
	t.Parallel()
	operation := &slowRouteOperation{}
	server := httptest.NewUnstartedServer(backupRouter(t, operation, routeAuth{user: auth.User{ID: "actor", Role: auth.RoleSysAdmin, Active: true}}))
	server.Config.WriteTimeout = 50 * time.Millisecond
	server.Start()
	defer server.Close()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+Path+"/inspect", strings.NewReader(`{"upload_id":"00000000-0000-4000-8000-000000000001"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(httpapi.WebTokenHeader, "backup-test-web-token")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("inspection status=%d", response.StatusCode)
	}
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
	return Inspection{SHA256: fixtureChecksum, CanRestore: true, RetainedSystemAdminID: actor, Issues: []Issue{}, Datasets: []Dataset{}}, nil
}
func (operation *routeOperation) RestoreWithProgress(_ context.Context, file *os.File, actor, checksum string, _ func(RestoreProgress) error) (Restoration, error) {
	operation.calls++
	operation.filePath = file.Name()
	return Restoration{Status: "restored", SHA256: checksum, AdminMapping: AdminMapping{TargetID: actor}}, nil
}

func backupRouter(t *testing.T, operation Operations, authenticator Authenticator, staged ...*Uploads) *httpapi.Router {
	t.Helper()
	router, err := httpapi.NewRouter(httpapi.Options{Mode: config.ModeDevelopment, HTTP: config.HTTPConfig{MaxBodyBytes: 1024, TrustedProxies: []string{}}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), WebToken: "backup-test-web-token", HealthcheckToken: "health-test-token", BodyLimitOverrides: BodyLimitOverrides()})
	if err != nil {
		t.Fatal(err)
	}
	jobs := NewJobs(operation, nil)
	var uploads *Uploads
	if len(staged) > 0 {
		uploads = staged[0]
	} else {
		uploads = fixtureUploads(t)
	}
	t.Cleanup(jobs.Close)
	RegisterRoutes(router.API, operation, jobs, authenticator, "backup-test-web-token", nil, uploads)
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
		{"missing session", "backup-test-web-token", auth.User{}, &auth.AuthError{StatusCode: 401}, 401},
		{"session storage unavailable", "backup-test-web-token", auth.User{}, context.DeadlineExceeded, 503},
		{"profile confirmation required", "backup-test-web-token", auth.User{}, &auth.AuthError{StatusCode: 403, Code: "profile_confirmation_required"}, 403},
		{"delegated admin", "backup-test-web-token", auth.User{Role: auth.RoleAdmin, Active: true}, nil, 403},
		{"inactive admin", "backup-test-web-token", auth.User{Role: auth.RoleSysAdmin}, nil, 403},
	} {
		t.Run(test.name, func(t *testing.T) {
			operation := &routeOperation{}
			router := backupRouter(t, operation, routeAuth{user: test.user, err: test.err})
			for _, endpoint := range []struct{ method, path string }{
				{http.MethodPost, "export"}, {http.MethodPost, "inspect"}, {http.MethodPost, "restore"},
				{http.MethodGet, "restore-jobs/current"}, {http.MethodGet, "restore-jobs/00000000-0000-4000-8000-000000000000"},
				{http.MethodPost, "restore-jobs/00000000-0000-4000-8000-000000000000/cancel"},
				{http.MethodPost, "uploads"}, {http.MethodGet, "uploads/current"},
				{http.MethodGet, "uploads/00000000-0000-4000-8000-000000000000"},
				{http.MethodPost, "uploads/00000000-0000-4000-8000-000000000000/parts/1"},
				{http.MethodPut, "uploads/00000000-0000-4000-8000-000000000000/parts/1"},
				{http.MethodPost, "uploads/00000000-0000-4000-8000-000000000000/complete"},
				{http.MethodDelete, "uploads/00000000-0000-4000-8000-000000000000"},
			} {
				request := httptest.NewRequest(endpoint.method, Path+"/"+endpoint.path, strings.NewReader("invalid"))
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

func TestBackupRoutesReuseStagedFileForInspectionAndRestore(t *testing.T) {
	t.Parallel()
	operation := &routeOperation{}
	router := backupRouter(t, operation, routeAuth{user: auth.User{ID: "actor", Role: auth.RoleSysAdmin, Active: true}})
	for _, action := range []string{"export", "inspect", "restore"} {
		body := `{"upload_id":"00000000-0000-4000-8000-000000000001","sha256":"` + fixtureChecksum + `","confirmation":"RESTORE"}`
		if action == "export" {
			body = `{}`
		}
		if action == "inspect" {
			body = `{"upload_id":"00000000-0000-4000-8000-000000000001"}`
		}
		request := httptest.NewRequest(http.MethodPost, Path+"/"+action, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set(httpapi.WebTokenHeader, "backup-test-web-token")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		want := http.StatusOK
		if action == "restore" {
			want = http.StatusAccepted
		}
		if response.Code != want {
			t.Fatalf("%s status=%d body=%s", action, response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("missing no-store")
		}
		if action == "inspect" {
			if _, err := os.Stat(operation.filePath); err != nil {
				t.Fatalf("staged file removed by inspect: %v", err)
			}
		}
		if action == "restore" {
			var accepted RestoreJob
			if err := json.Unmarshal(response.Body.Bytes(), &accepted); err != nil {
				t.Fatal(err)
			}
			if accepted.UploadID != "00000000-0000-4000-8000-000000000001" {
				t.Fatal("missing upload association")
			}
			deadline := time.NewTimer(time.Second)
			defer deadline.Stop()
			for {
				request := httptest.NewRequest(http.MethodGet, Path+"/restore-jobs/"+accepted.ID, nil)
				request.Header.Set(httpapi.WebTokenHeader, "backup-test-web-token")
				status := httptest.NewRecorder()
				router.ServeHTTP(status, request)
				var job RestoreJob
				if err := json.Unmarshal(status.Body.Bytes(), &job); err != nil {
					t.Fatal(err)
				}
				if job.Status == "succeeded" {
					break
				}
				select {
				case <-deadline.C:
					t.Fatal("restore did not complete")
				case <-time.After(time.Millisecond):
				}
			}
		}
		if action != "inspect" {
			if _, err := os.Stat(operation.filePath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("temporary retained: %v", err)
			}
		}
	}
}
