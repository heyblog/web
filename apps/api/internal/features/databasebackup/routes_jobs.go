package databasebackup

import (
	"context"
	"errors"
	"net/http"

	"heyblog-api/internal/platform/httpapi"

	"github.com/danielgtaylor/huma/v2"
)

type jobInput struct {
	ID string `path:"id" format:"uuid"`
}
type currentJob struct {
	Job *RestoreJob `json:"job"`
}

func registerJobRoutes(api huma.API, jobs *Jobs, authenticator Authenticator, webToken string, uploads *Uploads) {
	operation := func(method, suffix, id string) huma.Operation {
		return huma.Operation{OperationID: "database-backup-" + id, Method: method, Path: Path + "/restore-jobs/" + suffix,
			Tags: []string{"Database backup"}, Errors: []int{401, 403, 404, 422, 503},
			Security:    []map[string][]string{{"webToken": {}, "accessCookie": {}}},
			Middlewares: huma.Middlewares{httpapi.HumaWebAuthorization(webToken), authorize(authenticator)}}
	}
	httpapi.Register(api, operation(http.MethodGet, "current", "current-restore-job"), func(ctx context.Context, _ *emptyInput) (*output[currentJob], error) {
		actor, _ := ctx.Value(actorKey{}).(string)
		job := jobs.Current()
		if job == nil {
			job, _ = uploads.StoredJob(actor, "")
		} else if job.UploadID != "" {
			if _, err := uploads.StoredJob(actor, job.ID); err != nil {
				job = nil
			}
		}
		return &output[currentJob]{CacheControl: "private, no-store", Body: currentJob{Job: job}}, nil
	})
	httpapi.Register(api, operation(http.MethodGet, "{id}", "get-restore-job"), func(ctx context.Context, input *jobInput) (*output[RestoreJob], error) {
		actor, _ := ctx.Value(actorKey{}).(string)
		job, err := jobs.Get(input.ID)
		if errors.Is(err, ErrJobNotFound) {
			if stored, storedErr := uploads.StoredJob(actor, input.ID); storedErr == nil {
				job, err = *stored, nil
			}
		}
		if err == nil && job.UploadID != "" {
			_, err = uploads.StoredJob(actor, input.ID)
		}
		if err != nil {
			return nil, httpError(err)
		}
		return &output[RestoreJob]{CacheControl: "private, no-store", Body: job}, nil
	})
	httpapi.Register(api, operation(http.MethodPost, "{id}/cancel", "cancel-restore-job"), func(ctx context.Context, input *jobInput) (*output[RestoreJob], error) {
		actor, _ := ctx.Value(actorKey{}).(string)
		if job, err := jobs.Get(input.ID); err == nil && job.UploadID != "" {
			if _, err := uploads.StoredJob(actor, input.ID); err != nil {
				return nil, httpError(err)
			}
		}
		job, err := jobs.Cancel(input.ID)
		if err != nil {
			return nil, httpError(err)
		}
		return &output[RestoreJob]{CacheControl: "private, no-store", Body: job}, nil
	})
}
