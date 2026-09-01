package publicview

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/apperror"
	dbgen "heyblog-api/internal/database/gen"
)

func TestHomeReturnsStableDailyWindowAndNoFabricatedAnnouncement(t *testing.T) {
	t.Parallel()

	rows := make([]dbgen.DirectorySite, 8)
	for index := range rows {
		rows[index] = testSite(string(rune('A'+index)) + "1b2C3d4E")
	}
	var calls []dbgen.ListVisibleSitesParams
	queries := queryStub{
		count: 8,
		listVisible: func(_ context.Context, params dbgen.ListVisibleSitesParams) ([]dbgen.DirectorySite, error) {
			calls = append(calls, params)
			start := int(params.Offset)
			end := min(start+int(params.Limit), len(rows))
			return rows[start:end], nil
		},
		announcementErr: pgx.ErrNoRows,
	}
	service := New(queries)
	service.now = func() time.Time {
		return time.Date(2026, time.August, 11, 22, 0, 0, 0, time.FixedZone("test", 8*60*60))
	}

	first, err := service.Home(context.Background())
	if err != nil {
		t.Fatalf("Home() error = %v", err)
	}
	firstCalls := append([]dbgen.ListVisibleSitesParams(nil), calls...)
	calls = nil
	second, err := service.Home(context.Background())
	if err != nil {
		t.Fatalf("Home() second error = %v", err)
	}

	if first.SiteCount != 8 || len(first.Sites) != 6 || first.Announcement != nil {
		t.Fatalf("Home() = %#v", first)
	}
	if !reflect.DeepEqual(first.Sites, second.Sites) || !reflect.DeepEqual(firstCalls, calls) {
		t.Fatalf("daily window changed: first=%#v second=%#v calls=%#v/%#v", first.Sites, second.Sites, firstCalls, calls)
	}
	for _, card := range first.Sites {
		if card.HomepageURL != "https://example.com/blog" {
			t.Fatalf("homepage URL = %q", card.HomepageURL)
		}
	}
}

func TestHomeRejectsOutOfRangeDirectoryCounts(t *testing.T) {
	t.Parallel()

	for _, count := range []int64{-1, int64(math.MaxInt32) + 1} {
		count := count
		t.Run(fmt.Sprintf("count_%d", count), func(t *testing.T) {
			t.Parallel()
			listCalled := false
			service := New(queryStub{
				count: count,
				listVisible: func(context.Context, dbgen.ListVisibleSitesParams) ([]dbgen.DirectorySite, error) {
					listCalled = true
					return nil, nil
				},
			})

			if _, err := service.Home(context.Background()); err == nil {
				t.Fatal("Home() error = nil, want out-of-range error")
			}
			if listCalled {
				t.Fatal("ListVisibleSites() called for out-of-range count")
			}
		})
	}
}

func TestHomeMapsLeadingAnnouncementAction(t *testing.T) {
	t.Parallel()

	label := "查看说明"
	path := "/about"
	service := New(queryStub{
		count: 0,
		announcement: dbgen.ContentAnnouncement{
			Title:       "目录维护公告",
			ActionType:  "INTERNAL",
			ActionLabel: &label,
			ActionPath:  &path,
			StartsAt:    timestamp(time.Date(2026, time.August, 11, 0, 0, 0, 0, time.UTC)),
		},
	})

	view, err := service.Home(context.Background())
	if err != nil {
		t.Fatalf("Home() error = %v", err)
	}
	if view.Announcement == nil || view.Announcement.Action == nil || view.Announcement.Action.Href != path {
		t.Fatalf("announcement = %#v", view.Announcement)
	}
}

func TestSiteProfileMapsOnlyPublicReadModel(t *testing.T) {
	t.Parallel()

	row := testSite("A1b2C3d4E")
	feedRef := "/feed.xml"
	resourceURL := "https://resources.example.net/sitemap.xml"
	homepage := "https://astro.build"
	repository := "https://github.com/withastro/astro"
	queries := queryStub{
		byShortID: row,
		feeds: []dbgen.DirectorySiteFeed{{
			Name: "主订阅", LocationType: "RELATIVE", UrlRef: &feedRef, Format: "ATOM", IsEnabled: true, IsDefault: true,
		}},
		resources: []dbgen.DirectorySiteResource{{
			Kind: "SITEMAP", LocationType: "EXTERNAL", ExternalUrl: &resourceURL,
		}},
		tags: []dbgen.ListPublicSiteTagsRow{
			{Role: "PRIMARY", Name: "技术", Slug: "technology", Description: "技术写作"},
			{Role: "WARNING", Name: "访问提示", Slug: "access-notice", Description: "部分地区可能较慢"},
		},
		technologies: []dbgen.ListPublicSiteSoftwareComponentsRow{{
			Role: "FRAMEWORK", Name: "Astro", HomepageUrl: &homepage, RepositoryUrl: &repository, IsOpenSource: true,
		}},
	}
	service := New(queries)

	profile, err := service.SiteByIdentifier(context.Background(), SiteIdentifier{
		Kind: IdentifierShortID, Value: row.ShortID,
	})
	if err != nil {
		t.Fatalf("SiteByIdentifier() error = %v", err)
	}
	if profile.HomepageURL != "https://example.com/blog" || profile.Feeds[0].URL != "https://example.com/feed.xml" {
		t.Fatalf("profile URLs = homepage:%q feed:%#v", profile.HomepageURL, profile.Feeds)
	}
	if len(profile.Topics) != 1 || profile.Topics[0].Role != "PRIMARY" || len(profile.Warnings) != 1 {
		t.Fatalf("profile tags = topics:%#v warnings:%#v", profile.Topics, profile.Warnings)
	}
	if len(profile.Resources) != 1 || profile.Resources[0].URL != resourceURL || len(profile.Technologies) != 1 {
		t.Fatalf("profile resources = %#v technologies = %#v", profile.Resources, profile.Technologies)
	}
}

