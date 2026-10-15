package databasebackup

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/httpapi"
)

type Operations interface {
	Export(context.Context) (*os.File, error)
	Inspect(context.Context, *os.File, string) (Inspection, error)
	Restorer
}

type Authenticator interface {
	Current(context.Context, *http.Request) (auth.User, error)
}
type actorKey struct{}
type emptyInput struct{}
type output[T any] struct {
	CacheControl string `header:"Cache-Control"`
	Body         T
}
type exportOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	CacheControl       string `header:"Cache-Control"`
	Body               func(huma.Context)
}

func BodyLimitOverrides() map[httpapi.Route]int64 {
	limits := map[httpapi.Route]int64{}
	for _, action := range []string{"export", "inspect", "restore"} {
		limits[httpapi.Route{Method: http.MethodPost, Path: Path + "/" + action}] = BodyLimit
	}
	return limits
}

func RegisterRoutes(api huma.API, service Operations, jobs *Jobs, authenticator Authenticator, webToken string, logger *slog.Logger) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	jobs.logger = logger
	receive := func(ctx context.Context, action string) (uploadedBackup, error) {
		phaseLogger := logger.With("request_id", httpapi.RequestID(httpapi.NativeContext(ctx)), "action", action)
		started := time.Now()
		phaseLogger.InfoContext(ctx, "database backup upload started", "event", "database_backup_upload_started")
		uploaded, err := upload(httpapi.Request(ctx))
		if err == nil {
			phaseLogger.InfoContext(ctx, "database backup upload completed", "event", "database_backup_upload_completed", "received_bytes", uploaded.bytes, "duration_ms", time.Since(started).Milliseconds())
		}
		return uploaded, err
	}
	cleanup := func(file *os.File) {
		if err := errorsWithCleanup(nil, file); err != nil {
			logger.Warn("database backup temporary file cleanup failed", "event", "database_backup_cleanup_failed")
		}
	}
	operation := func(action string) huma.Operation {
		op := huma.Operation{
			OperationID: "database-backup-" + action, Method: http.MethodPost, Path: Path + "/" + action, Summary: "Database backup " + action,
			Tags: []string{"Database backup"}, MaxBodyBytes: BodyLimit, BodyReadTimeout: Timeout,
			Errors: []int{400, 401, 403, 409, 413, 422, 503}, Security: []map[string][]string{{"webToken": {}, "accessCookie": {}}},
			Middlewares: huma.Middlewares{httpapi.HumaWebAuthorization(webToken), authorize(authenticator)},
		}
		return op
	}
	httpapi.Register(api, operation("export"), func(ctx context.Context, _ *emptyInput) (*exportOutput, error) {
		work, cancel := context.WithTimeout(ctx, Timeout)
		defer cancel()
		file, err := service.Export(work)
		if err != nil {
			return nil, httpError(err)
		}
		return &exportOutput{ContentType: "application/json", ContentDisposition: `attachment; filename="heyblog-database-backup.json"`, CacheControl: "private, no-store", Body: func(response huma.Context) {
			defer cleanup(file)
			if _, err := io.Copy(response.BodyWriter(), file); err != nil {
				logger.Warn("database backup download interrupted", "event", "database_backup_download_interrupted")
			}
		}}, nil
	})
	httpapi.Register(api, operation("inspect"), func(ctx context.Context, _ *emptyInput) (*output[Inspection], error) {
		if err := jobs.reserve(); err != nil {
			return nil, httpError(err)
		}
		defer jobs.release()
		uploaded, err := receive(ctx, "inspect")
		if err != nil {
			return nil, httpError(err)
		}
		defer cleanup(uploaded.file)
		actor, _ := ctx.Value(actorKey{}).(string)
		work, cancel := context.WithTimeout(ctx, Timeout)
		defer cancel()
		phaseLogger := logger.With("request_id", httpapi.RequestID(httpapi.NativeContext(ctx)), "action", "inspect")
		started := time.Now()
		phaseLogger.InfoContext(ctx, "database backup inspection started", "event", "database_backup_inspection_started", "received_bytes", uploaded.bytes)
		preview, err := service.Inspect(work, uploaded.file, actor)
		if err != nil {
			return nil, httpError(&transferError{cause: err, stage: "inspect", bytes: uploaded.bytes})
		}
		phaseLogger.InfoContext(ctx, "database backup inspection completed", "event", "database_backup_inspection_completed", "received_bytes", uploaded.bytes, "duration_ms", time.Since(started).Milliseconds())
		return &output[Inspection]{CacheControl: "private, no-store", Body: preview}, nil
	})
	restoreOperation := operation("restore")
	restoreOperation.DefaultStatus = http.StatusAccepted
	httpapi.Register(api, restoreOperation, func(ctx context.Context, _ *emptyInput) (*output[RestoreJob], error) {
		if err := jobs.reserve(); err != nil {
			return nil, httpError(err)
		}
		accepted := false
		defer func() {
			if !accepted {
				jobs.release()
			}
		}()
		uploaded, err := receive(ctx, "restore")
		if err != nil {
			return nil, httpError(err)
		}
		defer func() {
			if !accepted {
				cleanup(uploaded.file)
			}
		}()
		if uploaded.fields["confirmation"] != "RESTORE" || len(uploaded.fields["sha256"]) != 64 {
			return nil, httpError(ErrInvalid)
		}
		actor, _ := ctx.Value(actorKey{}).(string)
		job, err := jobs.start(ctx, uploaded.file, actor, uploaded.fields["sha256"])
		if err != nil {
			return nil, httpError(err)
		}
		accepted = true
		return &output[RestoreJob]{CacheControl: "private, no-store", Body: job}, nil
	})
	registerJobRoutes(api, jobs, authenticator, webToken)
	// Publish the multipart contract after registration: parsing remains streaming
	// in upload, while Huma must not buffer the entire backup as a JSON body.
	for _, action := range []string{"inspect", "restore"} {
		op := api.OpenAPI().Paths[Path+"/"+action].Post

		fields := map[string]*huma.Schema{"file": {Type: "string", Format: "binary"}}
		required := []string{"file"}
		if action == "restore" {
			fields["sha256"] = &huma.Schema{Type: "string"}
			fields["confirmation"] = &huma.Schema{Type: "string", Enum: []any{"RESTORE"}}
			required = append(required, "sha256", "confirmation")
		}
		op.RequestBody = &huma.RequestBody{Required: true, Content: map[string]*huma.MediaType{"multipart/form-data": {Schema: &huma.Schema{Type: "object", Properties: fields, Required: required}}}}
	}

}

func authorize(service Authenticator) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		actor, err := service.Current(ctx.Context(), httpapi.Request(ctx.Context()))
		if err != nil {
			var authError *auth.AuthError
			if errors.As(err, &authError) && authError.StatusCode == http.StatusUnauthorized {
				httpapi.RejectHumaRequest(ctx, apperror.New(apperror.KindUnauthorized, "database_backup_auth_required", "sign in to continue"))
			} else if errors.As(err, &authError) && authError.StatusCode == http.StatusForbidden {
				httpapi.RejectHumaRequest(ctx, apperror.New(apperror.KindForbidden, authError.Code, authError.Message))
			} else {
				httpapi.RejectHumaRequest(ctx, httpError(err))
			}
			return
		}
		if actor.Role != auth.RoleSysAdmin || !actor.Active {
			httpapi.RejectHumaRequest(ctx, apperror.New(apperror.KindForbidden, "database_backup_forbidden", "system administrator access is required"))
			return
		}
		if err := httpapi.SetDeadlines(ctx.Context(), time.Now().Add(Timeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			httpapi.RejectHumaRequest(ctx, httpError(err))
			return
		}
		next(huma.WithValue(ctx, actorKey{}, actor.ID))
	}
}
