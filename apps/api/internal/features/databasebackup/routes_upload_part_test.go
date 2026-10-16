package databasebackup

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/httpapi"

	"github.com/stretchr/testify/require"
)

func directRouter(t *testing.T, uploads *Uploads) *httpapi.Router {
	t.Helper()
	return backupRouter(t, &routeOperation{}, routeAuth{user: auth.User{ID: "actor", Role: auth.RoleSysAdmin, Active: true}}, uploads)
}

func TestDirectPartRouteStreamsBeyondGlobalLimit(t *testing.T) {
	// Given: real HTTP and a global limit of only 1 KiB.
	uploads, _ := testDirectUploads(t)
	data := bytes.Repeat([]byte("a"), int(UploadPartSize))
	session, err := uploads.Create(t.Context(), "actor", int64(len(data)), partHashes(data))
	require.NoError(t, err)
	server := httptest.NewServer(directRouter(t, uploads))
	defer server.Close()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPut, server.URL+Path+"/uploads/"+session.ID+"/parts/1", bytes.NewReader(data))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set(httpapi.WebTokenHeader, "backup-test-web-token")
	// When
	response, err := server.Client().Do(request)
	require.NoError(t, err)
	defer func() { require.NoError(t, response.Body.Close()) }()
	// Then
	require.Equal(t, http.StatusNoContent, response.StatusCode)
	require.Equal(t, "private, no-store", response.Header.Get("Cache-Control"))
	current, err := uploads.Get(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Equal(t, []int{1}, current.UploadedParts)
}

func TestDirectPartRouteRejectsUnboundedAndMismatchedBodies(t *testing.T) {
	for _, test := range []struct {
		name        string
		size        int
		declared    bool
		contentType string
		status      int
	}{
		{"declared too large", int(UploadPartSize) + 1, true, "application/octet-stream", 413},
		{"chunked too large", int(UploadPartSize) + 1, false, "application/octet-stream", 413},
		{"wrong part size", 1024, true, "application/octet-stream", 422},
		{"JSON instead of binary", 1024, true, "application/json", 422},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			uploads, _ := testDirectUploads(t)
			data := bytes.Repeat([]byte("a"), int(UploadPartSize))
			session, err := uploads.Create(t.Context(), "actor", int64(len(data)), partHashes(data))
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodPut, Path+"/uploads/"+session.ID+"/parts/1", bytes.NewReader(bytes.Repeat([]byte("a"), test.size)))
			if !test.declared {
				request.ContentLength = -1
			}
			request.Header.Set("Content-Type", test.contentType)
			request.Header.Set(httpapi.WebTokenHeader, "backup-test-web-token")
			response := httptest.NewRecorder()
			// When
			directRouter(t, uploads).ServeHTTP(response, request)
			// Then
			require.Equal(t, test.status, response.Code, response.Body.String())
			_, err = os.Stat(uploads.partPath(session.ID, 1))
			require.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

func TestDirectPartCancellationInterruptsStalledSocketBeforeCleanup(t *testing.T) {
	// Given: the HTTP client declares a full part but stalls after one byte.
	uploads, _ := testDirectUploads(t)
	data := bytes.Repeat([]byte("a"), int(UploadPartSize))
	session, err := uploads.Create(t.Context(), "actor", int64(len(data)), partHashes(data))
	require.NoError(t, err)
	server := httptest.NewServer(directRouter(t, uploads))
	defer server.Close()
	connection, err := net.DialTimeout("tcp", strings.TrimPrefix(server.URL, "http://"), time.Second)
	require.NoError(t, err)
	defer func() { require.NoError(t, connection.Close()) }()
	_, err = fmt.Fprintf(connection, "PUT %s/uploads/%s/parts/1 HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/octet-stream\r\nContent-Length: %d\r\nX-Heyblog-Web-Token: backup-test-web-token\r\n\r\na", Path, session.ID, UploadPartSize)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		uploads.mu.Lock()
		defer uploads.mu.Unlock()
		return len(uploads.entries[session.ID].writers) == 1
	}, time.Second, time.Millisecond)
	// When
	_, err = uploads.Delete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	// Then: cleanup waits for the reader to exit rather than racing deletion.
	require.Eventually(t, func() bool {
		current, err := uploads.Get(t.Context(), "actor", session.ID)
		return err == nil && current.State == "cancelled" && current.CleanupStatus == "completed"
	}, time.Second, time.Millisecond)
	files, err := os.ReadDir(uploads.directory)
	require.NoError(t, err)
	require.Empty(t, files)
}
