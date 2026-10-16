package databasebackup

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/httpapi"

	"github.com/danielgtaylor/huma/v2"
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
type inspectInput struct {
	Body struct {
		UploadID string `json:"upload_id" format:"uuid"`
	}
}
type restoreInput struct {
	Body struct {
		UploadID     string `json:"upload_id" format:"uuid"`
		SHA256       string `json:"sha256" minLength:"64" maxLength:"64" pattern:"^[0-9a-f]{64}$"`
		Confirmation string `json:"confirmation" enum:"RESTORE"`
	}
}

func BodyLimitOverrides() map[httpapi.Route]int64 {
	return map[httpapi.Route]int64{{Method: http.MethodPut, Path: Path + "/uploads/:id/parts/:number"}: UploadPartSize}
}

func backupOperation(method, path, id string, authenticator Authenticator, token string) huma.Operation {
	return huma.Operation{OperationID: "database-backup-" + id, Method: method, Path: Path + path,
		Tags: []string{"Database backup"}, MaxBodyBytes: 64 << 10, BodyReadTimeout: 30 * time.Second,
		Errors: []int{400, 401, 403, 404, 409, 413, 422, 503}, Security: []map[string][]string{{"webToken": {}, "accessCookie": {}}},
		Middlewares: huma.Middlewares{httpapi.HumaWebAuthorization(token), authorize(authenticator)}}
}

func RegisterRoutes(api huma.API, service Operations, jobs *Jobs, authenticator Authenticator, webToken string, logger *slog.Logger, uploads *Uploads) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if uploads == nil {
		uploads = &Uploads{settings: UploadSettings{Transport: "disabled", PartSize: UploadPartSize, MaxFileSize: FileLimit}, logger: logger}
	}
	jobs.logger = logger
	httpapi.Register(api, backupOperation(http.MethodPost, "/export", "export", authenticator, webToken), func(ctx context.Context, _ *emptyInput) (*exportOutput, error) {
		work, cancel := operationContext(ctx)
		defer cancel()
		file, err := service.Export(work)
		if err != nil {
			return nil, httpError(err)
		}
		return &exportOutput{ContentType: "application/json", ContentDisposition: `attachment; filename="heyblog-database-backup.json"`, CacheControl: "private, no-store", Body: func(response huma.Context) {
			defer func() {
				if err := errorsWithCleanup(nil, file); err != nil {
					logger.Warn("database backup temporary file cleanup failed", "event", "database_backup_cleanup_failed")
				}
			}()
			if _, err := io.Copy(response.BodyWriter(), file); err != nil {
				logger.Warn("database backup download interrupted", "event", "database_backup_download_interrupted")
			}
		}}, nil
	})
	httpapi.Register(api, backupOperation(http.MethodPost, "/inspect", "inspect", authenticator, webToken), func(ctx context.Context, input *inspectInput) (*output[Inspection], error) {
		work, cancel := operationContext(ctx)
		defer cancel()
		actor, _ := ctx.Value(actorKey{}).(string)
		started := time.Now()
		logger.InfoContext(ctx, "database backup inspection started", "event", "database_backup_inspection_started", "upload_id", input.Body.UploadID)
		preview, err := uploads.Inspect(work, actor, input.Body.UploadID, service, jobs)
		if err != nil {
			return nil, httpError(err)
		}
		logger.InfoContext(ctx, "database backup inspection completed", "event", "database_backup_inspection_completed", "upload_id", input.Body.UploadID, "duration_ms", time.Since(started).Milliseconds())
		return &output[Inspection]{CacheControl: "private, no-store", Body: preview}, nil
	})
	restore := backupOperation(http.MethodPost, "/restore", "restore", authenticator, webToken)
	restore.DefaultStatus = http.StatusAccepted
	httpapi.Register(api, restore, func(ctx context.Context, input *restoreInput) (*output[RestoreJob], error) {
		actor, _ := ctx.Value(actorKey{}).(string)
		job, err := uploads.Restore(ctx, actor, input.Body.UploadID, input.Body.SHA256, jobs)
		if err != nil {
			return nil, httpError(err)
		}
		return &output[RestoreJob]{CacheControl: "private, no-store", Body: job}, nil
	})
	registerUploadRoutes(api, uploads, authenticator, webToken)
	registerJobRoutes(api, jobs, authenticator, webToken, uploads)
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
