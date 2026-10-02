package dataimport

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func parseUUIDText(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		return pgtype.UUID{}, err
	}
	return id, nil
}

func mustUUID(value string) pgtype.UUID {
	var result pgtype.UUID
	if err := result.Scan(value); err != nil {
		panic(fmt.Sprintf("validated import UUID %q is invalid: %v", value, err))
	}
	return result
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func nullableLocation(include bool, value string) *string {
	if !include {
		return nil
	}
	return &value
}

func nullableText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
