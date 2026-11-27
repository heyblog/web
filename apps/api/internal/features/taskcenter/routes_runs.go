package taskcenter

import (
	"context"
	"net/http"

	"heyblog-api/internal/platform/httpapi"

	"github.com/danielgtaylor/huma/v2"
)

type runQuery struct {
	IDInput
	Query
}
type retryInput struct {
	Selection string `json:"selection" enum:"failed,unfinished,all" default:"failed"`
}

func registerRunRoutes(api huma.API, r Routes) {
	httpapi.Register(api, r.operation("list-task-runs", "GET", "tasks/runs"), func(ctx context.Context, in *Query) (*output[Page[Run]], error) {
		if _, err := r.authorize(ctx, false); err != nil {
			return nil, err
		}
		return response(r.Repository.Runs(ctx, *in))
	})
	httpapi.Register(api, r.operation("get-task-run", "GET", "tasks/runs/{id}"), func(ctx context.Context, in *IDInput) (*output[Run], error) {
		if _, err := r.authorize(ctx, false); err != nil {
			return nil, err
		}
		return response(r.Repository.run(ctx, in.ID))
	})
	httpapi.Register(api, r.operation("create-manual-task", "POST", "tasks/runs"), func(ctx context.Context, in *bodyInput[ManualInput]) (*output[accepted], error) {
		return mutationResponse(ctx, r, in.Key, "run:create", in.Body, func(repo *Repository, a Actor) (accepted, error) {
			id, err := repo.Create(ctx, a, in.Body)
			return accepted{id}, err
		})
	})
	httpapi.Register(api, r.operation("list-task-targets", "GET", "tasks/runs/{id}/targets"), func(ctx context.Context, in *runQuery) (*output[Page[Target]], error) {
		if _, err := r.authorize(ctx, false); err != nil {
			return nil, err
		}
		return response(r.Repository.Targets(ctx, in.ID, in.Query))
	})
	for _, action := range []string{"pause", "resume", "cancel", "archive", "unarchive"} {
		httpapi.Register(api, r.operation(action+"-task-run", "POST", "tasks/runs/{id}/"+action), func(ctx context.Context, in *controlInput) (*output[Run], error) {
			result, err := mutationResponse(ctx, r, in.Key, action+":"+in.ID, in.ID, func(repo *Repository, a Actor) (Run, error) {
				return repo.Control(ctx, a, in.ID, action)
			})
			if err == nil && (action == "pause" || action == "resume" || action == "cancel") {
				result.Status = http.StatusAccepted
			}
			return result, err
		})
	}
	for _, action := range []string{"retry", "rerun"} {
		httpapi.Register(api, r.operation(action+"-task-run", "POST", "tasks/runs/{id}/"+action), func(ctx context.Context, in *idBodyInput[retryInput]) (*output[accepted], error) {
			return mutationResponse(ctx, r, in.Key, action+":"+in.ID, in.Body, func(repo *Repository, a Actor) (accepted, error) {
				selection := in.Body.Selection
				if action == "rerun" {
					selection = "all"
				}
				id, err := repo.Retry(ctx, a, in.ID, selection)
				return accepted{id}, err
			})
		})
	}
}
