package databasebackup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

func checksumFile(ctx context.Context, path string, size int64, hashes []string) (checksum string, resultErr error) {
	info, err := os.Lstat(path) // #nosec G703 -- paths belong to the private UUID staging directory.
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() != size {
		return "", ErrInvalid
	}
	file, err := os.Open(path) // #nosec G304 -- validated private staging path.
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	hash := sha256.New()
	buffer := make([]byte, UploadPartSize)
	var received int64
	for _, expected := range hashes {
		length := min(UploadPartSize, size-received)
		if _, err := io.ReadFull(contextReader{ctx: ctx, reader: file}, buffer[:length]); err != nil {
			return "", err
		}
		digest := sha256.Sum256(buffer[:length])
		if hex.EncodeToString(digest[:]) != expected {
			return "", ErrInvalid
		}
		if _, err := hash.Write(buffer[:length]); err != nil {
			return "", err
		}
		received += length
	}
	return hex.EncodeToString(hash.Sum(nil)), ctx.Err()
}

func (uploads *Uploads) directParts(ctx context.Context, record uploadRecord) ([]remotePart, error) {
	parts := []remotePart{}
	for index, digest := range record.PartSHA256 {
		size := min(UploadPartSize, record.Size-int64(index)*UploadPartSize)
		_, err := checksumFile(ctx, uploads.partPath(record.ID, index+1), size, []string{digest})
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrInvalid) {
			continue
		}
		if err != nil {
			return nil, err
		}
		parts = append(parts, remotePart{Number: index + 1, Size: size, ETag: digest})
	}
	return parts, nil
}

func resetDirect(record *uploadRecord) {
	record.State, record.SHA256, record.ErrorCode = "uploading", "", ""
	record.Inspection = nil
	record.UploadedParts, record.UploadedBytes, record.PreparedBytes = []int{}, 0, 0
}

// Recovery validates actual bytes, never trusting Redis claims about /tmp.
// A restore interrupted by a restart has already become terminal unknown.
func (uploads *Uploads) recoverDirect(ctx context.Context, entry *uploadEntry) error {
	record := &entry.record
	if err := validateUpload(record.Size, record.PartSHA256); err != nil {
		return err
	}
	if err := uploads.removeDirectPartials(record.ID); err != nil {
		return err
	}
	if record.State != "uploading" {
		checksum, err := checksumFile(ctx, uploads.localPath(record.ID), record.Size, record.PartSHA256)
		if err == nil {
			record.State, record.SHA256, record.PreparedBytes = "ready", checksum, record.Size
			record.ErrorCode = ""
			record.UploadedBytes, record.UploadedParts = record.Size, []int{}
			for index := range record.PartSHA256 {
				record.UploadedParts = append(record.UploadedParts, index+1)
			}
			if record.Inspection != nil && record.Inspection.SHA256 == checksum {
				record.State = "inspected"
			} else {
				record.Inspection = nil
			}
			return uploads.metadata.Save(ctx, *record)
		}
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, ErrInvalid) {
			return err
		}
	}
	resetDirect(record)
	parts, err := uploads.directParts(ctx, *record)
	if err != nil {
		return err
	}
	if err := reconcileParts(record, parts, false); err != nil {
		return err
	}
	return uploads.metadata.Save(ctx, *record)
}

func (uploads *Uploads) removeDirectPartials(id string) error {
	paths, err := filepath.Glob(filepath.Join(uploads.partsDirectory(id), "*.partial"))
	if err != nil {
		return err
	}
	paths = append(paths, uploads.localPath(id)+".partial")
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (uploads *Uploads) prepareDirect(ctx context.Context, record uploadRecord) (checksum string, resultErr error) {
	files := make([]*os.File, 0, len(record.PartSHA256))
	defer func() {
		for _, file := range files {
			resultErr = errors.Join(resultErr, file.Close())
		}
	}()
	readers := make([]io.Reader, 0, len(record.PartSHA256))
	for index := range record.PartSHA256 {
		file, err := os.Open(uploads.partPath(record.ID, index+1)) // #nosec G304 -- server-owned UUID and bounded part index.
		if err != nil {
			return "", err
		}
		files = append(files, file)
		readers = append(readers, file)
	}
	return uploads.prepareFile(ctx, record, io.MultiReader(readers...))
}
