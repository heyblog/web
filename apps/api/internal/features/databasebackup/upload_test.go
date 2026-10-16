package databasebackup

import (
	"errors"
	"heyblog-api/internal/platform/apperror"
	"net/http"
	"testing"
)

func TestBackupExpectedFailuresHaveAccurateDiagnostics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		cause  error
		reason string
		kind   apperror.Kind
	}{
		{ErrBusy, "operation_busy", apperror.KindConflict},
		{ErrInvalid, "invalid_backup", apperror.KindValidation},
		{ErrTooLarge, "upload_too_large", apperror.KindTooLarge},
		{&http.MaxBytesError{Limit: BodyLimit}, "upload_too_large", apperror.KindTooLarge},
	} {
		// Given / When
		var problem *apperror.Error
		if !errors.As(httpError(test.cause), &problem) {
			t.Fatal("missing typed failure")
		}
		// Then
		if problem.Kind() != test.kind || problem.Diagnostics()[0].Value != test.reason {
			t.Fatalf("failure classification: %v %v", problem.Kind(), problem.Diagnostics())
		}
	}
}
