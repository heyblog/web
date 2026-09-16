package siteaudit

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"heyblog-api/internal/apperror"
)

func siteAuditDiagnostics(err error) []apperror.Diagnostic {
	cause := err
	for unwrapped := errors.Unwrap(cause); unwrapped != nil; unwrapped = errors.Unwrap(cause) {
		cause = unwrapped
	}
	diagnostics := []apperror.Diagnostic{{Key: "cause_type", Value: fmt.Sprintf("%T", cause)}}
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) {
		return diagnostics
	}
	diagnostics = append(diagnostics, apperror.Diagnostic{Key: "database_sqlstate", Value: databaseError.Code})
	if databaseError.ConstraintName != "" {
		diagnostics = append(diagnostics, apperror.Diagnostic{Key: "database_constraint", Value: databaseError.ConstraintName})
	}
	if databaseError.TableName != "" {
		diagnostics = append(diagnostics, apperror.Diagnostic{Key: "database_table", Value: databaseError.TableName})
	}
	return diagnostics
}
