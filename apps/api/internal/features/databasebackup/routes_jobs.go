package databasebackup

import (
	"context"
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

func registerJobRoutes(api huma.API, jobs *Jobs, authenticator Authenticator, webToken string) {
	operation := func(method, suffix, id string) huma.Operation {
		return huma.Operation{OperationID: "database-backup-" + id, Method: method, Path: Path + "/restore-jobs/" + suffix,
			Tags: []string{"Database backup"}, Errors: []int{401, 403, 404, 422, 503},
			Security:    []map[string][]string{{"webToken": {}, "accessCookie": {}}},
			Middlewares: huma.Middlewares{httpapi.HumaWebAuthorization(webToken), authorize(authenticator)}}
	}
	httpapi.Register(api, operation(http.MethodGet, "current", "current-restore-job"), func(_ context.Context, _ *emptyInput) (*output[currentJob], error) {
		return &output[currentJob]{CacheControl: "private, no-store", Body: currentJob{Job: jobs.Current()}}, nil
	})
	httpapi.Register(api, operation(http.MethodGet, "{id}", "get-restore-job"), func(_ context.Context, input *jobInput) (*output[RestoreJob], error) {
		job, err := jobs.Get(input.ID)
		if err != nil {
			return nil, httpError(err)
		}
		return &output[RestoreJob]{CacheControl: "private, no-store", Body: job}, nil
	})
	httpapi.Register(api, operation(http.MethodPost, "{id}/cancel", "cancel-restore-job"), func(_ context.Context, input *jobInput) (*output[RestoreJob], error) {
		job, err := jobs.Cancel(input.ID)
		if err != nil {
			return nil, httpError(err)
		}
		return &output[RestoreJob]{CacheControl: "private, no-store", Body: job}, nil
	})
}
