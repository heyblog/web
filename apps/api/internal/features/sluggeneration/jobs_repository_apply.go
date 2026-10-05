package sluggeneration

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func (repository *JobRepository) Apply(ctx context.Context, id string, identity Identity, input JobApplyInput) (jobRecord, error) {
	return repository.applyTransaction(ctx, id, identity, input)
}

func (repository *JobRepository) applyTransaction(ctx context.Context, value string, identity Identity, input JobApplyInput) (jobRecord, error) {
	id, err := jobID(value)
	if err != nil {
		return jobRecord{}, err
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return jobRecord{}, jobDatabaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := dbgen.New(tx)
	// All dictionary writers serialize on this lock, including alias reservations.
	if err = q.LockTaxonomy(ctx); err != nil {
		return jobRecord{}, jobDatabaseError(err)
	}
	row, err := q.LockSlugJob(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return jobRecord{}, jobFailure("slug_job_not_found")
	}
	if err != nil {
		return jobRecord{}, jobDatabaseError(err)
	}
	job, err := decodeJob(row)
	if err != nil {
		return jobRecord{}, err
	}
	if err = authorizeJob(job, identity); err != nil {
		return jobRecord{}, err
	}
	if err = checkJobRevision(job, input.ExpectedRevision); err != nil {
		return jobRecord{}, err
	}
	if job.Status != "ready" {
		return jobRecord{}, jobFailure("slug_job_not_ready")
	}
	selected := make(map[string]bool, len(input.TagIDs))
	for _, id := range input.TagIDs {
		if selected[id] {
			return jobRecord{}, jobFailure("slug_job_duplicate_item")
		}
		selected[id] = true
	}
	applied := 0
	for index, item := range job.Items {
		if !selected[item.TagID] {
			continue
		}
		if item.State != "ready" || !validCandidate(item.Slug) {
			return jobRecord{}, jobFailure("slug_job_item_not_ready")
		}
		tagID, parseErr := jobID(item.TagID)
		if parseErr != nil {
			return jobRecord{}, parseErr
		}
		tag, readErr := q.LockSlugJobTag(ctx, tagID)
		if errors.Is(readErr, pgx.ErrNoRows) {
			return jobRecord{}, jobFailure("slug_job_tags_changed")
		}
		if readErr != nil {
			return jobRecord{}, jobDatabaseError(readErr)
		}
		if tag.Name != item.Name || tag.Description != item.Description || tag.Slug != item.OriginalSlug || !tag.UpdatedAt.Time.Equal(item.UpdatedAt) {
			return jobRecord{}, jobFailure("slug_job_tags_changed")
		}
		owners, readErr := q.TaxonomySlugOwner(ctx, item.Slug)
		if readErr != nil {
			return jobRecord{}, jobDatabaseError(readErr)
		}
		for _, owner := range owners {
			if owner != tagID {
				return jobRecord{}, jobFailure("slug_conflict")
			}
		}
		if tag.Slug != item.Slug {
			if err = q.ReserveTaxonomySlug(ctx, dbgen.ReserveTaxonomySlugParams{Slug: tag.Slug, TagID: tagID}); err != nil {
				return jobRecord{}, jobDatabaseError(err)
			}
			if err = q.SetSlugJobTag(ctx, dbgen.SetSlugJobTagParams{ID: tagID, Slug: item.Slug}); err != nil {
				return jobRecord{}, jobDatabaseError(err)
			}
		}
		job.Items[index].State = "applied"
		applied++
	}
	if applied != len(selected) {
		return jobRecord{}, jobFailure("slug_job_item_not_found")
	}
	allApplied := true
	for _, item := range job.Items {
		if item.State != "applied" {
			allApplied = false
		}
	}
	if allApplied {
		job.Status = "completed"
	}
	updated, err := saveJob(ctx, q, job)
	if err != nil {
		return jobRecord{}, jobDatabaseError(err)
	}
	return updated, jobDatabaseError(tx.Commit(ctx))
}
