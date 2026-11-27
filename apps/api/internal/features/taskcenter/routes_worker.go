package taskcenter

import (
	"context"

	"heyblog-api/internal/platform/httpapi"

	"heyblog.local/taskprotocol"

	"github.com/danielgtaylor/huma/v2"
)

type workerBody[T any] struct{ Body T }
type workerTarget[T any] struct {
	IDInput
	Body T
}

func registerWorkerRoutes(api huma.API, r Routes) {
	if r.WorkerToken == "" {
		return
	}
	operation := func(id, path string) huma.Operation {
		return huma.Operation{OperationID: id, Method: "POST", Path: "/api/internal/v1/tasks/" + path,
			MaxBodyBytes: 2 << 20, Security: []map[string][]string{{"apiBearer": {}}},
			Middlewares: huma.Middlewares{httpapi.HumaBearerAuthorization(r.WorkerToken, "task-worker")}, Errors: []int{401, 409, 422, 500}}
	}
	httpapi.Register(api, operation("register-worker", "register"), func(ctx context.Context, in *workerBody[taskprotocol.Instance]) (*output[taskprotocol.Worker], error) {
		return response(r.Repository.Register(ctx, in.Body))
	})
	httpapi.Register(api, operation("heartbeat-worker", "heartbeat"), func(ctx context.Context, in *workerBody[taskprotocol.WorkerInput]) (*output[taskprotocol.Worker], error) {
		return response(r.Repository.WorkerHeartbeat(ctx, in.Body))
	})
	httpapi.Register(api, operation("due-task-schedules", "due"), func(ctx context.Context, in *workerBody[taskprotocol.WorkerInput]) (*output[taskprotocol.Due], error) {
		return response(r.Repository.Due(ctx, in.Body))
	})
	httpapi.Register(api, operation("dispatch-task-schedule", "dispatch"), func(ctx context.Context, in *workerBody[taskprotocol.Dispatch]) (*output[accepted], error) {
		return response(r.Repository.Dispatch(ctx, in.Body))
	})
	httpapi.Register(api, operation("claim-task-target", "claim"), func(ctx context.Context, in *workerBody[taskprotocol.WorkerInput]) (*output[*taskprotocol.Work], error) {
		return response(r.Repository.Claim(ctx, in.Body))
	})
	httpapi.Register(api, operation("heartbeat-task-target", "targets/{id}/heartbeat"), func(ctx context.Context, in *workerTarget[taskprotocol.Fence]) (*output[string], error) {
		return response(r.Repository.Heartbeat(ctx, in.ID, in.Body))
	})
	httpapi.Register(api, operation("authorize-task-request", "targets/{id}/authorize"), func(ctx context.Context, in *workerTarget[taskprotocol.Authorization]) (*output[taskprotocol.Permit], error) {
		return response(r.Signing.Permit(ctx, in.ID, in.Body))
	})
	httpapi.Register(api, operation("send-task-request", "targets/{id}/sent"), func(ctx context.Context, in *workerTarget[Send]) (*output[accepted], error) {
		return response(accepted{in.Body.AttemptID}, r.Repository.Sent(ctx, in.ID, in.Body))
	})
	httpapi.Register(api, operation("report-task-request", "targets/{id}/report"), func(ctx context.Context, in *workerTarget[taskprotocol.Report]) (*output[taskprotocol.Evaluation], error) {
		return response(r.Execution.Report(ctx, in.ID, in.Body))
	})
	httpapi.Register(api, operation("finish-task-target", "targets/{id}/finish"), func(ctx context.Context, in *workerTarget[taskprotocol.Finish]) (*output[ExecutionResult], error) {
		return response(r.Execution.Finish(ctx, in.ID, in.Body))
	})
}
