package taskcenter

import (
	"context"

	"heyblog-api/internal/platform/httpapi"

	"github.com/danielgtaylor/huma/v2"
)

func registerConfigurationRoutes(api huma.API, r Routes) {
	httpapi.Register(api, r.operation("bind-verification-request-config", "POST", "tasks/request-configs/{id}/verification-default"), func(ctx context.Context, in *controlInput) (*output[accepted], error) {
		return mutationResponse(ctx, r, in.Key, "config:verification:"+in.ID, in.ID, func(repo *Repository, a Actor) (accepted, error) {
			return accepted{in.ID}, repo.BindVerification(ctx, a, in.ID)
		})
	})
	httpapi.Register(api, r.operation("list-task-schedules", "GET", "tasks/schedules"), func(ctx context.Context, _ *emptyInput) (*output[[]Schedule], error) {
		if _, err := r.authorize(ctx, false); err != nil {
			return nil, err
		}
		return response(r.Repository.Schedules(ctx))
	})
	httpapi.Register(api, r.operation("create-task-schedule", "POST", "tasks/schedules"), func(ctx context.Context, in *bodyInput[ScheduleInput]) (*output[Schedule], error) {
		return mutationResponse(ctx, r, in.Key, "schedule:create", in.Body, func(repo *Repository, a Actor) (Schedule, error) {
			return repo.Save(ctx, a, "", in.Body)
		})
	})
	httpapi.Register(api, r.operation("update-task-schedule", "PUT", "tasks/schedules/{id}"), func(ctx context.Context, in *idBodyInput[ScheduleInput]) (*output[Schedule], error) {
		return mutationResponse(ctx, r, in.Key, "schedule:"+in.ID, in.Body, func(repo *Repository, a Actor) (Schedule, error) {
			return repo.Save(ctx, a, in.ID, in.Body)
		})
	})
	httpapi.Register(api, r.operation("run-task-schedule", "POST", "tasks/schedules/{id}/run"), func(ctx context.Context, in *controlInput) (*output[accepted], error) {
		return mutationResponse(ctx, r, in.Key, "schedule:run:"+in.ID, in.ID, func(repo *Repository, a Actor) (accepted, error) {
			id, err := repo.Manual(ctx, a, in.ID)
			return accepted{id}, err
		})
	})
	httpapi.Register(api, r.operation("list-request-configs", "GET", "tasks/request-configs"), func(ctx context.Context, _ *emptyInput) (*output[[]RequestConfig], error) {
		if _, err := r.authorize(ctx, false); err != nil {
			return nil, err
		}
		return response(r.Repository.Configurations(ctx))
	})
	httpapi.Register(api, r.operation("create-request-config", "POST", "tasks/request-configs"), func(ctx context.Context, in *bodyInput[RequestConfigInput]) (*output[RequestConfig], error) {
		return mutationResponse(ctx, r, in.Key, "config:create", in.Body, func(repo *Repository, a Actor) (RequestConfig, error) {
			return repo.SaveConfiguration(ctx, a, "", in.Body)
		})
	})
	httpapi.Register(api, r.operation("update-request-config", "PUT", "tasks/request-configs/{id}"), func(ctx context.Context, in *idBodyInput[RequestConfigInput]) (*output[RequestConfig], error) {
		return mutationResponse(ctx, r, in.Key, "config:"+in.ID, in.Body, func(repo *Repository, a Actor) (RequestConfig, error) {
			return repo.SaveConfiguration(ctx, a, in.ID, in.Body)
		})
	})
}
