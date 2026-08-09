package database

import (
	"context"
	"strings"
	"testing"
)

func TestMigrateRejectsInvalidDatabaseURL(t *testing.T) {
	t.Parallel()

	err := Migrate(context.Background(), "not a postgres url")
	if err == nil {
		t.Fatal("Migrate() error = nil, want invalid URL error")
	}
	if !strings.Contains(err.Error(), "parse migration database URL") {
		t.Fatalf("Migrate() error = %q, want parse migration database URL", err)
	}
}
