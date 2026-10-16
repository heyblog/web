//go:build integration

package integration_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
	"time"

	"heyblog-api/internal/features/databasebackup"
	"heyblog-api/internal/infrastructure/cache"
	"heyblog-api/internal/platform/config"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestDevelopmentBackupUploadResumesWithRedisMetadata(t *testing.T) {
	// Given: the production constructor selects direct upload from API mode and
	// persists the immutable manifest in real Redis, with an isolated /tmp root.
	t.Setenv("TMPDIR", t.TempDir())
	container, err := tcredis.Run(t.Context(), "redis:8.4-alpine")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testcontainers.TerminateContainer(container)) })
	address, err := container.ConnectionString(t.Context())
	require.NoError(t, err)
	client, err := cache.OpenRedis(t.Context(), config.RedisConfig{URL: address, DialTimeout: 3 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	configuration := config.Config{Mode: config.ModeDevelopment}
	uploads, err := databasebackup.NewUploads(t.Context(), client, configuration, nil)
	require.NoError(t, err)
	t.Cleanup(uploads.Close)
	require.Equal(t, "direct", uploads.Settings().Transport)
	data := []byte("synthetic local backup")
	digest := sha256.Sum256(data)
	checksum := hex.EncodeToString(digest[:])
	session, err := uploads.Create(t.Context(), "actor", int64(len(data)), []string{checksum})
	require.NoError(t, err)
	require.NoError(t, uploads.ReceivePart(t.Context(), databasebackup.UploadPart{Owner: "actor", ID: session.ID, Number: 1, Body: io.NopCloser(bytes.NewReader(data))}))
	uploads.Close()
	// When: a new coordinator starts from Redis plus the actual staged files.
	restarted, err := databasebackup.NewUploads(t.Context(), client, configuration, nil)
	require.NoError(t, err)
	t.Cleanup(restarted.Close)
	current, err := restarted.Get(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	// Then: the published part survives and can be assembled without reupload.
	require.Equal(t, []int{1}, current.UploadedParts)
	_, err = restarted.Complete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		ready, err := restarted.Get(t.Context(), "actor", session.ID)
		return err == nil && ready.State == "ready" && ready.SHA256 == checksum
	}, 3*time.Second, time.Millisecond)
	_, err = restarted.Delete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		deleted, err := restarted.Get(t.Context(), "actor", session.ID)
		return err == nil && deleted.CleanupStatus == "completed"
	}, 3*time.Second, time.Millisecond)
}
