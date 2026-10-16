package databasebackup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

func (uploads *Uploads) prepareFile(ctx context.Context, record uploadRecord, body io.Reader) (string, error) {
	partial := uploads.localPath(record.ID) + ".partial"
	file, err := os.OpenFile(partial, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600) // #nosec G304 -- path uses a server-generated UUID in the owned staging directory.
	if err != nil {
		return "", err
	}
	closed := false
	defer func() {
		if !closed {
			if err := file.Close(); err != nil {
				uploads.report("file_close", record.ID)
			}
		}
		if err := os.Remove(partial); err != nil && !errors.Is(err, os.ErrNotExist) {
			uploads.report("partial_cleanup", record.ID)
		}
	}()
	hash := sha256.New()
	buffer := make([]byte, UploadPartSize)
	var received int64
	for _, expected := range record.PartSHA256 {
		size := min(UploadPartSize, record.Size-received)
		if _, err := io.ReadFull(contextReader{ctx: ctx, reader: body}, buffer[:size]); err != nil {
			return "", err
		}
		digest := sha256.Sum256(buffer[:size])
		if hex.EncodeToString(digest[:]) != expected {
			return "", ErrInvalid
		}
		if _, err := file.Write(buffer[:size]); err != nil {
			return "", err
		}
		if _, err := hash.Write(buffer[:size]); err != nil {
			return "", err
		}
		received += size
		uploads.mu.Lock()
		entry := uploads.entries[record.ID]
		entry.record.PreparedBytes = received
		err := uploads.metadata.Save(ctx, entry.record)
		uploads.mu.Unlock()
		if err != nil {
			return "", err
		}
	}
	var extra [1]byte
	if count, err := body.Read(extra[:]); count != 0 || !errors.Is(err, io.EOF) {
		return "", ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	closed = true
	if err := os.Rename(partial, uploads.localPath(record.ID)); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
