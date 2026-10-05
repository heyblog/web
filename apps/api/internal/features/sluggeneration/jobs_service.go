package sluggeneration

import (
	"context"
	"strconv"
	"strings"
	"time"

	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/config"
)

type JobService struct {
	generation *Service
	store      JobStore
	config     config.AIBatchConfig
	now        func() time.Time
}

func NewJobService(service *Service, store JobStore, configuration config.AIBatchConfig) *JobService {
	if configuration.Size == 0 {
		configuration = config.AIBatchConfig{Size: 10, MaxTags: 500, MaxOutputTokens: 4096}
	}
	return &JobService{generation: service, store: store, config: configuration, now: time.Now}
}
func publicJob(job jobRecord) Job {
	result := Job{ID: job.ID, Status: job.Status, Revision: strconv.FormatInt(job.Revision, 10), ModelID: job.ModelID, PauseCode: job.PauseCode, Items: make([]JobItem, 0, len(job.Items))}
	if !job.ResumeAfter.IsZero() {
		result.ResumeAfter = job.ResumeAfter.UTC().Format(time.RFC3339)
	}
	result.Counts.Total = len(job.Items)
	for _, item := range job.Items {
		result.Items = append(result.Items, item.JobItem)
		switch item.State {
		case "ready":
			result.Counts.Ready++
		case "failed", "stale":
			result.Counts.Failed++
		case "applied":
			result.Counts.Applied++
		}
	}
	return result
}
func checkJobRevision(job jobRecord, revision string) error {
	if revision == "" || revision != strconv.FormatInt(job.Revision, 10) {
		return jobFailure("slug_job_changed")
	}
	return nil
}
func validCandidate(slug string) bool {
	return len(slug) <= 128 && simpleName.MatchString(slug) && slug == strings.ToLower(slug) && !strings.Contains(slug, " ") && !strings.Contains(slug, "--") && !strings.HasPrefix(slug, "-") && !strings.HasSuffix(slug, "-")
}

