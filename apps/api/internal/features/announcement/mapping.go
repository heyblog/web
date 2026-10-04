package announcement

import (
	"strconv"
	"time"

	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5/pgtype"
)

func nullableTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
func databaseTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}
func nullableID(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	id := value.String()
	return &id
}
func mapRow(row dbgen.ContentAnnouncement, now time.Time) ManagedAnnouncement {
	result := ManagedAnnouncement{
		ID: row.ID.String(), Input: Input{Kind: row.Kind, Title: row.Title, BodyMarkdown: row.BodyMarkdown, Priority: row.Priority, ActionType: row.ActionType, ActionLabel: row.ActionLabel, ActionPath: row.ActionPath, ActionExternalURL: row.ActionExternalUrl, StartsAt: nullableTime(row.StartsAt), EndsAt: nullableTime(row.EndsAt)},
		Status: row.Status, RowVersion: strconv.FormatInt(row.RowVersion, 10), CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
		PublishedAt: nullableTime(row.PublishedAt), ArchivedAt: nullableTime(row.ArchivedAt), CreatedBy: nullableID(row.CreatedBy), UpdatedBy: nullableID(row.UpdatedBy), PublishedBy: nullableID(row.PublishedBy), ArchivedBy: nullableID(row.ArchivedBy),
	}
	switch row.Status {
	case "DRAFT", "ARCHIVED":
		result.EffectiveStatus = row.Status
	case "PUBLISHED":
		switch {
		case row.StartsAt.Time.After(now):
			result.EffectiveStatus = "SCHEDULED"
		case row.EndsAt.Valid && !row.EndsAt.Time.After(now):
			result.EffectiveStatus = "EXPIRED"
		default:
			result.EffectiveStatus = "ACTIVE"
		}
	}
	return result
}
