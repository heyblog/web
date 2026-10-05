package sluggeneration

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func (repository *JobRepository) Claim(ctx context.Context, token string) (jobRecord, bool, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return jobRecord{}, false, jobDatabaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := dbgen.New(tx)
	row, err := q.NextSlugJob(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return jobRecord{}, false, nil
	}
	if err != nil {
		return jobRecord{}, false, jobDatabaseError(err)
	}
	job, err := decodeJob(row)
	if err != nil {
		return jobRecord{}, false, err
	}
	// Never automatically resend a dispatched request whose outcome is unknown.
	for i := range job.Items {
		if job.Items[i].State == "running" {
			job.Items[i].State = "failed"
			job.Items[i].ErrorCode = "slug_generation_interrupted"
		}
	}
	items, err := json.Marshal(job.Items)
	if err != nil {
		return jobRecord{}, false, err
	}
	row, err = q.LeaseSlugJob(ctx, dbgen.LeaseSlugJobParams{ID: row.ID, Token: token, Items: items})
	if err != nil {
		return jobRecord{}, false, jobDatabaseError(err)
	}
	job, err = decodeJob(row)
	if err != nil {
		return jobRecord{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return jobRecord{}, false, jobDatabaseError(err)
	}
	return job, true, nil
}
func (repository *JobRepository) SaveClaim(ctx context.Context, job jobRecord) (bool, error) {
	id, err := jobID(job.ID)
	if err != nil {
		return false, err
	}
	items, err := json.Marshal(job.Items)
	if err != nil {
		return false, err
	}
	_, err = dbgen.New(repository.pool).FinishSlugJobClaim(ctx, dbgen.FinishSlugJobClaimParams{ID: id, Token: job.LeaseToken, ExpectedRevision: job.Revision, Status: job.Status, Items: items, PauseCode: job.PauseCode, ResumeAfter: pgtype.Timestamptz{Time: job.ResumeAfter, Valid: !job.ResumeAfter.IsZero()}})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, jobDatabaseError(err)
}
func (repository *JobRepository) Authorized(ctx context.Context, value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	id, err := jobID(value)
	if err != nil {
		return false, err
	}
	allowed, err := dbgen.New(repository.pool).SlugJobActorAuthorized(ctx, id)
	return allowed, jobDatabaseError(err)
}
