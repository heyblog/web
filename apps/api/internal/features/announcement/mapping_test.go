package announcement

import (
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func testRow(start time.Time) dbgen.ContentAnnouncement {
	return dbgen.ContentAnnouncement{Kind: "MAIN", Status: "PUBLISHED", StartsAt: pgtype.Timestamptz{Time: start, Valid: true}, EndsAt: pgtype.Timestamptz{Time: start.Add(time.Hour), Valid: true}}
}
