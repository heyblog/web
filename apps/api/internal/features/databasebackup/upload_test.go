package databasebackup

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/httpapi"
)

type interruptedUpload struct {
	io.Reader
	err error
}

func TestBackupUploadFailureReleasesTheInspectionSlot(t *testing.T) {
	// Given
	operation := &routeOperation{}
	router := backupRouter(t, operation, routeAuth{user: auth.User{ID: "actor", Role: auth.RoleSysAdmin, Active: true}})
	broken := httptest.NewRequest(http.MethodPost, Path+"/inspect", interruptedUpload{strings.NewReader("--test\r\nContent-Disposition: form-data; name=\"file\"; filename=\"backup.json\"\r\n\r\npartial"), io.ErrUnexpectedEOF})
	broken.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	broken.Header.Set(httpapi.WebTokenHeader, "backup-test-web-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, broken)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("interrupted upload status=%d body=%s", response.Code, response.Body.String())
	}
	// When: a new inspection follows the interrupted upload.
	request := httptest.NewRequest(http.MethodPost, Path+"/inspect", strings.NewReader("--test\r\nContent-Disposition: form-data; name=\"file\"; filename=\"backup.json\"\r\n\r\n{}\r\n--test--\r\n"))
	request.Header = broken.Header.Clone()
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	// Then
	if response.Code != http.StatusOK || operation.calls != 1 {
		t.Fatalf("slot retained after failed upload: status=%d calls=%d", response.Code, operation.calls)
	}
}

func TestBackupUploadFileFailuresAreUnavailableInsteadOfInvalid(t *testing.T) {
	// Given
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	request := httptest.NewRequest(http.MethodPost, Path+"/inspect", strings.NewReader("--test\r\nContent-Disposition: form-data; name=\"file\"; filename=\"backup.json\"\r\n\r\n{}\r\n--test--\r\n"))
	request.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	// When
	_, err := upload(request)
	// Then
	var fileError *os.PathError
	if !errors.As(err, &fileError) {
		t.Fatalf("file failure cause lost: %v", err)
	}
	var problem *apperror.Error
	if !errors.As(httpError(err), &problem) || problem.Kind() != apperror.KindUnavailable {
		t.Fatalf("file failure classified as invalid: %v", err)
	}
	diagnostics := problem.Diagnostics()
	if len(diagnostics) == 0 || diagnostics[0].Value != "file_io" {
		t.Fatalf("file failure diagnostic=%v", diagnostics)
	}
}

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

func TestBackupUploadMalformedMIMEHeadersRemainValidationFailures(t *testing.T) {
	t.Parallel()
	// Given: the complete request has a malformed header, rather than an interrupted transfer.
	request := httptest.NewRequest(http.MethodPost, Path+"/inspect", strings.NewReader("--test\r\ninvalid header without a colon\r\n\r\n{}\r\n--test--\r\n"))
	request.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	// When
	_, err := upload(request)
	// Then
	var problem *apperror.Error
	if !errors.As(httpError(err), &problem) || problem.Kind() != apperror.KindValidation || problem.Diagnostics()[0].Value != "invalid_backup" {
		t.Fatalf("malformed header classified as a dependency failure: %v", err)
	}
}

func (body interruptedUpload) Read(buffer []byte) (int, error) {
	n, err := body.Reader.Read(buffer)
	if errors.Is(err, io.EOF) {
		return n, body.err
	}
	return n, err
}

func TestBackupUploadPreservesInterruptedTransfersAndCleansTemporaryFiles(t *testing.T) {
	// Given
	directory := t.TempDir()
	t.Setenv("TMPDIR", directory)
	for _, cause := range []error{io.ErrUnexpectedEOF, context.Canceled, context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) {
			body := interruptedUpload{strings.NewReader("--test\r\nContent-Disposition: form-data; name=\"file\"; filename=\"backup.json\"\r\n\r\npartial"), cause}
			request := httptest.NewRequest(http.MethodPost, Path+"/inspect", body)
			request.Header.Set("Content-Type", "multipart/form-data; boundary=test")
			// When
			_, err := upload(request)
			// Then
			if !errors.Is(err, cause) {
				t.Fatalf("lost upload cause: %v", err)
			}
			var problem *apperror.Error
			if !errors.As(httpError(err), &problem) || problem.Kind() != apperror.KindUnavailable {
				t.Fatalf("interrupted upload classified as invalid backup: %v", err)
			}
			files, readErr := os.ReadDir(directory)
			if readErr != nil || len(files) != 0 {
				t.Fatalf("upload temporary retained: %v, count=%d", readErr, len(files))
			}
		})
	}
}