func (service *JobService) Create(ctx context.Context, identity Identity, input CreateJobInput) (Job, error) {
	if err := service.generation.guard.Allow(ctx, identity); err != nil {
		return Job{}, err
	}
	selection := input.Selection
	switch selection.Kind {
	case "invalid", "all":
		if len(selection.IDs) != 0 || selection.Filter != nil {
			return Job{}, jobFailure("invalid_job_selection")
		}
	case "ids":
		if len(selection.IDs) == 0 || selection.Filter != nil {
			return Job{}, jobFailure("invalid_job_selection")
		}
		seen := make(map[string]bool, len(selection.IDs))
		for _, id := range selection.IDs {
			if seen[id] {
				return Job{}, jobFailure("slug_job_duplicate_item")
			}
			seen[id] = true
		}
	case "filter":
		if selection.Filter == nil || len(selection.IDs) != 0 {
			return Job{}, jobFailure("invalid_job_selection")
		}
	default:
		return Job{}, apperror.New(apperror.KindValidation, "invalid_job_selection", "select a supported tag selection")
	}
	settings, err := service.generation.Settings(ctx)
	if err != nil {
		return Job{}, err
	}
	job, err := service.store.Create(ctx, identity, selection, settings.ModelID, service.config.MaxTags)
	return publicJob(job), err
}
func (service *JobService) Get(ctx context.Context, id string, identity Identity) (Job, error) {
	job, err := service.store.Get(ctx, id, identity)
	return publicJob(job), err
}
func (service *JobService) List(ctx context.Context, identity Identity) ([]Job, error) {
	rows, err := service.store.List(ctx, identity)
	if err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, publicJob(row))
	}
	return jobs, nil
}
func (service *JobService) Control(ctx context.Context, id string, identity Identity, input JobControlInput) (Job, error) {
	if err := service.generation.guard.Allow(ctx, identity); err != nil {
		return Job{}, err
	}
	job, err := service.store.Mutate(ctx, id, identity, input.ExpectedRevision, func(job *jobRecord) error {
		if job.Status == "cancelled" || job.Status == "completed" {
			return jobFailure("slug_job_finished")
		}
		switch input.Action {
		case "pause":
			if job.Status != "queued" && job.Status != "running" {
				return jobFailure("slug_job_not_running")
			}
			job.Status = "paused"
			job.PauseCode = "slug_job_paused"
		case "cancel":
			job.Status = "cancelled"
			job.PauseCode = ""
		case "resume":
			if job.Status != "paused" || service.now().Before(job.ResumeAfter) {
				return jobFailure("slug_job_cannot_resume")
			}
			job.Status = "queued"
			job.PauseCode = ""
			job.ResumeAfter = time.Time{}
		case "retry":
			if job.Status != "ready" && job.Status != "paused" {
				return jobFailure("slug_job_not_ready")
			}
			if service.now().Before(job.ResumeAfter) {
				return jobFailure("slug_job_cannot_resume")
			}
			retries := 0
			for i := range job.Items {
				if job.Items[i].State == "failed" {
					job.Items[i].State = "pending"
					job.Items[i].ErrorCode = ""
					job.Items[i].Slug = ""
					retries++
				}
			}
			if retries == 0 {
				return jobFailure("slug_job_no_failed_items")
			}
			job.Status = "queued"
			job.PauseCode = ""
			job.ResumeAfter = time.Time{}
		default:
			return apperror.New(apperror.KindValidation, "invalid_job_action", "select a supported task action")
		}
		for i := range job.Items {
			if job.Items[i].State == "running" {
				job.Items[i].State = "failed"
				job.Items[i].ErrorCode = "slug_generation_interrupted"
			}
		}
		return nil
	})
	return publicJob(job), err
}
func (service *JobService) Edit(ctx context.Context, id string, identity Identity, input JobEditInput) (Job, error) {
	if err := service.generation.guard.Allow(ctx, identity); err != nil {
		return Job{}, err
	}
	job, err := service.store.Mutate(ctx, id, identity, input.ExpectedRevision, func(job *jobRecord) error {
		if job.Status != "ready" {
			return jobFailure("slug_job_not_ready")
		}
		if err := checkJobRevision(*job, input.ExpectedRevision); err != nil {
			return err
		}
		edits := make(map[string]string, len(input.Items))
		for _, edit := range input.Items {
			if !validCandidate(edit.Slug) {
				return jobFailure("invalid_slug")
			}
			if _, exists := edits[edit.TagID]; exists {
				return jobFailure("slug_job_duplicate_item")
			}
			edits[edit.TagID] = edit.Slug
		}
		for i, item := range job.Items {
			if slug, ok := edits[item.TagID]; ok {
				if item.State != "ready" {
					return jobFailure("slug_job_item_not_ready")
				}
				job.Items[i].Slug = slug
				job.Items[i].Source = "manual"
				delete(edits, item.TagID)
			}
		}
		if len(edits) != 0 {
			return jobFailure("slug_job_item_not_found")
		}
		seen := map[string]bool{}
		for _, item := range job.Items {
			if item.State != "ready" {
				continue
			}
			if seen[item.Slug] {
				return jobFailure("slug_conflict")
			}
			seen[item.Slug] = true
			occupied, checkErr := service.generation.store.Occupied(ctx, item.Slug, item.TagID)
			if checkErr != nil {
				return unavailable()
			}
			if occupied {
				return jobFailure("slug_conflict")
			}
		}
		return nil
	})
	return publicJob(job), err
}
func (service *JobService) Apply(ctx context.Context, id string, identity Identity, input JobApplyInput) (Job, error) {
	if len(input.TagIDs) == 0 {
		return Job{}, jobFailure("slug_job_empty")
	}
	if err := service.generation.guard.Allow(ctx, identity); err != nil {
		return Job{}, err
	}
	job, err := service.store.Apply(ctx, id, identity, input)
	return publicJob(job), err
}
