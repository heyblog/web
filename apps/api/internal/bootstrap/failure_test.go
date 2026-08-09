package bootstrap

import (
	"errors"
	"reflect"
	"testing"
)

func TestFailureMetadataRetainsEverySafeStageAndLeafType(t *testing.T) {
	t.Parallel()

	err := errors.Join(
		withStage("database_migration", errors.New("postgres://user:secret@example.test")),
		withStage("redis_open", testFailure{}),
	)
	stages, leafTypes := failureMetadata(err)

	if !reflect.DeepEqual(stages, []string{"database_migration", "redis_open"}) {
		t.Fatalf("stages = %v, want sorted safe stages", stages)
	}
	if !reflect.DeepEqual(leafTypes, []string{"*errors.errorString", "bootstrap.testFailure"}) {
		t.Fatalf("leaf types = %v, want sorted cause types", leafTypes)
	}
}

type testFailure struct{}

func (testFailure) Error() string { return "private failure" }
