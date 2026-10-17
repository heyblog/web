package sitemanagement

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type attachmentCleanupStore interface {
	ListAttachmentCleanup(context.Context) ([]AttachmentCleanupItem, error)
	MarkAttachmentArchived(context.Context, string) error
	MarkAttachmentDeleted(context.Context, string) error
	MarkAttachmentUploadCleaned(context.Context, string) error
}

// AttachmentCleanupWorker completes the asynchronous half of the evidence
// lifecycle. Database triggers record cleanup/retention intent transactionally;
// this worker performs the remote COS operation and only then advances the row.
type AttachmentCleanupWorker struct {
	store   attachmentCleanupStore
	storage *EvidenceStorage
	logger  *slog.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	once    sync.Once
}

func NewAttachmentCleanupWorker(parent context.Context, store attachmentCleanupStore, storage *EvidenceStorage, logger *slog.Logger) *AttachmentCleanupWorker {
	ctx, cancel := context.WithCancel(parent)
	worker := &AttachmentCleanupWorker{store: store, storage: storage, logger: logger, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	go worker.loop()
	return worker
}

func (worker *AttachmentCleanupWorker) loop() {
	defer close(worker.done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		worker.sweep()
		select {
		case <-worker.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (worker *AttachmentCleanupWorker) sweep() {
	if worker.store == nil || worker.storage == nil || !worker.storage.Enabled() {
		return
	}
	items, err := worker.store.ListAttachmentCleanup(worker.ctx)
	if err != nil {
		worker.report("list", "")
		return
	}
	for _, item := range items {
		if worker.ctx.Err() != nil {
			return
		}
		ctx, cancel := context.WithTimeout(worker.ctx, time.Minute)
		var operationErr error
		key, keyErr := worker.storage.Key(item.ID)
		if keyErr != nil || key != item.StorageKey || item.StorageIdentity != worker.storage.origin {
			cancel()
			worker.report("storage_mismatch", item.ID)
			continue
		}
		if item.CleanupUpload {
			operationErr = worker.storage.CleanupKey(ctx, key+"-upload")
			if operationErr == nil {
				operationErr = worker.store.MarkAttachmentUploadCleaned(ctx, item.ID)
			}
		}
		if operationErr != nil {
			cancel()
			worker.report("upload_cleanup", item.ID)
			continue
		}
		switch item.Status {
		case AttachmentCleanupPending, AttachmentArchived:
			operationErr = worker.storage.CleanupKey(ctx, item.StorageKey)
			if operationErr == nil {
				operationErr = worker.store.MarkAttachmentDeleted(ctx, item.ID)
			}
		case AttachmentBound:
			operationErr = worker.storage.Archive(ctx, item.StorageKey)
			if operationErr == nil {
				operationErr = worker.store.MarkAttachmentArchived(ctx, item.ID)
			}
		}
		cancel()
		if operationErr != nil {
			worker.report("process", item.ID)
		}
	}
}

func (worker *AttachmentCleanupWorker) report(stage, id string) {
	if worker.logger != nil {
		worker.logger.Warn("manual claim evidence lifecycle operation failed", "event", "manual_claim_attachment_operation_failed", "stage", stage, "attachment_id", id)
	}
}

func (worker *AttachmentCleanupWorker) Close() {
	if worker == nil {
		return
	}
	worker.once.Do(func() {
		worker.cancel()
		<-worker.done
	})
}
