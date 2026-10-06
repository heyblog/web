package sluggeneration

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/siteaudit"
	"heyblog-api/internal/infrastructure/tokenhub"
	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/config"
)

type memoryJobs struct {
	job        jobRecord
	allowed    bool
	claimable  bool
	saves      int
	dispatched bool
}

func (store *memoryJobs) Create(context.Context, Identity, JobSelection, string, int) (jobRecord, error) {
	return store.job, nil
}
func (store *memoryJobs) Get(context.Context, string, Identity) (jobRecord, error) {
	return store.job, nil
}
func (store *memoryJobs) List(context.Context, Identity) ([]jobRecord, error) {
	return []jobRecord{store.job}, nil
}
func (store *memoryJobs) Apply(context.Context, string, Identity, JobApplyInput) (jobRecord, error) {
	return store.job, nil
}
func (store *memoryJobs) Mutate(_ context.Context, _ string, _ Identity, revision string, fn func(*jobRecord) error) (jobRecord, error) {
	if revision != "" && revision != strconv.FormatInt(store.job.Revision, 10) {
		return jobRecord{}, jobFailure("slug_job_changed")
	}
	job := store.job
	job.Items = append([]jobItemRecord(nil), job.Items...)
	if err := fn(&job); err != nil {
		return jobRecord{}, err
	}
	job.Revision++
	store.job = job
	return job, nil
}
func (store *memoryJobs) Claim(_ context.Context, token string) (jobRecord, bool, error) {
	if !store.claimable {
		return jobRecord{}, false, nil
	}
	store.claimable = false
	store.job.Status = "running"
	store.job.LeaseToken = token
	store.job.Revision++
	return store.job, true, nil
}
func (store *memoryJobs) SaveClaim(_ context.Context, job jobRecord) (bool, error) {
	store.saves++
	for _, item := range job.Items {
		if item.State == "running" {
			store.dispatched = true
		}
	}
	if job.Revision != store.job.Revision {
		return false, nil
	}
	job.Revision++
	store.job = job
	return true, nil
}
func (store *memoryJobs) Authorized(context.Context, string) (bool, error) { return store.allowed, nil }

type jobProvider struct {
	*testProvider
	jobs       *memoryJobs
	batchCalls int
	err        error
	invalid    bool
}

