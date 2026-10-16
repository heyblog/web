package databasebackup

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type UploadPart struct {
	Owner  string
	ID     string
	Number int
	Body   io.ReadCloser
}

// ReceivePart tracks writers separately from preparation so two bounded parts
// can stream concurrently without holding the coordinator lock during I/O.
func (uploads *Uploads) ReceivePart(ctx context.Context, part UploadPart) error {
	uploads.mu.Lock()
	if uploads.settings.Transport != "direct" {
		uploads.mu.Unlock()
		return ErrUploadUnavailable
	}
	entry, err := uploads.entry(part.Owner, part.ID)
	if err != nil {
		uploads.mu.Unlock()
		return err
	}
	if entry.record.State != "uploading" || part.Number < 1 || part.Number > len(entry.record.PartSHA256) {
		uploads.mu.Unlock()
		return ErrUploadState
	}
	if entry.worker || entry.leased || len(entry.writers) >= 2 || entry.writers[part.Number] != nil {
		uploads.mu.Unlock()
		return ErrBusy
	}
	work, cancel := context.WithTimeout(ctx, 2*time.Minute)
	stop := context.AfterFunc(uploads.ctx, cancel)
	unblock := context.AfterFunc(work, func() {
		if err := part.Body.Close(); err != nil {
			uploads.report("part_body_close", part.ID)
		}
	})
	if entry.writers == nil {
		entry.writers = map[int]context.CancelFunc{}
	}
	entry.writers[part.Number] = cancel
	record := entry.record
	uploads.workers.Add(1)
	uploads.mu.Unlock()
	defer func() {
		stop()
		unblock()
		cancel()
		uploads.mu.Lock()
		delete(entry.writers, part.Number)
		if entry.record.CleanupStatus == "pending" {
			uploads.wakeCleanup()
		}
		uploads.mu.Unlock()
		uploads.workers.Done()
	}()
	path, err := uploads.writeDirectPart(work, record, part)
	if err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			uploads.report("part_partial_cleanup", part.ID)
		}
	}()
	uploads.mu.Lock()
	defer uploads.mu.Unlock()
	if err := work.Err(); err != nil {
		return err
	}
	if entry.record.State != "uploading" || !uploads.now().Before(entry.record.ExpiresAt) {
		return ErrUploadState
	}
	if err := os.Rename(path, uploads.partPath(part.ID, part.Number)); err != nil {
		return err
	}
	// The filesystem is authoritative if this metadata write fails. A retry or
	// restart validates the published part again before reporting it as uploaded.
	found := false
	for _, number := range entry.record.UploadedParts {
		found = found || number == part.Number
	}
	if !found {
		entry.record.UploadedParts = append(entry.record.UploadedParts, part.Number)
		entry.record.UploadedBytes += min(UploadPartSize, record.Size-int64(part.Number-1)*UploadPartSize)
	}
	entry.record.touch(uploads.now())
	return uploads.metadata.Save(work, entry.record)
}

func (uploads *Uploads) writeDirectPart(ctx context.Context, record uploadRecord, part UploadPart) (string, error) {
	directory := uploads.partsDirectory(record.ID)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(directory, "*.partial")
	if err != nil {
		return "", err
	}
	path := file.Name()
	size := min(UploadPartSize, record.Size-int64(part.Number-1)*UploadPartSize)
	count, copyErr := io.Copy(file, io.LimitReader(contextReader{ctx: ctx, reader: part.Body}, size+1))
	if copyErr == nil && count != size {
		copyErr = ErrInvalid
	}
	err = errors.Join(copyErr, file.Sync(), file.Close())
	if err == nil {
		_, err = checksumFile(ctx, path, size, []string{record.PartSHA256[part.Number-1]})
	}
	if err != nil {
		return "", errors.Join(err, os.Remove(path))
	}
	return path, nil
}

func (uploads *Uploads) partsDirectory(id string) string {
	return filepath.Join(uploads.directory, id+".parts")
}

func (uploads *Uploads) partPath(id string, number int) string {
	return filepath.Join(uploads.partsDirectory(id), strconv.Itoa(number)+".part")
}
