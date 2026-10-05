package sluggeneration

import (
	"context"
	"net/http"

	"heyblog-api/internal/platform/httpapi"

	"github.com/danielgtaylor/huma/v2"
)

type jobPathInput struct {
	ID string `path:"id"`
}
type jobBodyInput[T any] struct {
	ID   string `path:"id"`
	Body T
}
type jobList struct {
	Jobs []Job `json:"jobs"`
}

func (service *Service) registerJobRoutes(api huma.API, webToken string) error {
	operation := func(id, method, path string) huma.Operation {
		return huma.Operation{OperationID: id, Method: method, Path: "/management/taxonomy/slug-jobs" + path, Tags: []string{"taxonomy"}, MaxBodyBytes: 131072,
			Security: []map[string][]string{{"webToken": {}, "accessCookie": {}}}, Errors: []int{401, 403, 404, 409, 413, 422, 429, 503},
			Middlewares: huma.Middlewares{httpapi.HumaWebAuthorization(webToken), service.authorization(false)}}
	}
	create := operation("create-slug-job", http.MethodPost, "")
	create.DefaultStatus = http.StatusAccepted
	httpapi.Register(api, create, func(ctx context.Context, input *struct{ Body CreateJobInput }) (*response[Job], error) {
		if service.jobs == nil {
			return nil, unavailable()
		}
		identity, _ := ctx.Value(identityKey{}).(Identity)
		job, err := service.jobs.Create(ctx, identity, input.Body)
		if err != nil {
			setRetryHeader(ctx, err)
			return nil, err
		}
		return &response[Job]{CacheControl: "private, no-store", Body: job}, nil
	})
	httpapi.Register(api, operation("list-slug-jobs", http.MethodGet, ""), func(ctx context.Context, _ *struct{}) (*response[jobList], error) {
		if service.jobs == nil {
			return nil, unavailable()
		}
		identity, _ := ctx.Value(identityKey{}).(Identity)
		jobs, err := service.jobs.List(ctx, identity)
		if err != nil {
			return nil, err
		}
		return &response[jobList]{CacheControl: "private, no-store", Body: jobList{Jobs: jobs}}, nil
	})
	httpapi.Register(api, operation("get-slug-job", http.MethodGet, "/{id}"), func(ctx context.Context, input *jobPathInput) (*response[Job], error) {
		if service.jobs == nil {
			return nil, unavailable()
		}
		identity, _ := ctx.Value(identityKey{}).(Identity)
		job, err := service.jobs.Get(ctx, input.ID, identity)
		if err != nil {
			return nil, err
		}
		return &response[Job]{CacheControl: "private, no-store", Body: job}, nil
	})
	httpapi.Register(api, operation("control-slug-job", http.MethodPost, "/{id}/control"), func(ctx context.Context, input *jobBodyInput[JobControlInput]) (*response[Job], error) {
		if service.jobs == nil {
			return nil, unavailable()
		}
		identity, _ := ctx.Value(identityKey{}).(Identity)
		job, err := service.jobs.Control(ctx, input.ID, identity, input.Body)
		if err != nil {
			setRetryHeader(ctx, err)
			return nil, err
		}
		return &response[Job]{CacheControl: "private, no-store", Body: job}, nil
	})
	httpapi.Register(api, operation("edit-slug-job", http.MethodPatch, "/{id}/items"), func(ctx context.Context, input *jobBodyInput[JobEditInput]) (*response[Job], error) {
		if service.jobs == nil {
			return nil, unavailable()
		}
		identity, _ := ctx.Value(identityKey{}).(Identity)
		job, err := service.jobs.Edit(ctx, input.ID, identity, input.Body)
		if err != nil {
			setRetryHeader(ctx, err)
			return nil, err
		}
		return &response[Job]{CacheControl: "private, no-store", Body: job}, nil
	})
	httpapi.Register(api, operation("apply-slug-job", http.MethodPost, "/{id}/apply"), func(ctx context.Context, input *jobBodyInput[JobApplyInput]) (*response[Job], error) {
		if service.jobs == nil {
			return nil, unavailable()
		}
		identity, _ := ctx.Value(identityKey{}).(Identity)
		job, err := service.jobs.Apply(ctx, input.ID, identity, input.Body)
		if err != nil {
			setRetryHeader(ctx, err)
			return nil, err
		}
		return &response[Job]{CacheControl: "private, no-store", Body: job}, nil
	})
	return nil
}
