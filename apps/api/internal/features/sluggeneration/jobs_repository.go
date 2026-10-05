package sluggeneration

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/platform/apperror"
)

type JobRepository struct{ pool *pgxpool.Pool }

func NewJobRepository(pool *pgxpool.Pool) *JobRepository { return &JobRepository{pool: pool} }

func jobID(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil || !id.Valid {
		return id, apperror.New(apperror.KindValidation, "invalid_job_id", "a valid identifier is required")
	}
	return id, nil
}
func jobFailure(code string) error {
	return apperror.New(apperror.KindConflict, code, "reload the slug preview before continuing")
}
func jobDatabaseError(err error) error {
	if err == nil {
		return nil
	}
	var business *apperror.Error
	if errors.As(err, &business) {
		return err
	}
	var failure *pgconn.PgError
	if errors.As(err, &failure) && failure.Code == "23505" {
		return jobFailure("slug_job_conflict")
	}
	return apperror.Wrap(err, apperror.KindUnavailable, "slug_job_unavailable", "slug tasks are temporarily unavailable", "slug job storage")
}
func decodeJob(row dbgen.DirectorySlugGenerationJob) (jobRecord, error) {
	job := jobRecord{ID: row.ID.String(), OwnerID: row.OwnerID.String(), IPHash: row.IdentityIpHash, ModelID: row.ModelID, Status: row.Status, Revision: row.Revision, PauseCode: row.PauseCode, LeaseToken: row.LeaseToken}
	if row.ResumeAfter.Valid {
		job.ResumeAfter = row.ResumeAfter.Time
	}
	if err := json.Unmarshal(row.Items, &job.Items); err != nil {
		return jobRecord{}, jobDatabaseError(err)
	}
	return job, nil
}
func authorizeJob(job jobRecord, identity Identity) error {
	if !identity.SystemAdmin && job.OwnerID != identity.UserID {
		return apperror.New(apperror.KindNotFound, "slug_job_not_found", "the slug task was not found")
	}
	return nil
}
func (repository *JobRepository) Create(ctx context.Context, identity Identity, selection JobSelection, model string, cap int) (jobRecord, error) {
	if cap < 1 || cap > 500 {
		return jobRecord{}, apperror.New(apperror.KindValidation, "slug_job_too_large", "the task limit must be between 1 and 500")
	}
	owner, err := jobID(identity.UserID)
	if err != nil {
		return jobRecord{}, err
	}
	ids := make([]pgtype.UUID, 0, len(selection.IDs))
	for _, value := range selection.IDs {
		id, parseErr := jobID(value)
		if parseErr != nil {
			return jobRecord{}, parseErr
		}
		ids = append(ids, id)
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return jobRecord{}, jobDatabaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := dbgen.New(tx)
	if err = q.LockTaxonomy(ctx); err != nil {
		return jobRecord{}, jobDatabaseError(err)
	}
	params := dbgen.SelectSlugJobTagsParams{Kind: selection.Kind, Ids: ids, MaxRows: int32(cap + 1)}
	if selection.Filter != nil {
		params.Query = selection.Filter.Query
		params.Enabled = selection.Filter.Enabled
	}
	tags, err := q.SelectSlugJobTags(ctx, params)
	if err != nil {
		return jobRecord{}, jobDatabaseError(err)
	}
	if len(tags) > cap {
		return jobRecord{}, apperror.New(apperror.KindValidation, "slug_job_too_large", "the tag selection exceeds the configured task limit")
	}
	if len(tags) == 0 {
		return jobRecord{}, apperror.New(apperror.KindValidation, "slug_job_empty", "select at least one matching tag")
	}
	if selection.Kind == "ids" && len(tags) != len(ids) {
		return jobRecord{}, jobFailure("slug_job_tags_changed")
	}
	items := make([]jobItemRecord, 0, len(tags))
	for _, tag := range tags {
		items = append(items, jobItemRecord{JobItem: JobItem{TagID: tag.ID.String(), Name: tag.Name, OriginalSlug: tag.Slug, State: "pending"}, Description: tag.Description, UpdatedAt: tag.UpdatedAt.Time})
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return jobRecord{}, jobDatabaseError(err)
	}
	row, err := q.InsertSlugJob(ctx, dbgen.InsertSlugJobParams{OwnerID: owner, IdentityIpHash: digestIdentity(identity.IP), ModelID: model, Items: encoded})
	if err != nil {
		return jobRecord{}, jobDatabaseError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return jobRecord{}, jobDatabaseError(err)
	}
	return decodeJob(row)
}
func (repository *JobRepository) Get(ctx context.Context, value string, identity Identity) (jobRecord, error) {
	id, err := jobID(value)
	if err != nil {
		return jobRecord{}, err
	}
	row, err := dbgen.New(repository.pool).GetSlugJob(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return jobRecord{}, apperror.New(apperror.KindNotFound, "slug_job_not_found", "the slug task was not found")
	}
	if err != nil {
		return jobRecord{}, jobDatabaseError(err)
	}
	job, err := decodeJob(row)
	if err != nil {
		return jobRecord{}, err
	}
	return job, authorizeJob(job, identity)
}
func (repository *JobRepository) List(ctx context.Context, identity Identity) ([]jobRecord, error) {
	owner, err := jobID(identity.UserID)
	if err != nil {
		return nil, err
	}
	rows, err := dbgen.New(repository.pool).ListSlugJobs(ctx, dbgen.ListSlugJobsParams{OwnerID: owner, SystemAdmin: identity.SystemAdmin})
	if err != nil {
		return nil, jobDatabaseError(err)
	}
	jobs := make([]jobRecord, 0, len(rows))
	for _, row := range rows {
		job, decodeErr := decodeJob(row)
		if decodeErr != nil {
			return nil, decodeErr
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}
func saveJob(ctx context.Context, q *dbgen.Queries, job jobRecord) (jobRecord, error) {
	id, err := jobID(job.ID)
	if err != nil {
		return jobRecord{}, err
	}
	items, err := json.Marshal(job.Items)
	if err != nil {
		return jobRecord{}, err
	}
	row, err := q.UpdateSlugJob(ctx, dbgen.UpdateSlugJobParams{ID: id, ExpectedRevision: job.Revision, Status: job.Status, Items: items, PauseCode: job.PauseCode, ResumeAfter: pgtype.Timestamptz{Time: job.ResumeAfter, Valid: !job.ResumeAfter.IsZero()}})
	if errors.Is(err, pgx.ErrNoRows) {
		return jobRecord{}, jobFailure("slug_job_changed")
	}
	if err != nil {
		return jobRecord{}, err
	}
	return decodeJob(row)
}
func (repository *JobRepository) Mutate(ctx context.Context, value string, identity Identity, revision string, fn func(*jobRecord) error) (jobRecord, error) {
	id, err := jobID(value)
	if err != nil {
		return jobRecord{}, err
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return jobRecord{}, jobDatabaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row, err := dbgen.New(tx).LockSlugJob(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return jobRecord{}, apperror.New(apperror.KindNotFound, "slug_job_not_found", "the slug task was not found")
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
	if revision != "" && revision != strconv.FormatInt(job.Revision, 10) {
		return jobRecord{}, jobFailure("slug_job_changed")
	}
	if err = fn(&job); err != nil {
		return jobRecord{}, err
	}
	updated, err := saveJob(ctx, dbgen.New(tx), job)
	if err != nil {
		return jobRecord{}, jobDatabaseError(err)
	}
	return updated, jobDatabaseError(tx.Commit(ctx))
}
