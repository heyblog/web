package apikey

import (
	"errors"
	"testing"

	"heyblog-api/internal/apperror"
)

func TestHTTPErrorMapsClientNameConflict(t *testing.T) {
	t.Parallel()

	mapped := HTTPError(ErrClientNameConflict)
	var applicationError *apperror.Error
	if !errors.As(mapped, &applicationError) {
		t.Fatalf("HTTPError() = %T, want *apperror.Error", mapped)
	}
	if applicationError.Kind() != apperror.KindConflict || applicationError.Code() != "api_client_name_conflict" {
		t.Fatalf("HTTPError() = (%q, %q), want conflict api_client_name_conflict", applicationError.Kind(), applicationError.Code())
	}
}
