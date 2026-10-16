package databasebackup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"testing"

	"heyblog-api/internal/platform/config"

	"github.com/stretchr/testify/require"
)

func testDirectUploads(t *testing.T) (*Uploads, *memoryUploadMetadata) {
	t.Helper()
	metadata := &memoryUploadMetadata{records: map[string][]byte{}}
	uploads, err := openUploads(t.Context(), metadata, nil, "direct/", t.TempDir(), "", slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	t.Cleanup(uploads.Close)
	return uploads, metadata
}

func receivePart(ctx context.Context, uploads *Uploads, id string, number int, data []byte) error {
	return uploads.ReceivePart(ctx, UploadPart{Owner: "actor", ID: id, Number: number, Body: io.NopCloser(bytes.NewReader(data))})
}

func TestDirectUploadAssemblesVerifiedPartsInOrder(t *testing.T) {
	// Given: the final part arrives before the first part.
	uploads, _ := testDirectUploads(t)
	data := append(bytes.Repeat([]byte("a"), int(UploadPartSize)), []byte("last part")...)
	session, err := uploads.Create(t.Context(), "actor", int64(len(data)), partHashes(data))
	require.NoError(t, err)
	require.NoError(t, receivePart(t.Context(), uploads, session.ID, 2, data[UploadPartSize:]))
	require.NoError(t, receivePart(t.Context(), uploads, session.ID, 1, data[:UploadPartSize]))
	// When
	_, err = uploads.Complete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	ready := waitUpload(t, uploads, session.ID, "ready")
	// Then
	actual, err := os.ReadFile(uploads.localPath(session.ID))
	require.NoError(t, err)
	require.Equal(t, data, actual)
	digest := sha256.Sum256(data)
	require.Equal(t, hex.EncodeToString(digest[:]), ready.SHA256)
}

func TestDirectUploadRejectsInvalidPartWithoutPublishing(t *testing.T) {
	for _, test := range []struct {
		name   string
		number int
		data   []byte
	}{
		{"hash", 1, []byte("bad")}, {"short", 1, []byte("ok")}, {"oversized", 1, []byte("okay")}, {"number", 2, []byte("yes")},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			uploads, _ := testDirectUploads(t)
			session, err := uploads.Create(t.Context(), "actor", 3, partHashes([]byte("yes")))
			require.NoError(t, err)
			// When
			err = receivePart(t.Context(), uploads, session.ID, test.number, test.data)
			// Then
			require.Error(t, err)
			current, err := uploads.Get(t.Context(), "actor", session.ID)
			require.NoError(t, err)
			require.Empty(t, current.UploadedParts)
			_, err = uploads.Complete(t.Context(), "actor", session.ID)
			require.ErrorIs(t, err, ErrUploadState)
		})
	}
}

func TestDirectUploadEnforcesOwnershipAndTransport(t *testing.T) {
	// Given
	uploads, _ := testDirectUploads(t)
	session, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	require.NoError(t, err)
	// When / Then
	err = uploads.ReceivePart(t.Context(), UploadPart{Owner: "other", ID: session.ID, Number: 1, Body: io.NopCloser(bytes.NewReader([]byte("x")))})
	require.ErrorIs(t, err, ErrUploadNotFound)
	_, _, err = uploads.Sign(t.Context(), "actor", session.ID, 1)
	require.ErrorIs(t, err, ErrUploadUnavailable)
	cos, _, _ := testUploads(t, []byte("x"))
	require.ErrorIs(t, receivePart(t.Context(), cos, session.ID, 1, []byte("x")), ErrUploadUnavailable)
}

func TestProductionWithoutCOSCannotReceiveBackupFiles(t *testing.T) {
	// Given
	uploads, err := NewUploads(t.Context(), nil, config.Config{Mode: config.ModeProduction}, nil)
	require.NoError(t, err)
	t.Cleanup(uploads.Close)
	// When
	_, err = uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	// Then
	require.ErrorIs(t, err, ErrUploadUnavailable)
	require.Equal(t, "disabled", uploads.Settings().Transport)
	require.ErrorIs(t, receivePart(t.Context(), uploads, fixtureID, 1, []byte("x")), ErrUploadUnavailable)
}