func (provider *jobProvider) GenerateBatch(_ context.Context, _ string, inputs []tokenhub.BatchInput) ([]tokenhub.BatchResult, error) {
	provider.batchCalls++
	if !provider.jobs.dispatched {
		return nil, errors.New("dispatch was not persisted")
	}
	results := make([]tokenhub.BatchResult, 0, len(inputs))
	for _, input := range inputs {
		results = append(results, tokenhub.BatchResult{ID: input.ID, Slug: "life"})
	}
	if provider.invalid {
		results[0].ID = "unknown"
	}
	return results, provider.err
}
func jobFixture() (*JobService, *memoryJobs, *jobProvider, *memoryStore, *testGuard) {
	generation, cache, provider, guard := testService()
	jobs := &memoryJobs{allowed: true, claimable: true, job: jobRecord{ID: "job", OwnerID: "user", ModelID: "deepseek/deepseek-flash", Revision: 1, Items: []jobItemRecord{
		{JobItem: JobItem{TagID: "b", Name: "生活记录", State: "pending"}},
		{JobItem: JobItem{TagID: "a", Name: "生活", State: "pending"}},
	}}}
	batch := &jobProvider{testProvider: provider, jobs: jobs}
	generation.provider = batch
	service := NewJobService(generation, jobs, config.AIBatchConfig{Size: 10, MaxTags: 500, MaxOutputTokens: 4096})
	return service, jobs, batch, cache, guard
}
func TestJobBatchesPersistDispatchAndAllocateStableCandidates(t *testing.T) {
	service, jobs, provider, cache, guard := jobFixture()
	cache.occupied["life"] = "existing"
	if err := service.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.batchCalls != 1 || guard.charged != 1 || guard.released != 1 || !jobs.dispatched {
		t.Fatalf("paid dispatch calls=%d guard=%+v", provider.batchCalls, guard)
	}
	if jobs.job.Status != "ready" || jobs.job.Items[0].TagID != "a" || jobs.job.Items[0].Slug != "life" || jobs.job.Items[1].Slug != "life" || jobs.job.Items[0].State != "needs_confirmation" || jobs.job.Items[1].State != "needs_confirmation" {
		t.Fatalf("preview=%+v", jobs.job)
	}
	if jobs.saves != 3 {
		t.Fatalf("saves=%d want dispatch and completion", jobs.saves)
	}
}
func TestJobCacheAndLocalGenerationSpendNoBudget(t *testing.T) {
	service, jobs, provider, cache, guard := jobFixture()
	jobs.job.Items[0].Name = "Go Tools"
	key, err := cacheKey(jobs.job.ModelID, tokenhub.Input{Name: "生活"})
	if err != nil {
		t.Fatal(err)
	}
	cache.cache[key] = "life"
	if err = service.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.batchCalls != 0 || guard.charged != 0 || jobs.job.Status != "ready" {
		t.Fatalf("job=%+v paid=%d", jobs.job, guard.charged)
	}
}
func TestJobBudgetPausesBeforeDispatchAndPermissionRevocationStopsWorker(t *testing.T) {
	service, jobs, provider, _, guard := jobFixture()
	guard.err = rateFailure("slug_daily_limit", 120)
	if err := service.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if jobs.job.Status != "paused" || jobs.job.PauseCode != "slug_daily_limit" || jobs.job.ResumeAfter.IsZero() || provider.batchCalls != 0 {
		t.Fatalf("job=%+v", jobs.job)
	}
	service, jobs, provider, _, guard = jobFixture()
	jobs.allowed = false
	if err := service.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if jobs.job.PauseCode != "slug_job_permission_revoked" || guard.charged != 0 || provider.batchCalls != 0 {
		t.Fatalf("job=%+v", jobs.job)
	}
}
func TestInvalidPaidBatchRequiresExplicitRetry(t *testing.T) {
	service, jobs, provider, _, guard := jobFixture()
	provider.invalid = true
	if err := service.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if jobs.job.Status != "ready" || publicJob(jobs.job).Counts.Failed != 2 || guard.charged != 1 {
		t.Fatalf("job=%+v", jobs.job)
	}
	if err := service.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.batchCalls != 1 {
		t.Fatal("failed paid dispatch was automatically retried")
	}
	if _, err := service.Control(context.Background(), "job", Identity{}, JobControlInput{Action: "retry", ExpectedRevision: strconv.FormatInt(jobs.job.Revision, 10)}); err != nil {
		t.Fatal(err)
	}
	if jobs.job.Status != "queued" || jobs.job.Items[0].State != "pending" {
		t.Fatalf("retry=%+v", jobs.job)
	}
}
func TestTaskPreviewRevisionCollisionAndPauseControls(t *testing.T) {
	service, jobs, _, cache, _ := jobFixture()
	jobs.job.Status = "ready"
	jobs.job.Items[0].State = "ready"
	jobs.job.Items[0].Slug = "first"
	jobs.job.Items[1].State = "ready"
	jobs.job.Items[1].Slug = "second"
	_, err := service.Edit(context.Background(), "job", Identity{}, JobEditInput{ExpectedRevision: "0", Items: []JobEdit{{TagID: "a", Slug: "custom"}}})
	if err == nil {
		t.Fatal("stale edit accepted")
	}
	cache.occupied["custom"] = "owner"
	_, err = service.Edit(context.Background(), "job", Identity{}, JobEditInput{ExpectedRevision: "1", Items: []JobEdit{{TagID: "a", Slug: "custom"}}})
	if err == nil {
		t.Fatal("occupied candidate accepted")
	}
	jobs.job.Status = "paused"
	jobs.job.ResumeAfter = time.Now().Add(time.Hour)
	_, err = service.Control(context.Background(), "job", Identity{}, JobControlInput{Action: "resume"})
	if err == nil {
		t.Fatal("budget pause resumed early")
	}
}
func TestReviewAutomaticallyGeneratesMissingSlugAndRequiresTaxonomyPermission(t *testing.T) {
	service, _, provider, guard := testService()
	tags := []siteaudit.TagSnapshot{{SuggestedName: "编程", Level: 3, Role: "TERTIARY"}}
	_, err := service.ReviewedTags(context.Background(), auth.User{ID: "user", Role: auth.RoleAdmin}, "ip", tags)
	var failure *apperror.Error
	if !errors.As(err, &failure) || failure.Kind() != apperror.KindForbidden || provider.calls != 0 {
		t.Fatalf("unauthorized=%v", err)
	}
	result, err := service.ReviewedTags(context.Background(), auth.User{ID: "user", Role: auth.RoleSysAdmin}, "ip", tags)
	if err != nil || result[0].Slug != "programming" || tags[0].Slug != "" || guard.charged != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestReviewTwentyMissingChineseTagsUsesTwoPaidCalls(t *testing.T) {
	service, _, provider, guard := testService()
	provider.batchUnique = true
	tags := make([]siteaudit.TagSnapshot, 20)
	for i := range tags {
		tags[i] = siteaudit.TagSnapshot{SuggestedName: fmt.Sprintf("中文%d", i), Role: "TERTIARY", Level: 3}
	}
	result, err := service.ReviewedTags(context.Background(), auth.User{ID: "user", Role: auth.RoleSysAdmin}, "ip", tags)
	if err != nil || provider.calls != 2 || guard.charged != 2 || guard.allowed != 1 {
		t.Fatalf("result=%+v err=%v calls=%d guard=%+v", result, err, provider.calls, guard)
	}
	seen := map[string]bool{}
	for _, tag := range result {
		if tag.Slug == "" || seen[tag.Slug] {
			t.Fatalf("duplicate/empty generated slug: %+v", tag)
		}
		seen[tag.Slug] = true
	}
}

func TestPaidCandidatesPersistBeforeCacheFailure(t *testing.T) {
	service, jobs, provider, cache, guard := jobFixture()
	cache.cacheError = errors.New("cache outage")
	if err := service.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if jobs.job.Status != "paused" || jobs.job.Items[0].State != "ready" || jobs.job.Items[0].Slug != "life" || provider.batchCalls != 1 || guard.charged != 1 {
		t.Fatalf("paid candidates lost: %+v", jobs.job)
	}
	if jobs.saves != 3 {
		t.Fatalf("dispatch/result/pause persistence saves=%d", jobs.saves)
	}
}

func TestConfirmationCanBeResolvedWithManualDistinctSlugWithoutProvider(t *testing.T) {
	// Given a completed preview where both meanings translated to the same slug.
	service, jobs, provider, _, _ := jobFixture()
	if err := service.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if publicJob(jobs.job).Counts.NeedsConfirmation != 2 {
		t.Fatalf("missing confirmation states: %+v", jobs.job)
	}
	// When an administrator confirms distinct meanings by assigning independent slugs.
	result, err := service.Edit(context.Background(), "job", Identity{}, JobEditInput{ExpectedRevision: strconv.FormatInt(jobs.job.Revision, 10), Items: []JobEdit{{TagID: "a", Slug: "life"}, {TagID: "b", Slug: "life-records"}}})
	// Then the items are ready and no additional paid request occurs.
	if err != nil || result.Counts.Ready != 2 || result.Counts.NeedsConfirmation != 0 || provider.batchCalls != 1 {
		t.Fatalf("result=%+v err=%v paid=%d", result, err, provider.batchCalls)
	}
}
