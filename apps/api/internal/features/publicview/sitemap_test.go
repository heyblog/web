package publicview

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/platform/apperror"
)

func TestSitemapPagesExposeCursorOnlyWhenMoreRowsExist(t *testing.T) {
	t.Parallel()
	for _, kind := range []SitemapKind{SitemapSites, SitemapAnnouncements} {
		for _, count := range []int{0, 1, 1000, 1001} {
			t.Run(fmt.Sprintf("%s/%d", kind, count), func(t *testing.T) {
				t.Parallel()
				// Given an authoritative query result at the paging boundary.
				stamp := timestamp(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
				ids := make([]pgtype.UUID, count)
				sites := make([]dbgen.ListSitemapSitesRow, count)
				announcements := make([]dbgen.ListSitemapAnnouncementsRow, count)
				for index := range count {
					if err := ids[index].Scan(fmt.Sprintf("00000000-0000-7000-8000-%012d", index+1)); err != nil {
						t.Fatal(err)
					}
					sites[index] = dbgen.ListSitemapSitesRow{ID: ids[index], ShortID: "A1b2C3d4E"}
					announcements[index] = dbgen.ListSitemapAnnouncementsRow{ID: ids[index], StartsAt: stamp, PublishedAt: stamp, UpdatedAt: stamp}
				}
				after := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
				queries := queryStub{
					sitemapSites: func(_ context.Context, cursor pgtype.UUID) ([]dbgen.ListSitemapSitesRow, error) {
						if cursor != after {
							t.Fatalf("cursor = %v", cursor)
						}
						return sites, nil
					},
					sitemapAnnouncements: func(_ context.Context, cursor pgtype.UUID) ([]dbgen.ListSitemapAnnouncementsRow, error) {
						if cursor != after {
							t.Fatalf("cursor = %v", cursor)
						}
						return announcements, nil
					},
				}
				// When the service assembles the sitemap page.
				page, err := New(queries).Sitemap(context.Background(), SitemapQuery{Kind: kind, After: after})
				// Then the lookahead is excluded and the cursor identifies the last emitted item.
				if err != nil {
					t.Fatal(err)
				}
				if page.Items == nil || len(page.Items) != min(count, 1000) {
					t.Fatalf("items count = %d", len(page.Items))
				}
				if count > 1000 {
					if page.NextAfter == nil || *page.NextAfter != ids[999].String() {
						t.Fatalf("nextAfter = %v", page.NextAfter)
					}
				} else if page.NextAfter != nil {
					t.Fatalf("unexpected cursor = %s", *page.NextAfter)
				}
			})
		}
	}
}

func TestSitemapQueryFailureRetainsExistingInternalErrorPolicy(t *testing.T) {
	t.Parallel()
	for _, kind := range []SitemapKind{SitemapSites, SitemapAnnouncements} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			// Given a failed sitemap query.
			failure := errors.New("query failed")
			service := New(queryStub{
				sitemapSites:         func(context.Context, pgtype.UUID) ([]dbgen.ListSitemapSitesRow, error) { return nil, failure },
				sitemapAnnouncements: func(context.Context, pgtype.UUID) ([]dbgen.ListSitemapAnnouncementsRow, error) { return nil, failure },
			})
			// When the service is asked for a page.
			_, err := service.Sitemap(context.Background(), SitemapQuery{Kind: kind})
			// Then the safe typed failure preserves its diagnostic cause.
			var applicationError *apperror.Error
			if !errors.Is(err, failure) || !errors.As(err, &applicationError) || applicationError.Kind() != apperror.KindInternal {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestAnnouncementDetailReturnsActualPublicationAndUpdateTimes(t *testing.T) {
	t.Parallel()
	// Given a scheduled announcement published before its public start and later edited.
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	row := dbgen.ContentAnnouncement{
		Kind: "MAIN", Status: "PUBLISHED", ActionType: "NONE", StartsAt: timestamp(start),
		PublishedAt: timestamp(start.Add(-time.Hour)), UpdatedAt: timestamp(start.Add(time.Minute)),
	}
	// When the public detail read maps persistence metadata.
	view, err := New(queryStub{announcement: row}).AnnouncementByID(context.Background(), "00000000-0000-7000-8000-000000000001")
	// Then neither date is synthesized from the scheduled start.
	if err != nil {
		t.Fatal(err)
	}
	if !view.PublishedAt.Equal(row.PublishedAt.Time) || !view.UpdatedAt.Equal(row.UpdatedAt.Time) {
		t.Fatalf("dates = (%v, %v)", view.PublishedAt, view.UpdatedAt)
	}
}

func TestSitemapAnnouncementRejectsMissingPersistenceTimestamps(t *testing.T) {
	t.Parallel()
	for _, missing := range []string{"startsAt", "publishedAt", "updatedAt"} {
		t.Run(missing, func(t *testing.T) {
			t.Parallel()
			// Given malformed persistence metadata that cannot represent a published page.
			stamp := timestamp(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
			row := dbgen.ListSitemapAnnouncementsRow{StartsAt: stamp, PublishedAt: stamp, UpdatedAt: stamp}
			switch missing {
			case "startsAt":
				row.StartsAt = pgtype.Timestamptz{}
			case "publishedAt":
				row.PublishedAt = pgtype.Timestamptz{}
			case "updatedAt":
				row.UpdatedAt = pgtype.Timestamptz{}
			}
			service := New(queryStub{sitemapAnnouncements: func(context.Context, pgtype.UUID) ([]dbgen.ListSitemapAnnouncementsRow, error) {
				return []dbgen.ListSitemapAnnouncementsRow{row}, nil
			}})
			// When the sitemap read maps that row.
			_, err := service.Sitemap(context.Background(), SitemapQuery{Kind: SitemapAnnouncements})
			// Then an internal failure replaces an apparently successful invalid page.
			var applicationError *apperror.Error
			if !errors.As(err, &applicationError) || applicationError.Kind() != apperror.KindInternal {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func (stub queryStub) ListSitemapSites(ctx context.Context, after pgtype.UUID) ([]dbgen.ListSitemapSitesRow, error) {
	if stub.sitemapSites != nil {
		return stub.sitemapSites(ctx, after)
	}
	return []dbgen.ListSitemapSitesRow{}, nil
}

func (stub queryStub) ListSitemapAnnouncements(ctx context.Context, after pgtype.UUID) ([]dbgen.ListSitemapAnnouncementsRow, error) {
	if stub.sitemapAnnouncements != nil {
		return stub.sitemapAnnouncements(ctx, after)
	}
	return []dbgen.ListSitemapAnnouncementsRow{}, nil
}
