package databasebackup

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (uploads *Uploads) Create(ctx context.Context, owner string, size int64, hashes []string) (UploadSession, error) {
	if err := validateUpload(size, hashes); err != nil {
		return UploadSession{}, err
	}
	uploads.mu.Lock()
	defer uploads.mu.Unlock()
	if !uploads.settings.Enabled {
		return UploadSession{}, ErrUploadUnavailable
	}
	if uploads.closed {
		return UploadSession{}, ErrClosed
	}
	for _, entry := range uploads.entries {
		if entry.record.Owner == owner && (!uploadTerminal(entry.record.State) || entry.record.CleanupStatus == "pending") {
			return UploadSession{}, ErrBusy
		}
	}
	now, id := uploads.now(), uuid.NewString()
	record := uploadRecord{UploadSession: UploadSession{ID: id, Size: size, PartSize: UploadPartSize, PartSHA256: append([]string{}, hashes...),
		UploadedParts: []int{}, State: "uploading", CleanupStatus: "not_required"}, StorageIdentity: uploads.storageIdentity(), Owner: owner, Key: uploads.prefix + id + ".json", CreatedAt: now}
	for i := range record.PartSHA256 {
		record.PartSHA256[i] = strings.ToLower(record.PartSHA256[i])
	}
	record.touch(now)
	// Commit intent before COS can create a resource, including an ambiguous init.
	if err := uploads.metadata.Save(ctx, record); err != nil {
		return UploadSession{}, err
	}
	entry := &uploadEntry{record: record}
	uploads.entries[id] = entry
	if err := uploads.ensureInitiated(ctx, entry); err != nil {
		return UploadSession{}, err
	}
	return entry.record.session(), nil
}

func (uploads *Uploads) ensureInitiated(ctx context.Context, entry *uploadEntry) error {
	if uploads.settings.Transport == "direct" {
		return nil
	}
	if entry.record.RemoteUploadID != "" {
		return nil
	}
	work, finish, err := uploads.control(ctx, entry)
	if err != nil {
		return err
	}
	defer finish()
	key := entry.record.Key
	uploads.mu.Unlock()
	ids, err := uploads.storage.Uploads(work, key)
	uploads.mu.Lock()
	if err != nil {
		return err
	}
	if entry.record.State != "uploading" {
		return ErrUploadState
	}
	if len(ids) > 0 {
		entry.record.RemoteUploadID = ids[0]
	} else {
		previousUncertainty := entry.record.RemoteUncertainUntil
		uploads.extendRemoteUncertainty(&entry.record)
		if err := uploads.metadata.Save(ctx, entry.record); err != nil {
			return err
		}
		uploads.mu.Unlock()
		id, err := uploads.storage.Initiate(work, key)
		uploads.mu.Lock()
		if err != nil {
			uploads.extendRemoteUncertainty(&entry.record)
			return errors.Join(err, uploads.saveDetached(entry.record))
		}
		entry.record.RemoteUploadID = id
		entry.record.RemoteUncertainUntil = previousUncertainty
	}
	if entry.record.State != "uploading" {
		return ErrUploadState
	}
	return uploads.metadata.Save(ctx, entry.record)
}

func (uploads *Uploads) Get(ctx context.Context, owner, id string) (UploadSession, error) {
	uploads.mu.Lock()
	defer uploads.mu.Unlock()
	entry, err := uploads.entry(owner, id)
	if err != nil {
		return UploadSession{}, err
	}
	if entry.worker || len(entry.writers) > 0 {
		return entry.record.session(), nil
	}
	if entry.record.State == "uploading" {
		if err := uploads.ensureInitiated(ctx, entry); err != nil {
			return UploadSession{}, err
		}
		parts, err := uploads.listParts(ctx, entry)
		if err != nil {
			return UploadSession{}, err
		}
		previous := entry.record.UploadedBytes
		if err := reconcileParts(&entry.record, parts, false); err != nil {
			return UploadSession{}, err
		}
		if entry.record.UploadedBytes > previous {
			entry.record.touch(uploads.now())
		}
		if err := uploads.metadata.Save(ctx, entry.record); err != nil {
			return UploadSession{}, err
		}
	}
	if (entry.record.State == "ready" || entry.record.State == "inspected") && !entry.leased && !entry.worker {
		if _, err := os.Stat(uploads.localPath(id)); os.IsNotExist(err) {
			if uploads.settings.Transport == "direct" {
				resetDirect(&entry.record)
				parts, err := uploads.listParts(ctx, entry)
				if err != nil {
					return UploadSession{}, err
				}
				if err := reconcileParts(&entry.record, parts, false); err != nil {
					return UploadSession{}, err
				}
				return entry.record.session(), uploads.metadata.Save(ctx, entry.record)
			}
			entry.record.State, entry.record.PreparedBytes = "preparing", 0
			if err := uploads.metadata.Save(ctx, entry.record); err != nil {
				return UploadSession{}, err
			}
			uploads.startPreparation(entry)
		}
	}
	return entry.record.session(), nil
}

func (uploads *Uploads) Sign(ctx context.Context, owner, id string, number int) (string, time.Time, error) {
	uploads.mu.Lock()
	defer uploads.mu.Unlock()
	if uploads.settings.Transport != "cos" {
		return "", time.Time{}, ErrUploadUnavailable
	}
	entry, err := uploads.entry(owner, id)
	if err != nil {
		return "", time.Time{}, err
	}
	if entry.record.State != "uploading" || number < 1 || number > len(entry.record.PartSHA256) || !uploads.now().Before(entry.record.ExpiresAt) {
		return "", time.Time{}, ErrUploadState
	}
	if err := uploads.ensureInitiated(ctx, entry); err != nil {
		return "", time.Time{}, err
	}
	entry.record.touch(uploads.now())
	if err := uploads.metadata.Save(ctx, entry.record); err != nil {
		return "", time.Time{}, err
	}
	size := min(UploadPartSize, entry.record.Size-int64(number-1)*UploadPartSize)
	signed, err := uploads.storage.Sign(ctx, entry.record.Key, entry.record.RemoteUploadID, number, size)
	return signed, uploads.now().Add(10 * time.Minute), err
}

func reconcileParts(record *uploadRecord, parts []remotePart, complete bool) error {
	if complete && len(parts) != len(record.PartSHA256) {
		return ErrUploadState
	}
	record.UploadedParts, record.UploadedBytes = []int{}, 0
	seen := map[int]bool{}
	for _, part := range parts {
		if part.Number < 1 || part.Number > len(record.PartSHA256) || part.ETag == "" || seen[part.Number] {
			return ErrInvalid
		}
		expected := UploadPartSize
		if part.Number == len(record.PartSHA256) {
			expected = record.Size - int64(part.Number-1)*UploadPartSize
		}
		if part.Size != expected {
			return ErrInvalid
		}
		seen[part.Number] = true
		record.UploadedParts = append(record.UploadedParts, part.Number)
		record.UploadedBytes += part.Size
	}
	return nil
}
