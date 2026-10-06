//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"heyblog-api/internal/features/sluggeneration"
	"heyblog-api/internal/platform/config"
)

func TestDurableSlugJobsPreviewApplyAndRestart(t *testing.T) {
	f := newAuditMigrationFixture(t)
	ctx := t.Context()
	if _, err := f.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	container, err := tcredis.Run(ctx, "redis:8.4-alpine")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Error(err)
		}
	})
	url, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	options, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	var owner, other string
	if err = f.pool.QueryRow(ctx, `INSERT INTO identity.users(email,username,display_name,role)VALUES('slug-admin@example.test','slug_job_admin','Slug Job Admin','SYS_ADMIN') RETURNING id::text`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if err = f.pool.QueryRow(ctx, `INSERT INTO identity.users(email,username,display_name,role)VALUES('slug-other@example.test','slug_job_other','Slug Job Other','SYS_ADMIN') RETURNING id::text`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	var first, second string
	if err = f.pool.QueryRow(ctx, `INSERT INTO directory.tag_dictionary(name,normalized_name,slug)VALUES('Batch Alpha','batch alpha','legacy-batch-alpha') RETURNING id::text`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err = f.pool.QueryRow(ctx, `INSERT INTO directory.tag_dictionary(name,normalized_name,slug)VALUES('Batch Beta','batch beta','legacy-batch-beta') RETURNING id::text`).Scan(&second); err != nil {
		t.Fatal(err)
	}
	configuration := config.AIConfig{Timeout: 15 * time.Second, DefaultModel: "deepseek/deepseek-flash", Limits: config.AILimits{UserPerMinute: 100, IPPerMinute: 100, UserPerDay: 100, GlobalPerDay: 500, Concurrent: 2}, Batch: config.AIBatchConfig{Size: 10, MaxTags: 500, MaxOutputTokens: 4096}}
	generation := sluggeneration.NewService(sluggeneration.Dependencies{Pool: f.pool, Redis: client, Config: configuration})
	repository := sluggeneration.NewJobRepository(f.pool)
	jobs := sluggeneration.NewJobService(generation, repository, configuration.Batch)
	identity := sluggeneration.Identity{UserID: owner, IP: "127.0.0.1"}
	create := func() sluggeneration.Job {
		t.Helper()
		job, err := jobs.Create(ctx, identity, sluggeneration.CreateJobInput{Selection: sluggeneration.JobSelection{Kind: "ids", IDs: []string{first, second}}})
		if err != nil {
			t.Fatal(err)
		}
		return job
	}
	job := create()
	if _, err = jobs.Get(ctx, job.ID, sluggeneration.Identity{UserID: other}); err == nil {
		t.Fatal("another owner read job")
	}
	if _, err = jobs.Get(ctx, job.ID, sluggeneration.Identity{UserID: other, SystemAdmin: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = jobs.Create(ctx, identity, sluggeneration.CreateJobInput{Selection: sluggeneration.JobSelection{Kind: "ids", IDs: []string{first}}}); err == nil {
		t.Fatal("two active jobs per owner allowed")
	}
	if err = jobs.Step(ctx); err != nil {
		t.Fatal(err)
	}
	job, err = jobs.Get(ctx, job.ID, identity)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "ready" || job.Counts.Ready != 2 {
		t.Fatalf("preview=%+v", job)
	}
	if _, err = f.admin.Exec(ctx, `UPDATE directory.slug_generation_jobs SET status='paused',pause_code='slug_rate_limited',resume_after=now()+interval '1 minute' WHERE id=$1::uuid`, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, earlyClaimed, earlyErr := repository.Claim(ctx, "too-early"); earlyErr != nil || earlyClaimed {
		t.Fatalf("rate wait bypassed: %v %v", earlyClaimed, earlyErr)
	}
	if _, err = f.admin.Exec(ctx, `UPDATE directory.slug_generation_jobs SET resume_after=now()-interval '1 second' WHERE id=$1::uuid`, job.ID); err != nil {
		t.Fatal(err)
	}
	waited, autoClaimed, claimErr := repository.Claim(ctx, "rate-recovered")
	if claimErr != nil || !autoClaimed || waited.PauseCode != "" {
		t.Fatalf("rate wait did not resume: %+v %v", waited, claimErr)
	}
	waited.Status = "ready"
	if saved, err := repository.SaveClaim(ctx, waited); err != nil || !saved {
		t.Fatalf("finish resumed task: %v %v", saved, err)
	}
	job, err = jobs.Get(ctx, job.ID, identity)
	if err != nil {
		t.Fatal(err)
	}
	var original string
	if err = f.pool.QueryRow(ctx, `SELECT slug FROM directory.tags WHERE id=$1::uuid`, first).Scan(&original); err != nil || original != "legacy-batch-alpha" {
		t.Fatalf("generation changed dictionary: %s %v", original, err)
	}
	for _, item := range job.Items {
		if item.TagID == second {
			_, err = f.pool.Exec(ctx, `INSERT INTO directory.tag_slug_aliases(slug,tag_id)VALUES($1,$2::uuid)`, item.Slug, first)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err = jobs.Apply(ctx, job.ID, identity, sluggeneration.JobApplyInput{TagIDs: []string{first, second}, ExpectedRevision: job.Revision}); err == nil {
		t.Fatal("colliding atomic apply succeeded")
	}
	if err = f.pool.QueryRow(ctx, `SELECT slug FROM directory.tags WHERE id=$1::uuid`, first).Scan(&original); err != nil || original != "legacy-batch-alpha" {
		t.Fatal("partial apply leaked despite conflict")
	}
	if _, err = f.pool.Exec(ctx, `DELETE FROM directory.tag_slug_aliases WHERE tag_id=$1::uuid`, first); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, `UPDATE directory.tags SET description='changed after preview' WHERE id=$1::uuid`, second); err != nil {
		t.Fatal(err)
	}
	if _, err = jobs.Apply(ctx, job.ID, identity, sluggeneration.JobApplyInput{TagIDs: []string{first, second}, ExpectedRevision: job.Revision}); err == nil {
		t.Fatal("stale snapshot applied")
	}
	job, err = jobs.Apply(ctx, job.ID, identity, sluggeneration.JobApplyInput{TagIDs: []string{first}, ExpectedRevision: job.Revision})
	if err != nil || job.Counts.Applied != 1 {
		t.Fatalf("apply=%+v %v", job, err)
	}
	var resolved string
	if err = f.pool.QueryRow(ctx, `SELECT directory.canonical_tag_slug('legacy-batch-alpha')`).Scan(&resolved); err != nil || resolved != "batch-alpha" {
		t.Fatalf("old slug=%s %v", resolved, err)
	}
	if _, err = jobs.Apply(ctx, job.ID, identity, sluggeneration.JobApplyInput{TagIDs: []string{second}, ExpectedRevision: "1"}); err == nil {
		t.Fatal("stale task revision applied")
	}
	if _, err = jobs.Control(ctx, job.ID, identity, sluggeneration.JobControlInput{Action: "cancel", ExpectedRevision: job.Revision}); err != nil {
		t.Fatal(err)
	}
	job = create()
	record, claimed, err := repository.Claim(ctx, "crashed-worker")
	if err != nil || !claimed || record.ID != job.ID {
		t.Fatalf("claim=%s %v %v", record.ID, claimed, err)
	}
	record.Items[0].State = "running"
	if ok, err := repository.SaveClaim(ctx, record); err != nil || !ok {
		t.Fatalf("dispatch persistence=%v %v", ok, err)
	}
	if _, err = f.admin.Exec(ctx, `UPDATE directory.slug_generation_jobs SET lease_until=now()-interval '1 second' WHERE id=$1::uuid`, job.ID); err != nil {
		t.Fatal(err)
	}
	recovered, claimed, err := repository.Claim(ctx, "new-worker")
	if err != nil || !claimed || recovered.Items[0].State != "failed" || recovered.Items[0].ErrorCode != "slug_generation_interrupted" {
		t.Fatalf("recovery=%+v %v", recovered, err)
	}
	record.Status = "ready"
	if saved, err := repository.SaveClaim(ctx, record); err != nil || saved {
		t.Fatalf("stale worker overwrote newer claim: %v %v", saved, err)
	}
	recovered.Status = "ready"
	if saved, err := repository.SaveClaim(ctx, recovered); err != nil || !saved {
		t.Fatalf("finish recovery: %v %v", saved, err)
	}
	if _, err = jobs.Control(ctx, job.ID, identity, sluggeneration.JobControlInput{Action: "cancel"}); err != nil {
		t.Fatal(err)
	}
	// Given different confirmed concepts whose local names normalize to one candidate.
	var gamma, gammaVariant string
	if err = f.pool.QueryRow(ctx, `INSERT INTO directory.tag_dictionary(name,normalized_name,slug) VALUES('Batch Gamma','batch gamma','gamma-original') RETURNING id::text`).Scan(&gamma); err != nil {
		t.Fatal(err)
	}
	if err = f.pool.QueryRow(ctx, `INSERT INTO directory.tag_dictionary(name,normalized_name,slug) VALUES('Batch-Gamma','batch-gamma','gamma-variant-original') RETURNING id::text`).Scan(&gammaVariant); err != nil {
		t.Fatal(err)
	}
	conflictJob, err := jobs.Create(ctx, identity, sluggeneration.CreateJobInput{Selection: sluggeneration.JobSelection{Kind: "ids", IDs: []string{gamma, gammaVariant}}})
	if err != nil {
		t.Fatal(err)
	}
	// When their preview is generated and persisted by the worker.
	if err = jobs.Step(ctx); err != nil {
		t.Fatal(err)
	}
	conflictJob, err = jobs.Get(ctx, conflictJob.ID, identity)
	if err != nil {
		t.Fatal(err)
	}
	// Then both candidates need confirmation and an unresolved item cannot apply.
	if conflictJob.Counts.NeedsConfirmation != 2 || conflictJob.Counts.Ready != 0 {
		t.Fatalf("conflict preview=%+v", conflictJob)
	}
	for _, item := range conflictJob.Items {
		if item.Slug != "batch-gamma" || len(item.Conflicts) == 0 {
			t.Fatalf("missing conflict=%+v", item)
		}
	}
	if _, err = jobs.Apply(ctx, conflictJob.ID, identity, sluggeneration.JobApplyInput{ExpectedRevision: conflictJob.Revision, TagIDs: []string{gamma}}); err == nil {
		t.Fatal("unresolved candidate applied")
	}
	// An explicit independent spelling resolves the ambiguity without provider configuration.
	conflictJob, err = jobs.Edit(ctx, conflictJob.ID, identity, sluggeneration.JobEditInput{ExpectedRevision: conflictJob.Revision, Items: []sluggeneration.JobEdit{{TagID: gamma, Slug: "batch-gamma"}, {TagID: gammaVariant, Slug: "batch-gamma-variant"}}})
	if err != nil || conflictJob.Counts.Ready != 2 || conflictJob.Counts.NeedsConfirmation != 0 {
		t.Fatalf("manual resolution=%+v %v", conflictJob, err)
	}
	conflictJob, err = jobs.Apply(ctx, conflictJob.ID, identity, sluggeneration.JobApplyInput{ExpectedRevision: conflictJob.Revision, TagIDs: []string{gamma, gammaVariant}})
	if err != nil || conflictJob.Status != "completed" {
		t.Fatalf("manual apply=%+v %v", conflictJob, err)
	}
	job = create()
	if _, err = f.pool.Exec(ctx, `UPDATE identity.users SET role='USER' WHERE id=$1::uuid`, owner); err != nil {
		t.Fatal(err)
	}
	if err = jobs.Step(ctx); err != nil {
		t.Fatal(err)
	}
	job, err = jobs.Get(ctx, job.ID, identity)
	if err != nil || job.Status != "paused" || job.PauseCode != "slug_job_permission_revoked" {
		t.Fatalf("revoked=%+v %v", job, err)
	}
	var internal []byte
	if err = f.pool.QueryRow(ctx, `SELECT items FROM directory.slug_generation_jobs WHERE id=$1::uuid`, job.ID).Scan(&internal); err != nil {
		t.Fatal(err)
	}
	var saved []map[string]json.RawMessage
	if err = json.Unmarshal(internal, &saved); err != nil {
		t.Fatal(err)
	}
	if _, exists := saved[0]["updated_at"]; !exists {
		t.Fatal("durable tag snapshot missing")
	}
}

func TestSlugJobWorkerStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := sluggeneration.NewJobService(nil, nil, config.AIBatchConfig{})
	done := make(chan struct{})
	go func() { defer close(done); service.Run(ctx, nil) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker remained after cancellation")
	}
}