func TestSiteProfileTreatsInvisibleAndMissingSitesAsNotFound(t *testing.T) {
	t.Parallel()

	hidden := testSite("A1b2C3d4E")
	hidden.Visibility = "HIDDEN"
	tests := []struct {
		name    string
		queries queryStub
	}{
		{name: "hidden", queries: queryStub{byShortID: hidden}},
		{name: "missing", queries: queryStub{byShortIDErr: pgx.ErrNoRows}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := New(test.queries)
			_, err := service.SiteByIdentifier(context.Background(), SiteIdentifier{
				Kind: IdentifierShortID, Value: "A1b2C3d4E",
			})
			var applicationError *apperror.Error
			if !errors.As(err, &applicationError) || applicationError.Kind() != apperror.KindNotFound {
				t.Fatalf("error = %v, want not found", err)
			}
		})
	}
}

func TestSiteProfileRejectsInvalidIdentifierBeforeQuery(t *testing.T) {
	t.Parallel()

	service := New(queryStub{})
	_, err := service.SiteByIdentifier(context.Background(), SiteIdentifier{
		Kind: IdentifierShortID, Value: "bad",
	})
	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) || applicationError.Kind() != apperror.KindBadRequest {
		t.Fatalf("error = %v, want bad request", err)
	}
}

func testSite(shortID string) dbgen.DirectorySite {
	joined := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
	return dbgen.DirectorySite{
		ID:             pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		ShortID:        shortID,
		Name:           "示例博客",
		Scheme:         "https",
		NormalizedHost: "example.com",
		BasePath:       "/blog",
		Summary:        "记录技术与生活。",
		AccessScope:    "ALL",
		Visibility:     "VISIBLE",
		JoinedAt:       timestamp(joined),
		UpdatedAt:      timestamp(joined.Add(time.Hour)),
	}
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

type queryStub struct {
	count           int64
	countErr        error
	listVisible     func(context.Context, dbgen.ListVisibleSitesParams) ([]dbgen.DirectorySite, error)
	announcement    dbgen.ContentAnnouncement
	announcementErr error
	byID            dbgen.DirectorySite
	byIDErr         error
	byShortID       dbgen.DirectorySite
	byShortIDErr    error
	byCustomID      dbgen.DirectorySite
	byCustomIDErr   error
	feeds           []dbgen.DirectorySiteFeed
	feedsErr        error
	resources       []dbgen.DirectorySiteResource
	resourcesErr    error
	tags            []dbgen.ListPublicSiteTagsRow
	tagsErr         error
	technologies    []dbgen.ListPublicSiteSoftwareComponentsRow
	technologiesErr error
}

func (stub queryStub) CountVisibleSites(context.Context) (int64, error) {
	return stub.count, stub.countErr
}

func (stub queryStub) ListVisibleSites(
	ctx context.Context,
	params dbgen.ListVisibleSitesParams,
) ([]dbgen.DirectorySite, error) {
	if stub.listVisible != nil {
		return stub.listVisible(ctx, params)
	}
	return []dbgen.DirectorySite{}, nil
}

func (stub queryStub) GetLeadingActiveMainAnnouncement(context.Context) (dbgen.ContentAnnouncement, error) {
	return stub.announcement, stub.announcementErr
}

func (stub queryStub) GetSiteByID(context.Context, pgtype.UUID) (dbgen.DirectorySite, error) {
	return stub.byID, stub.byIDErr
}

func (stub queryStub) GetSiteByShortID(context.Context, string) (dbgen.DirectorySite, error) {
	return stub.byShortID, stub.byShortIDErr
}

func (stub queryStub) GetSiteByCustomID(context.Context, *string) (dbgen.DirectorySite, error) {
	return stub.byCustomID, stub.byCustomIDErr
}

func (stub queryStub) ListPublicSiteFeeds(context.Context, pgtype.UUID) ([]dbgen.DirectorySiteFeed, error) {
	return stub.feeds, stub.feedsErr
}

func (stub queryStub) ListSiteResources(context.Context, pgtype.UUID) ([]dbgen.DirectorySiteResource, error) {
	return stub.resources, stub.resourcesErr
}

func (stub queryStub) ListPublicSiteTags(context.Context, pgtype.UUID) ([]dbgen.ListPublicSiteTagsRow, error) {
	return stub.tags, stub.tagsErr
}

func (stub queryStub) ListPublicSiteSoftwareComponents(
	context.Context,
	pgtype.UUID,
) ([]dbgen.ListPublicSiteSoftwareComponentsRow, error) {
	return stub.technologies, stub.technologiesErr
}
