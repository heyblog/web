package taskcenter

import (
	"context"

	"heyblog-api/internal/platform/httpapi"

	"heyblog.local/taskprotocol"

	"github.com/danielgtaylor/huma/v2"
)

func registerObservabilityRoutes(api huma.API, r Routes) {
	httpapi.Register(api, r.operation("task-overview", "GET", "tasks/overview"), func(ctx context.Context, _ *emptyInput) (*output[Overview], error) {
		if _, err := r.authorize(ctx, false); err != nil {
			return nil, err
		}
		return response(r.Repository.Overview(ctx))
	})
	httpapi.Register(api, r.operation("task-statistics", "GET", "tasks/stats"), func(ctx context.Context, in *Query) (*output[[]Statistics], error) {
		if _, err := r.authorize(ctx, false); err != nil {
			return nil, err
		}
		return response(r.Repository.Stats(ctx, *in))
	})
	httpapi.Register(api, r.operation("list-workers", "GET", "tasks/workers"), func(ctx context.Context, _ *emptyInput) (*output[[]taskprotocol.Worker], error) {
		if _, err := r.authorize(ctx, false); err != nil {
			return nil, err
		}
		return response(r.Repository.Workers(ctx))
	})
	for _, action := range []string{"drain", "resume"} {
		httpapi.Register(api, r.operation(action+"-worker", "POST", "tasks/workers/{id}/"+action), func(ctx context.Context, in *controlInput) (*output[accepted], error) {
			if _, err := r.authorize(ctx, true); err != nil {
				return nil, err
			}
			return mutationResponse(ctx, r, in.Key, "worker:"+action+":"+in.ID, in.ID, func(repo *Repository, a Actor) (accepted, error) {
				return accepted{in.ID}, repo.Drain(ctx, a, in.ID, action == "drain")
			})
		})
	}
}
