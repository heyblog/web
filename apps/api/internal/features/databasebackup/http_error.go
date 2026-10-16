package databasebackup

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"

	"heyblog-api/internal/platform/apperror"
)

func httpError(err error) error {
	var databaseError *pgconn.PgError
	var networkError net.Error
	var fileError *os.PathError
	var transfer *transferError
	var bodyLimit *http.MaxBytesError
	var protocolError textproto.ProtocolError
	errors.As(err, &databaseError)
	reason := "dependency_failure"
	switch {
	case errors.Is(err, ErrUploadUnavailable):
		reason = "uploads_not_configured"
	case errors.Is(err, ErrUploadNotFound):
		reason = "upload_not_found"
	case errors.Is(err, ErrUploadState):
		reason = "upload_not_ready"
	case errors.Is(err, context.Canceled):
		reason = "request_cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		reason = "deadline_exceeded"
	case errors.As(err, &networkError) && networkError.Timeout():
		reason = "network_timeout"
	case errors.Is(err, ErrBusy):
		reason = "operation_busy"
	case errors.Is(err, ErrTooLarge), errors.As(err, &bodyLimit):
		reason = "upload_too_large"
	case errors.Is(err, ErrInvalid), errors.Is(err, multipart.ErrMessageTooLarge), errors.As(err, &protocolError):
		reason = "invalid_backup"
	case errors.Is(err, ErrTarget):
		reason = "target_not_ready"
	case errors.Is(err, ErrJobNotFound):
		reason = "job_not_found"
	case errors.Is(err, ErrOutcomeUnknown):
		reason = "outcome_unknown"
	case errors.As(err, &fileError):
		reason = "file_io"
	case errors.Is(err, io.ErrUnexpectedEOF):
		reason = "upload_incomplete"
	case errors.As(err, &transfer) && transfer.stage == "upload":
		reason = "upload_interrupted"
	case databaseError != nil:
		reason = "database_error"
	}
	diagnostics := []apperror.Diagnostic{{Key: "reason", Value: reason}}
	if errors.As(err, &transfer) {
		diagnostics = append(diagnostics, apperror.Diagnostic{Key: "stage", Value: transfer.stage}, apperror.Diagnostic{Key: "received_bytes", Value: strconv.FormatInt(transfer.bytes, 10)})
	}
	if databaseError != nil {
		diagnostics = append(diagnostics, apperror.Diagnostic{Key: "sqlstate", Value: databaseError.Code})
	}
	wrap := func(kind apperror.Kind, code, detail string) error {
		return apperror.Wrap(err, kind, code, detail, "database backup").WithDiagnostics(diagnostics)
	}
	switch {
	case errors.Is(err, ErrUploadUnavailable):
		return wrap(apperror.KindUnavailable, "database_backup_upload_unavailable", "backup uploads are not configured")
	case errors.Is(err, ErrUploadNotFound):
		return wrap(apperror.KindNotFound, "database_backup_upload_not_found", "the backup upload was not found")
	case errors.Is(err, ErrUploadState):
		return wrap(apperror.KindConflict, "database_backup_upload_not_ready", "the backup upload is not ready for this operation")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return wrap(apperror.KindUnavailable, "database_backup_unavailable", "database backup is temporarily unavailable")
	case errors.Is(err, ErrBusy):
		return wrap(apperror.KindConflict, "database_backup_busy", "a database backup operation is already active")
	case errors.Is(err, ErrJobNotFound):
		return wrap(apperror.KindNotFound, "database_backup_job_not_found", "the database restore task was not found")
	case errors.Is(err, ErrOutcomeUnknown):
		return wrap(apperror.KindUnavailable, "restore_outcome_unknown", "the database restore outcome could not be confirmed")
	case errors.Is(err, ErrTooLarge), errors.As(err, &bodyLimit):
		return wrap(apperror.KindTooLarge, "database_backup_too_large", "the backup exceeds the upload limit")
	case errors.Is(err, ErrInvalid), errors.Is(err, multipart.ErrMessageTooLarge), errors.As(err, &protocolError):
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
