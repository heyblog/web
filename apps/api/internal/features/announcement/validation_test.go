package announcement

import (
	"errors"
	"testing"
	"time"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/apperror"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestManagementPermissionRequiresAdministratorRole(t *testing.T) {
	for _, test := range []struct {
		role        auth.Role
		permissions []auth.Permission
		allowed     bool
	}{
		{auth.RoleSysAdmin, nil, true}, {auth.RoleAdmin, []auth.Permission{auth.PermissionAnnouncementManage}, true},
		{auth.RoleAdmin, nil, false}, {auth.RoleUser, []auth.Permission{auth.PermissionAnnouncementManage}, false},
	} {
		t.Run(string(test.role)+" permission", func(t *testing.T) {
			// Given a user role and permission set.
			user := auth.User{Role: test.role, Permissions: test.permissions}
			// When management authorization is evaluated.
			got := canManage(user)
			// Then both role and permission requirements hold.
			if got != test.allowed {
				t.Fatalf("canManage = %v, want %v", got, test.allowed)
			}
		})
	}
}

func TestInputRejectsInvalidAnnouncementFields(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	text := "value"
	tests := []struct {
		name   string
		change func(*Input)
	}{
		{"kind", func(input *Input) { input.Kind = "MODAL" }}, {"blank title", func(input *Input) { input.Title = "  " }},
		{"banner priority", func(input *Input) { input.Kind = "BANNER"; input.Priority = 1 }},
		{"action shape", func(input *Input) { input.ActionType = "NONE"; input.ActionLabel = &text }},
		{"window", func(input *Input) { input.StartsAt = &now; input.EndsAt = &now }},
		{"missing start", func(input *Input) { input.EndsAt = &now }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given a valid draft with one invalid field.
			input := Input{Kind: "MAIN", Title: "Title", ActionType: "NONE"}
			test.change(&input)
			// When the boundary parses it.
			err := validateInput(input)
			// Then the invalid shape is rejected.
			var failure *apperror.Error
			if !errors.As(err, &failure) || failure.Kind() != apperror.KindValidation {
				t.Fatalf("validation error = %v", err)
			}
		})
	}
}

func TestVersionPreservesLargeIntegersAndRejectsOverflow(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{{"9007199254740993", true}, {"9223372036854775807", true}, {"9223372036854775808", false}, {"0", false}, {"01", false}, {"-1", false}} {
		t.Run(test.value, func(t *testing.T) {
			// Given a decimal version transmitted as a string.
			// When the boundary parses it.
			_, err := parseVersion(test.value)
			// Then precision is retained within signed bigint range.
			if (err == nil) != test.valid {
				t.Fatalf("parseVersion error = %v", err)
			}
		})
	}
}

func TestDatabaseConflictDoesNotExposeDatabaseDetails(t *testing.T) {
	// Given a PostgreSQL exclusion conflict with private diagnostics.
	source := &pgconn.PgError{Code: "23P01", Message: "private row data"}
	// When the persistence boundary maps it.
	err := databaseError(source)
	// Then the stable conflict contract exposes only a safe message.
	var failure *apperror.Error
	if !errors.As(err, &failure) || failure.Code() != "banner_window_conflict" || failure.Kind() != apperror.KindConflict {
		t.Fatalf("mapped error = %v", err)
	}
}

func TestEffectiveStatusUsesInclusiveStartExclusiveEnd(t *testing.T) {
	for _, test := range []struct {
		name     string
		offset   time.Duration
		expected string
	}{{"before", -time.Nanosecond, "SCHEDULED"}, {"start", 0, "ACTIVE"}, {"end", time.Hour, "EXPIRED"}} {
		t.Run(test.name, func(t *testing.T) {
			// Given a published announcement with a bounded visibility window.
			now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
			row := testRow(now)
			// When its display state is evaluated at a boundary.
			result := mapRow(row, now.Add(test.offset))
			// Then start is inclusive and end is exclusive.
			if result.EffectiveStatus != test.expected {
				t.Fatalf("state = %s", result.EffectiveStatus)
			}
		})
	}
}
