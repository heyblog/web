package databasebackup

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"heyblog-api/internal/platform/config"

	"github.com/stretchr/testify/require"
	cos "github.com/tencentyun/cos-go-sdk-v5"
)

func TestCOSAdapterOwnsMultipartCompletionAndExactKeyCleanup(t *testing.T) {
	// Given: a wire-level COS fixture exercises the official SDK serialization.
	key := "backups/" + fixtureID + ".json"
	var mu sync.Mutex
	completed := false
	deleted := map[string]bool{}
	aborted := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		query := request.URL.Query()
		response.Header().Set("Content-Type", "application/xml")
		switch {
		case request.Method == http.MethodPost && query.Has("uploads"):
			_, _ = io.WriteString(response, `<InitiateMultipartUploadResult><UploadId>known-upload</UploadId></InitiateMultipartUploadResult>`)
		case request.Method == http.MethodGet && query.Get("uploadId") != "":
			_, _ = io.WriteString(response, `<ListPartsResult><IsTruncated>false</IsTruncated><Part><PartNumber>1</PartNumber><ETag>part-etag</ETag><Size>9</Size></Part></ListPartsResult>`)
		case request.Method == http.MethodPost && query.Get("uploadId") != "":
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Error(err)
			}
			if !strings.Contains(string(body), "part-etag") {
				t.Error("server completion did not use listed ETag")
			}
			completed = true
			_, _ = io.WriteString(response, `<CompleteMultipartUploadResult><ETag>object-etag</ETag></CompleteMultipartUploadResult>`)
		case request.Method == http.MethodHead:
			if !completed || deleted["version-1"] {
				response.WriteHeader(http.StatusNotFound)
				return
			}
			response.Header().Set("Content-Length", "9")
			response.Header().Set("ETag", "object-etag")
			response.Header().Set("x-cos-version-id", "version-1")
		case request.Method == http.MethodGet && query.Has("uploads"):
			_, _ = fmt.Fprintf(response, `<ListMultipartUploadsResult><IsTruncated>false</IsTruncated><Upload><Key>%s</Key><UploadId>known-upload</UploadId></Upload><Upload><Key>%s</Key><UploadId>ambiguous-upload</UploadId></Upload><Upload><Key>%s.other</Key><UploadId>unrelated-upload</UploadId></Upload></ListMultipartUploadsResult>`, key, key, key)
		case request.Method == http.MethodDelete && query.Get("uploadId") != "":
			aborted = append(aborted, query.Get("uploadId"))
			response.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodGet && query.Has("versions"):
			_, _ = fmt.Fprintf(response, `<ListVersionsResult><IsTruncated>false</IsTruncated><Version><Key>%s</Key><VersionId>version-1</VersionId></Version><DeleteMarker><Key>%s</Key><VersionId>marker-1</VersionId></DeleteMarker><Version><Key>%s.other</Key><VersionId>unrelated-version</VersionId></Version></ListVersionsResult>`, key, key, key)
		case request.Method == http.MethodDelete:
			deleted[query.Get("VersionId")] = true
			response.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodGet:
			if query.Get("versionId") != "version-1" || request.Header.Get("If-Match") != "object-etag" {
				t.Error("download was not bound to the completed object")
			}
			_, _ = io.WriteString(response, "synthetic")
		default:
			t.Errorf("unexpected COS operation %s", request.Method)
			response.WriteHeader(500)
		}
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	client := cos.NewClient(&cos.BaseURL{BucketURL: endpoint}, &http.Client{Transport: &cos.AuthorizationTransport{SecretID: "synthetic-id", SecretKey: "synthetic-key", Transport: server.Client().Transport}})
	storage := &cosStorage{client: client}
	// When
	id, err := storage.Initiate(t.Context(), key)
	require.NoError(t, err)
	require.Equal(t, "known-upload", id)
	parts, err := storage.Parts(t.Context(), key, id)
	require.NoError(t, err)
	signed, err := storage.Sign(t.Context(), key, id, 1, 9)
	require.NoError(t, err)
	parsed, err := url.Parse(signed)
	require.NoError(t, err)
	require.Equal(t, id, parsed.Query().Get("uploadId"))
	require.Equal(t, "1", parsed.Query().Get("partNumber"))
	require.Contains(t, parsed.Query().Get("q-header-list"), "content-length")
	require.Contains(t, parsed.Query().Get("q-url-param-list"), "uploadid")
	object, err := storage.Complete(t.Context(), key, id, parts)
	require.NoError(t, err)
	body, err := storage.Open(t.Context(), key, object)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.NoError(t, body.Close())
	require.Equal(t, "synthetic", string(data))
	require.NoError(t, storage.Cleanup(t.Context(), key))
	// Then
	mu.Lock()
	actualAborts := append([]string{}, aborted...)
	actualDeletes := map[string]bool{}
	for id, value := range deleted {
		actualDeletes[id] = value
	}
	mu.Unlock()
	require.ElementsMatch(t, []string{"known-upload", "ambiguous-upload"}, actualAborts)
	require.Equal(t, map[string]bool{"version-1": true, "marker-1": true}, actualDeletes)
}

func TestCOSUsesRegionalEndpointForBrowserAndBackend(t *testing.T) {
	// Given / When
	storage, origin, err := newCOSStorage(config.BackupCOSConfig{Bucket: "backup-123", Region: "ap-singapore", SecretID: "synthetic-id", SecretKey: "synthetic-key"})
	// Then
	require.NoError(t, err)
	require.Equal(t, "https://backup-123.cos.ap-singapore.myqcloud.com", origin)
	require.Equal(t, "https://backup-123.cos.ap-singapore.myqcloud.com", storage.Identity())
	signed, err := storage.Sign(t.Context(), "backups/test.json", "upload-id", 1, 9)
	require.NoError(t, err)
	parsed, err := url.Parse(signed)
	require.NoError(t, err)
	require.Equal(t, "backup-123.cos.ap-singapore.myqcloud.com", parsed.Host)
}
