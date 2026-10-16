package databasebackup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"heyblog-api/internal/platform/config"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type uploadEntry struct {
	record  uploadRecord
	cancel  context.CancelFunc
	leased  bool
	worker  bool
	writers map[int]context.CancelFunc
}

var storedUploadPrefix = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9/_-]*/$`)

// Uploads owns durable upload intent and private staging files. A single API
// process owns this namespace; Redis must retain AOF across process restarts.
type Uploads struct {
	mu                sync.Mutex
	workers           sync.WaitGroup
	entries           map[string]*uploadEntry
	storage           uploadStorage
	metadata          uploadMetadata
	settings          UploadSettings
	prefix, directory string
	ctx               context.Context
	cancel            context.CancelFunc
	wake              chan struct{}
	closed            bool
	logger            *slog.Logger
	now               func() time.Time
}

func NewUploads(ctx context.Context, client *redis.Client, configuration config.Config, logger *slog.Logger) (*Uploads, error) {
	if logger == nil {
		logger = slog.Default()
	}
	directory := filepath.Join(os.TempDir(), "heyblog-database-uploads")
	if !configuration.BackupCOS.Enabled {
		if configuration.Mode == config.ModeDevelopment {
			return openUploads(ctx, redisUploadMetadata{client}, nil, "direct/", directory, "", logger)
		}
		return &Uploads{settings: UploadSettings{Transport: "disabled", PartSize: UploadPartSize, MaxFileSize: FileLimit}, logger: logger}, nil
	}
	storage, origin, err := newCOSStorage(configuration.BackupCOS)
	if err != nil {
		return nil, err
	}
	return openUploads(ctx, redisUploadMetadata{client}, storage, configuration.BackupCOS.Prefix, directory, origin, logger)
}

func openUploads(ctx context.Context, metadata uploadMetadata, storage uploadStorage, prefix, directory, origin string, logger *slog.Logger) (*Uploads, error) {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, fmt.Errorf("create backup staging directory: %w", err)
	}
	if err := os.Chmod(directory, 0700); err != nil { // #nosec G302 -- owner-only directories need traversal permission.
		return nil, err
	}
	records, err := metadata.Load(ctx)
	if err != nil {
		return nil, err
	}
	work, cancel := context.WithCancel(context.WithoutCancel(ctx))
	uploads := &Uploads{entries: map[string]*uploadEntry{}, storage: storage, metadata: metadata, prefix: prefix, directory: directory,
		ctx: work, cancel: cancel, logger: logger, now: time.Now, wake: make(chan struct{}, 1),
		settings: UploadSettings{Enabled: true, Transport: "cos", UploadOrigin: origin, PartSize: UploadPartSize, MaxFileSize: FileLimit}}
	if storage == nil {
		uploads.settings.Transport = "direct"
	}
	for _, record := range records {
		if record.StorageIdentity != uploads.storageIdentity() && record.CleanupStatus != "completed" {
			cancel()
			return nil, ErrUploadStorageChanged
		}
		parsed, err := uuid.Parse(record.ID)
		validKey := record.Key == prefix+record.ID+".json"
		if record.CleanupStatus == "completed" {
			// Historical results never access COS again, including after a prefix change.
			originalPrefix, found := strings.CutSuffix(record.Key, record.ID+".json")
			validKey = found && storedUploadPrefix.MatchString(originalPrefix) && !strings.Contains(originalPrefix, "//") && uploadTerminal(record.State)
		}
		if err != nil || parsed.String() != record.ID || !validKey {
			cancel()
			return nil, ErrInvalid
		}
		switch record.State {
		case "restoring":
			record.State, record.ErrorCode, record.CleanupStatus = "unknown", "restore_outcome_unknown", "pending"
			if record.Job != nil {
				record.Job.Status, record.Job.ErrorCode = "unknown", "restore_outcome_unknown"
			}
		case "preparing":
			record.State, record.ErrorCode = "preparation_failed", "database_backup_preparation_interrupted"
		case "inspecting":
			record.State = "ready"
		}
		if err := metadata.Save(ctx, record); err != nil {
			cancel()
			return nil, err
		}
		uploads.entries[record.ID] = &uploadEntry{record: record}
	}
	var resume []*uploadEntry
	for _, entry := range uploads.entries {
		if !uploadTerminal(entry.record.State) && !uploads.now().Before(entry.record.ExpiresAt) {
			entry.record.State, entry.record.CleanupStatus = "expired", "pending"
			if err := metadata.Save(ctx, entry.record); err != nil {
				cancel()
				return nil, err
			}
		}
		if entry.record.State == "ready" || entry.record.State == "inspected" {
			// Redis readiness cannot attest a cache left by a killed process.
			// Always verify the immutable COS version again after restart.
			entry.record.State, entry.record.PreparedBytes = "preparing", 0
			if err := metadata.Save(ctx, entry.record); err != nil {
				cancel()
				return nil, err
			}
			resume = append(resume, entry)
		}
	}
	if uploads.settings.Transport == "direct" {
		resume = nil
		for _, entry := range uploads.entries {
			if uploadTerminal(entry.record.State) {
				continue
			}
			if err := uploads.recoverDirect(ctx, entry); err != nil {
				cancel()
				return nil, err
			}
		}
	}
	if err := uploads.pruneLocalOrphans(); err != nil {
		cancel()
		return nil, err
	}
	for _, entry := range resume {
		uploads.startPreparation(entry)
	}
	uploads.workers.Add(1)
	go uploads.sweepLoop()
	return uploads, nil
}

func (uploads *Uploads) Settings() UploadSettings { return uploads.settings }

func (uploads *Uploads) Current(ctx context.Context, owner string) (*UploadSession, error) {
	if !uploads.settings.Enabled {
		return nil, nil
	}
	uploads.mu.Lock()
	var latest *uploadEntry
	for _, entry := range uploads.entries {
		if entry.record.Owner == owner && (latest == nil || entry.record.CreatedAt.After(latest.record.CreatedAt)) {
			latest = entry
		}
	}
	if latest == nil {
		uploads.mu.Unlock()
		return nil, nil
	}
	id := latest.record.ID
	uploads.mu.Unlock()
	session, err := uploads.Get(ctx, owner, id)
	return &session, err
}

func (uploads *Uploads) entry(owner, id string) (*uploadEntry, error) {
	if !uploads.settings.Enabled {
		return nil, ErrUploadUnavailable
	}
	if uploads.closed {
		return nil, ErrClosed
	}
	entry := uploads.entries[id]
	if entry == nil || entry.record.Owner != owner {
		return nil, ErrUploadNotFound
	}
	if !uploadTerminal(entry.record.State) && !entry.leased && !entry.worker && len(entry.writers) == 0 && !uploads.now().Before(entry.record.ExpiresAt) {
		entry.record.State, entry.record.CleanupStatus = "expired", "pending"
		if err := uploads.saveDetached(entry.record); err != nil {
			return nil, err
		}
	}
	return entry, nil
}

func (uploads *Uploads) storageIdentity() string {
	if uploads.settings.Transport == "direct" {
		return "direct:" + uploads.directory
	}
	return uploads.storage.Identity()
}
