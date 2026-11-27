package taskcenter

import (
	"context"
	"errors"

	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/httpapi"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type Batch struct {
	IDs    []string `json:"ids" minItems:"1" maxItems:"100" uniqueItems:"true"`
	Action string   `json:"action" enum:"pause,resume,cancel,archive,unarchive"`
}
type BatchItem struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Code  string `json:"code"`
}
type cleanupInput struct {
	Confirmation string `json:"confirmation" format:"uuid"`
}
type exportInput struct {
	IDInput
	Query
	Format string `query:"format" enum:"csv,json" default:"json"`
}
type exportOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	CacheControl       string `header:"Cache-Control"`
	Body               []byte
}

func registerHistoryRoutes(api huma.API, r Routes) {
	httpapi.Register(api, r.operation("list-task-attempts", "GET", "tasks/runs/{id}/attempts"), func(ctx context.Context, in *runQuery) (*output[Page[Attempt]], error) {
		if _, err := r.authorize(ctx, false); err != nil {
			return nil, err
		}
		return response(r.Repository.Attempts(ctx, in.ID, in.Query))
	})
	httpapi.Register(api, r.operation("list-task-events", "GET", "tasks/runs/{id}/events"), func(ctx context.Context, in *runQuery) (*output[Page[TaskEvent]], error) {
		if _, err := r.authorize(ctx, false); err != nil {
			return nil, err
		}
		return response(r.Repository.Events(ctx, in.ID, in.Query))
	})
	httpapi.Register(api, r.operation("batch-task-control", "POST", "tasks/runs/batch"), func(ctx context.Context, in *bodyInput[Batch]) (*output[[]BatchItem], error) {
		return mutationResponse(ctx, r, in.Key, "runs:batch", in.Body, func(repo *Repository, a Actor) ([]BatchItem, error) {
			if len(in.Body.IDs) == 0 || len(in.Body.IDs) > 100 {
				return nil, invalid(errors.New("invalid batch size"))
			}
			items := []BatchItem{}
			for _, id := range in.Body.IDs {
				if _, err := uuid.Parse(id); err != nil {
					items = append(items, BatchItem{ID: id, Code: "invalid_id"})
					continue
				}
				item, err := repo.Control(ctx, a, id, in.Body.Action)
				if err != nil {
					var failure *apperror.Error
					if !errors.As(err, &failure) {
						return nil, err
					}
					items = append(items, BatchItem{ID: id, State: failure.CurrentState(), Code: failure.Code()})
				} else {
					items = append(items, BatchItem{ID: id, State: string(item.State), Code: "accepted"})
				}
			}
			return items, nil
		})
	})
	httpapi.Register(api, r.operation("cleanup-task-run", "POST", "tasks/runs/{id}/cleanup"), func(ctx context.Context, in *idBodyInput[cleanupInput]) (*output[Run], error) {
		if _, err := r.authorize(ctx, true); err != nil {
			return nil, err
		}
		return mutationResponse(ctx, r, in.Key, "cleanup:"+in.ID, in.Body, func(repo *Repository, a Actor) (Run, error) {
			return repo.Cleanup(ctx, a, in.ID, in.Body.Confirmation)
		})
	})
	httpapi.Register(api, r.operation("export-task-run", "GET", "tasks/runs/{id}/export"), func(ctx context.Context, in *exportInput) (*exportOutput, error) {
		if _, err := r.authorize(ctx, false); err != nil {
			return nil, err
		}
		content, err := r.Repository.Export(ctx, in.ID, in.Format, in.Query)
		if err != nil {
			return nil, err
		}
		kind := "application/json"
		if in.Format == "csv" {
			kind = "text/csv; charset=utf-8"
		}
		return &exportOutput{ContentType: kind, ContentDisposition: `attachment; filename="task-` + in.ID + `.` + in.Format + `"`,
			CacheControl: "private, no-store", Body: content}, nil
	})
}
