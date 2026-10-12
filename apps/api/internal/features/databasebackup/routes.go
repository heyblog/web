package databasebackup

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgconn"

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
		file, _, err := upload(httpapi.Request(ctx))
		if err != nil {
			return nil, httpError(err)
		}
		defer cleanup(file)
		actor, _ := ctx.Value(actorKey{}).(string)
		work, cancel := context.WithTimeout(ctx, Timeout)
		defer cancel()
		preview, err := service.Inspect(work, file, actor)
		if err != nil {
			return nil, httpError(err)
		}
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
		file, fields, err := upload(httpapi.Request(ctx))
		if err != nil {
			return nil, httpError(err)
		}
		defer func() {
			if !accepted {
				cleanup(file)
			}
		}()
		if fields["confirmation"] != "RESTORE" || len(fields["sha256"]) != 64 {
			return nil, httpError(ErrInvalid)
		}
		actor, _ := ctx.Value(actorKey{}).(string)
		job, err := jobs.start(ctx, file, actor, fields["sha256"])
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

func httpError(err error) error {
	var databaseError *pgconn.PgError
	var networkError net.Error
	reason := "dependency_failure"
	switch {
	case errors.Is(err, context.Canceled):
		reason = "request_cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		reason = "deadline_exceeded"
	case errors.As(err, &networkError) && networkError.Timeout():
		reason = "network_timeout"
	case errors.As(err, &databaseError):
		reason = "database_error"
	}
	diagnostics := []apperror.Diagnostic{{Key: "reason", Value: reason}}
	if databaseError != nil {
		diagnostics = append(diagnostics, apperror.Diagnostic{Key: "sqlstate", Value: databaseError.Code})
	}
	wrap := func(kind apperror.Kind, code, detail string) error {
		return apperror.Wrap(err, kind, code, detail, "database backup").WithDiagnostics(diagnostics)
	}
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return wrap(apperror.KindUnavailable, "database_backup_unavailable", "database backup is temporarily unavailable")
	case errors.Is(err, ErrBusy):
		return wrap(apperror.KindConflict, "database_backup_busy", "a database restore is already active")
	case errors.Is(err, ErrJobNotFound):
		return wrap(apperror.KindNotFound, "database_backup_job_not_found", "the database restore task was not found")
	case errors.Is(err, ErrOutcomeUnknown):
		return wrap(apperror.KindUnavailable, "restore_outcome_unknown", "the database restore outcome could not be confirmed")
	case errors.Is(err, ErrTooLarge):
		return wrap(apperror.KindTooLarge, "database_backup_too_large", "the backup exceeds the upload limit")
	case errors.Is(err, ErrInvalid):
		return wrap(apperror.KindValidation, "database_backup_invalid", "the backup file or confirmation is invalid")
	case errors.Is(err, ErrTarget):
		return wrap(apperror.KindConflict, "database_backup_target_not_ready", "restore requires an initialized database")
	case errors.As(err, &databaseError):
		if databaseError.Code == "55000" || databaseError.Code == "55P03" {
			return wrap(apperror.KindConflict, "database_backup_target_not_ready", "the database is not ready for restoration")
		}
		return wrap(apperror.KindValidation, "database_backup_restore_failed", "the backup could not be restored; no changes were saved")
	default:
		return wrap(apperror.KindUnavailable, "database_backup_unavailable", "database backup is temporarily unavailable")
	}
}
