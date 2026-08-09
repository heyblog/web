package bootstrap

import (
	"fmt"
	"reflect"
	"sort"
)

type stageError struct {
	stage string
	cause error
}

func withStage(stage string, cause error) error {
	if cause == nil {
		return nil
	}
	return &stageError{stage: stage, cause: cause}
}

func (err *stageError) Error() string {
	return fmt.Sprintf("%s: %v", err.stage, err.cause)
}

func (err *stageError) Unwrap() error {
	return err.cause
}

func failureMetadata(err error) ([]string, []string) {
	stages := make(map[string]struct{})
	leafTypes := make(map[string]struct{})
	var visit func(error)
	visit = func(current error) {
		if current == nil {
			return
		}
		if staged, ok := current.(*stageError); ok { //nolint:errorlint // Exact traversal retains every joined failure.
			stages[staged.stage] = struct{}{}
		}
		switch wrapped := current.(type) { //nolint:errorlint // errors.As stops at the first joined match.
		case interface{ Unwrap() []error }:
			for _, child := range wrapped.Unwrap() {
				visit(child)
			}
		case interface{ Unwrap() error }:
			visit(wrapped.Unwrap())
		default:
			leafTypes[reflect.TypeOf(current).String()] = struct{}{}
		}
	}
	visit(err)
	return sortedKeys(stages), sortedKeys(leafTypes)
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
