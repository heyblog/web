package siteaudit

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"

	"heyblog-api/internal/apperror"
)

func TestRegisterRoutesUsesShortIDForPublicMaintenancePaths(t *testing.T) {
	t.Parallel()

	router := gin.New()
	configuration := huma.DefaultConfig("test", "1.0.0")
	configuration.OpenAPIPath = ""
	configuration.DocsPath = ""
	configuration.SchemasPath = ""
	api := humagin.New(router, configuration)
	if err := RegisterRoutes(api, &Service{}, "test-web-token", nil); err != nil {
		t.Fatalf("RegisterRoutes() error = %v", err)
	}

	wanted := map[string]bool{
		"POST /site-submissions/:shortId/updates":      false,
		"POST /site-submissions/:shortId/deletions":    false,
		"POST /site-submissions/:shortId/restorations": false,
		"GET /site-submissions/site-availability":      false,
		"GET /site-submissions/sites/:shortId":         false,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, exists := wanted[key]; exists {
			wanted[key] = true
		}
	}
	for route, found := range wanted {
		if !found {
			t.Errorf("registered routes missing %s", route)
		}
	}
}

func TestMapServiceErrorKeepsOnlySafeDatabaseDiagnostics(t *testing.T) {
	t.Parallel()

	databaseErr := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "site_tags_tag_id_fkey",
		TableName:      "site_tags",
		Message:        "secret connection context",
		Detail:         "private row contents",
	}
	err := mapServiceError(fmt.Errorf("assign reviewed tag: %w", databaseErr), "review site audit")

	var applicationErr *apperror.Error
	if !errors.As(err, &applicationErr) {
		t.Fatalf("mapped error type = %T, want *apperror.Error", err)
	}
	got := make(map[string]string)
	for _, diagnostic := range applicationErr.Diagnostics() {
		got[diagnostic.Key] = diagnostic.Value
	}
	if got["cause_type"] != "*pgconn.PgError" ||
		got["database_sqlstate"] != "23503" ||
		got["database_constraint"] != "site_tags_tag_id_fkey" ||
		got["database_table"] != "site_tags" {
		t.Fatalf("diagnostics = %#v, want safe database fields", got)
	}
	for _, value := range got {
		if strings.Contains(value, "secret") || strings.Contains(value, "private") {
			t.Fatalf("diagnostics exposed private database content: %#v", got)
		}
	}
}

func TestMapServiceErrorReturnsConflictForRegisteredSiteAddress(t *testing.T) {
	t.Parallel()

	// Given a canonical-site address conflict from the application service.
	serviceErr := newServiceError("site_address_conflict", http.StatusConflict, "the site address is already registered")

	// When the review handler maps the error to its HTTP boundary.
	err := mapServiceError(serviceErr, "review site audit")

	// Then callers receive a stable conflict code and operators receive a stable operation.
	var applicationErr *apperror.Error
	if !errors.As(err, &applicationErr) {
		t.Fatalf("mapped error type = %T, want *apperror.Error", err)
	}
	if applicationErr.Kind() != apperror.KindConflict || applicationErr.Code() != "site_address_conflict" {
		t.Fatalf("mapped error = (%q, %q), want conflict site_address_conflict", applicationErr.Kind(), applicationErr.Code())
	}
	if applicationErr.Operation() != "review site audit" {
		t.Fatalf("operation = %q, want review site audit", applicationErr.Operation())
	}
}

func TestMapServiceErrorReturnsValidationForSiteURLPurposeConflict(t *testing.T) {
	t.Parallel()

	err := mapServiceError(ErrSiteURLPurposeConflict, "submit site audit")

	var applicationErr *apperror.Error
	if !errors.As(err, &applicationErr) {
		t.Fatalf("mapped error type = %T, want *apperror.Error", err)
	}
	if applicationErr.Kind() != apperror.KindValidation || applicationErr.Code() != "site_url_purpose_conflict" {
		t.Fatalf("mapped error = (%q, %q), want validation site_url_purpose_conflict", applicationErr.Kind(), applicationErr.Code())
	}
}
