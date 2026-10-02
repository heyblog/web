package siteaudit

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func parseUUID(value string) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(value); err != nil {
		return pgtype.UUID{}, fmt.Errorf("parse UUID: %w", err)
	}
	return parsed, nil
}

func uuidString(value pgtype.UUID) (string, error) {
	if !value.Valid {
		return "", errors.New("UUID is null")
	}
	driverValue, err := value.Value()
	if err != nil {
		return "", fmt.Errorf("format UUID: %w", err)
	}
	text, ok := driverValue.(string)
	if !ok {
		return "", fmt.Errorf("format UUID: unexpected value type %T", driverValue)
	}
	return text, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copyOfValue := value
	return &copyOfValue
}

func boolPointer(value bool) *bool {
	copyOfValue := value
	return &copyOfValue
}

func timestampPointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	copyOfValue := value.Time
	return &copyOfValue
}

func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
