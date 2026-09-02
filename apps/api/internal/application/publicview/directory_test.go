package publicview

import (
	"context"
	"testing"
	"time"

	dbgen "heyblog-api/internal/database/gen"
)

func TestDefaultDirectoryQueryUsesChinaCalendarSeed(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 1, 16, 30, 0, 0, time.UTC)

	query := DefaultDirectoryQuery(now)

	if query.Page != 1 || query.Sort != DirectorySortRandom || query.Order != DirectoryOrderDescending {
		t.Fatalf("DefaultDirectoryQuery() = %#v", query)
	}
	if query.Seed != "site-directory:2026-09-02" {
		t.Fatalf("seed = %q, want China-calendar seed", query.Seed)
	}
}

func TestDirectoryClampsPageAndBuildsCurrentSiteCards(t *testing.T) {
	t.Parallel()

	row := testSite("A1b2C3d4E")
	feedRef := "/blog/feed.xml"
	service := New(queryStub{
		directoryCounts: dbgen.CountDirectorySitesByStatusRow{NormalCount: 25, AbnormalCount: 4},
		listDirectory: func(
			_ context.Context,
			parameters dbgen.ListDirectorySitesParams,
		) ([]dbgen.DirectorySite, error) {
			if parameters.PageOffset != 24 || parameters.PageLimit != directoryPageSize ||
				parameters.SiteVisibility != "VISIBLE" {
				t.Fatalf("list parameters = %#v", parameters)
			}
			return []dbgen.DirectorySite{row}, nil
		},
		batchTags: []dbgen.ListPublicSiteTagsBySiteIDsRow{
			{SiteID: row.ID, Role: "PRIMARY", Name: "技术", Slug: "technology"},
			{SiteID: row.ID, Role: "WARNING", Name: "访问较慢", Slug: "slow-access"},
		},
		batchFeeds: []dbgen.DirectorySiteFeed{{
			SiteID: row.ID, Name: "默认订阅", LocationType: "RELATIVE", UrlRef: &feedRef,
			Format: "ATOM", IsEnabled: true, IsDefault: true,
		}},
	})
	query := DefaultDirectoryQuery(time.Now())
	query.Page = 9

	view, err := service.Directory(context.Background(), query)

	if err != nil {
		t.Fatalf("Directory() error = %v", err)
	}
	if view.Pagination.Page != 2 || view.Pagination.TotalPages != 2 || len(view.Items) != 1 {
		t.Fatalf("Directory() = %#v", view)
	}
	card := view.Items[0]
	if card.DefaultFeed == nil || card.DefaultFeed.URL != "https://example.com/blog/feed.xml" {
		t.Fatalf("card default feed = %#v", card.DefaultFeed)
	}
	if len(card.Topics) != 1 || len(card.Warnings) != 1 {
		t.Fatalf("card tags = %#v / %#v", card.Topics, card.Warnings)
	}
}

func TestDirectoryOptionsSeparateTagRolesAndTechnologies(t *testing.T) {
	t.Parallel()

	service := New(queryStub{
		directoryTags: []dbgen.ListDirectoryTagOptionsRow{
			{Name: "技术", Slug: "technology", Role: "PRIMARY", NormalCount: 8, AbnormalCount: 1},
			{Name: "写作", Slug: "writing", Role: "SECONDARY", NormalCount: 5},
			{Name: "访问较慢", Slug: "slow-access", Role: "WARNING", NormalCount: 2},
		},
		directoryTechnologies: []dbgen.ListDirectoryTechnologyOptionsRow{{
			Name: "Astro", NormalizedName: "astro", NormalCount: 3, AbnormalCount: 1,
		}},
	})

	options, err := service.DirectoryOptions(context.Background())

	if err != nil {
		t.Fatalf("DirectoryOptions() error = %v", err)
	}
	if len(options.PrimaryTags) != 1 || len(options.SecondaryTags) != 1 ||
		len(options.Warnings) != 1 || len(options.Technologies) != 1 {
		t.Fatalf("DirectoryOptions() = %#v", options)
	}
	if options.Technologies[0].Value != "astro" || options.PrimaryTags[0].AbnormalCount != 1 {
		t.Fatalf("technology option = %#v", options.Technologies[0])
	}
}

func TestDirectoryQueryParametersKeepStableRandomState(t *testing.T) {
	t.Parallel()

	query := DirectoryQuery{
		Page:          3,
		Query:         "astro",
		PrimaryTags:   []string{"technology", "life"},
		SecondaryTags: []string{"writing", "design"},
		Warnings:      []string{"slow-access"},
		Technologies:  []string{"astro"},
		AccessScopes:  []string{"ALL"},
		Feed:          DirectoryFeedWith,
		Sort:          DirectorySortRandom,
		Order:         DirectoryOrderDescending,
		Seed:          "site-directory:shuffle:alpha",
	}

	parameters := query.databaseParameters(96, "VISIBLE")

	if parameters.Offset != 48 || parameters.Limit != directoryPageSize {
		t.Fatalf("pagination parameters = %#v", parameters)
	}
	if parameters.Seed != query.Seed || parameters.SortMode != string(DirectorySortRandom) {
		t.Fatalf("random parameters = %#v", parameters)
	}
}

func TestDirectoryUsesAbnormalCountAndHiddenVisibility(t *testing.T) {
	t.Parallel()

	service := New(queryStub{
		directoryCounts: dbgen.CountDirectorySitesByStatusRow{NormalCount: 8, AbnormalCount: 3},
		listDirectory: func(
			_ context.Context,
			parameters dbgen.ListDirectorySitesParams,
		) ([]dbgen.DirectorySite, error) {
			if parameters.SiteVisibility != "HIDDEN" {
				t.Fatalf("visibility = %q, want HIDDEN", parameters.SiteVisibility)
			}
			return []dbgen.DirectorySite{}, nil
		},
	})
	query := DefaultDirectoryQuery(time.Now())
	query.Status = DirectoryStatusAbnormal

	view, err := service.Directory(context.Background(), query)

	if err != nil {
		t.Fatalf("Directory() error = %v", err)
	}
	if view.Pagination.TotalItems != 3 || view.StatusCounts.Normal != 8 || view.StatusCounts.Abnormal != 3 {
		t.Fatalf("Directory() counts = %#v / %#v", view.Pagination, view.StatusCounts)
	}
}
