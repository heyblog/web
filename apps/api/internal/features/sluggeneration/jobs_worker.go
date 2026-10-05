package sluggeneration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"heyblog-api/internal/platform/apperror"
)

// RunJobs is owned by bootstrap; cancellation completes before dependency pools close.
func (service *Service) RunJobs(ctx context.Context, logger *slog.Logger) {
	if service.jobs == nil {
		return
	}
	service.jobs.Run(ctx, logger)
}
func (service *JobService) Run(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if err := service.Step(ctx); err != nil && ctx.Err() == nil {
			logger.ErrorContext(ctx, "slug task worker failed", "event", "slug_job_worker_failed", "error_type", errorType(err))
		}
	}
}
func errorType(err error) string {
	var business *apperror.Error
	if errors.As(err, &business) {
		return business.Code()
	}
	return "slug_job_unavailable"
}
func (service *JobService) save(ctx context.Context, job *jobRecord) (bool, error) {
	ok, err := service.store.SaveClaim(ctx, *job)
	if ok {
		job.Revision++
	}
	return ok, err
}
func (service *JobService) Step(ctx context.Context) error {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return err
	}
	job, claimed, err := service.store.Claim(ctx, hex.EncodeToString(bytes))
	if err != nil || !claimed {
		return err
	}
	allowed, err := service.store.Authorized(ctx, job.OwnerID)
	if err != nil {
		return err
	}
	if !allowed {
		job.Status = "paused"
		job.PauseCode = "slug_job_permission_revoked"
		_, err = service.save(ctx, &job)
		return err
	}
	if err = service.generateChunk(ctx, &job); err != nil {
		job.Status = "paused"
		job.PauseCode = errorType(err)
		if seconds, parseErr := strconv.ParseInt(retryAfter(err), 10, 64); parseErr == nil {
			job.ResumeAfter = service.now().Add(time.Duration(seconds) * time.Second)
		}
		if job.PauseCode == "generation_in_progress" {
			job.ResumeAfter = service.now().Add(time.Second)
		}
		_, saveErr := service.save(ctx, &job)
		return saveErr
	}
	if job.Status != "running" {
		return nil
	}
	pending := false
	for _, item := range job.Items {
		if item.State == "pending" {
			pending = true
		}
	}
	if pending {
		job.Status = "queued"
	} else {
		if err = service.allocate(ctx, &job); err != nil {
			return err
		}
		job.Status = "ready"
	}
	_, err = service.save(ctx, &job)
	return err
}
