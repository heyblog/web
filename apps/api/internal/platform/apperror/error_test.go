package apperror

import (
	"errors"
	"testing"
)

func TestErrorPreservesClassificationAndCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("database unavailable")
	err := Wrap(cause, KindUnavailable, CodeServiceUnavailable, "service is temporarily unavailable", "load account")

	if !errors.Is(err, cause) {
		t.Fatal("errors.Is() = false, want wrapped cause")
	}
	if err.Kind() != KindUnavailable || err.Code() != CodeServiceUnavailable {
		t.Fatalf("classification = (%q, %q), want unavailable service code", err.Kind(), err.Code())
	}
	if err.PublicDetail() != "service is temporarily unavailable" || err.Operation() != "load account" {
		t.Fatalf("public metadata = (%q, %q), want configured values", err.PublicDetail(), err.Operation())
	}
}

func TestErrorCopiesInvalidParameters(t *testing.T) {
	t.Parallel()

	parameters := []InvalidParam{{Name: "title", Reason: "is required"}}
	err := New(KindValidation, CodeValidationFailed, "request validation failed").WithInvalidParams(parameters)
	parameters[0].Reason = "changed"
	got := err.InvalidParams()
	got[0].Reason = "also changed"

	if err.InvalidParams()[0].Reason != "is required" {
		t.Fatal("InvalidParams() exposed mutable error state")
	}
}

func TestErrorCopiesSafeDiagnostics(t *testing.T) {
	t.Parallel()

	diagnostics := []Diagnostic{{Key: "database_sqlstate", Value: "23505"}}
	err := New(KindInternal, CodeInternal, "an unexpected error occurred").WithDiagnostics(diagnostics)
	diagnostics[0].Value = "changed"
	got := err.Diagnostics()
	got[0].Value = "also changed"

	if err.Diagnostics()[0].Value != "23505" {
		t.Fatal("Diagnostics() exposed mutable error state")
	}
}
