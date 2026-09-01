package publicview

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/apperror"
	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/domain/content"
)

func TestHomeLoadsFreshRandomCardsWithPublicResources(t *testing.T) {
	t.Parallel()

	first := testSite("A1b2C3d4E")
	second := testSite("B1b2C3d4E")
	feedRef := "/blog/feed.xml"
	sitemapRef := "/blog/sitemap.xml"
	randomCalls := 0
	queries := queryStub{
		count: 8,
		listRandom: func(_ context.Context, limit int32) ([]dbgen.DirectorySite, error) {
			randomCalls++
			if limit != homeSiteLimit {
				t.Fatalf("ListRandomVisibleSites() limit = %d", limit)
			}
			return []dbgen.DirectorySite{first, second}, nil
		},
		announcementErr: pgx.ErrNoRows,
		batchTags: []dbgen.ListPublicSiteTagsBySiteIDsRow{
			{SiteID: first.ID, Role: "PRIMARY", Name: "技术", Slug: "technology"},
			{SiteID: first.ID, Role: "SECONDARY", Name: "生活", Slug: "life"},
			{SiteID: first.ID, Role: "WARNING", Name: "访问提示", Slug: "access-notice", Description: "部分地区可能较慢"},
		},
		batchFeeds: []dbgen.DirectorySiteFeed{{
			SiteID: first.ID, Name: "主订阅", LocationType: "RELATIVE", UrlRef: &feedRef, Format: "ATOM", IsEnabled: true, IsDefault: true,
		}},
		batchSitemaps: []dbgen.DirectorySiteResource{{
			SiteID: first.ID, Kind: "SITEMAP", LocationType: "RELATIVE", UrlRef: &sitemapRef,
		}},
	}
	service := New(queries)

	view, err := service.Home(context.Background())
	if err != nil {
		t.Fatalf("Home() error = %v", err)
	}
	if _, err := service.Home(context.Background()); err != nil {
		t.Fatalf("Home() second error = %v", err)
	}

	if view.SiteCount != 8 || len(view.Sites) != 2 || view.Announcement != nil {
		t.Fatalf("Home() = %#v", view)
	}
	if randomCalls != 2 {
		t.Fatalf("ListRandomVisibleSites() calls = %d, want 2", randomCalls)
	}
	card := view.Sites[0]
	if card.HomepageURL != "https://example.com/blog" || len(card.Topics) != 2 || len(card.Warnings) != 1 {
		t.Fatalf("card profile = %#v", card)
	}
	if card.UpdatedAt != first.UpdatedAt.Time {
		t.Fatalf("card updated at = %v, want %v", card.UpdatedAt, first.UpdatedAt.Time)
	}
	if card.DefaultFeed == nil || card.DefaultFeed.URL != "https://example.com/blog/feed.xml" || card.DefaultFeed.Format != "ATOM" {
		t.Fatalf("card default feed = %#v", card.DefaultFeed)
	}
	if card.SitemapURL == nil || *card.SitemapURL != "https://example.com/blog/sitemap.xml" {
		t.Fatalf("card sitemap = %#v", card.SitemapURL)
	}
}

func TestHomeRejectsInvalidRandomSiteUpdatedAt(t *testing.T) {
	t.Parallel()

	row := testSite("A1b2C3d4E")
	row.UpdatedAt = pgtype.Timestamptz{}
	service := New(queryStub{
		count: 1,
		listRandom: func(context.Context, int32) ([]dbgen.DirectorySite, error) {
			return []dbgen.DirectorySite{row}, nil
		},
		announcementErr: pgx.ErrNoRows,
	})

	_, err := service.Home(context.Background())
	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) || applicationError.Kind() != apperror.KindInternal {
		t.Fatalf("Home() error = %v, want internal application error", err)
	}
}

func TestHomeReturnsEmptySitesWithoutAssociationQueries(t *testing.T) {
	t.Parallel()

	service := New(queryStub{count: 0, announcementErr: pgx.ErrNoRows})
	view, err := service.Home(context.Background())
	if err != nil {
		t.Fatalf("Home() error = %v", err)
	}
	if view.Sites == nil || len(view.Sites) != 0 {
		t.Fatalf("Home().Sites = %#v, want non-nil empty slice", view.Sites)
	}
}

func TestHomeRejectsNegativeDirectoryCount(t *testing.T) {
	t.Parallel()

	listCalled := false
	service := New(queryStub{
		count: -1,
		listRandom: func(context.Context, int32) ([]dbgen.DirectorySite, error) {
			listCalled = true
			return nil, nil
		},
	})

	if _, err := service.Home(context.Background()); err == nil {
		t.Fatal("Home() error = nil, want out-of-range error")
	}
	if listCalled {
		t.Fatal("ListRandomVisibleSites() called for a negative count")
	}
}

func TestHomeMapsRandomCardQueryFailuresToInternalErrors(t *testing.T) {
	t.Parallel()

	row := testSite("A1b2C3d4E")
	queryFailure := errors.New("query failed")
	tests := []struct {
		name      string
		configure func(*queryStub)
	}{
		{
			name: "random sites",
			configure: func(stub *queryStub) {
				stub.listRandom = func(context.Context, int32) ([]dbgen.DirectorySite, error) {
					return nil, queryFailure
				}
			},
		},
		{name: "tags", configure: func(stub *queryStub) { stub.batchTagsErr = queryFailure }},
		{name: "feeds", configure: func(stub *queryStub) { stub.batchFeedsErr = queryFailure }},
		{name: "sitemaps", configure: func(stub *queryStub) { stub.batchSitemapErr = queryFailure }},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			stub := queryStub{
				count: 1,
				listRandom: func(context.Context, int32) ([]dbgen.DirectorySite, error) {
					return []dbgen.DirectorySite{row}, nil
				},
				announcementErr: pgx.ErrNoRows,
			}
			testCase.configure(&stub)
			_, err := New(stub).Home(context.Background())
			var applicationError *apperror.Error
			if !errors.As(err, &applicationError) || applicationError.Kind() != apperror.KindInternal {
				t.Fatalf("Home() error = %v, want internal application error", err)
			}
		})
	}
}

func TestHomeMapsLeadingAnnouncementActions(t *testing.T) {
	t.Parallel()

	label := "查看说明"
	path := "/about"
	externalURL := "https://example.com/announcements/current"
	tests := []struct {
		name       string
		actionType content.ActionType
		label      *string
		path       *string
		external   *string
		wantAction *AnnouncementAction
	}{
		{name: "none", actionType: content.ActionNone},
		{
			name: "internal", actionType: content.ActionInternal, label: &label, path: &path,
			wantAction: &AnnouncementAction{Label: label, Href: path},
		},
		{
			name: "external", actionType: content.ActionExternal, label: &label, external: &externalURL,
			wantAction: &AnnouncementAction{Label: label, Href: externalURL, External: true},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			service := New(queryStub{
				count: 0,
				announcement: dbgen.ContentAnnouncement{
					Kind:              string(content.KindMain),
					Title:             "目录维护公告",
					Status:            string(content.StatusPublished),
					ActionType:        string(testCase.actionType),
					ActionLabel:       testCase.label,
					ActionPath:        testCase.path,
					ActionExternalUrl: testCase.external,
					StartsAt:          timestamp(time.Date(2026, time.August, 11, 0, 0, 0, 0, time.UTC)),
				},
			})

			view, err := service.Home(context.Background())
			if err != nil {
				t.Fatalf("Home() error = %v", err)
			}
			if view.Announcement == nil {
				t.Fatal("Home().Announcement = nil")
			}
			if testCase.wantAction == nil && view.Announcement.Action != nil {
				t.Fatalf("announcement action = %#v, want nil", view.Announcement.Action)
			}
			if testCase.wantAction != nil && (view.Announcement.Action == nil || *view.Announcement.Action != *testCase.wantAction) {
				t.Fatalf("announcement action = %#v, want %#v", view.Announcement.Action, testCase.wantAction)
			}
		})
	}
}

func TestHomeRejectsMalformedAnnouncementPersistenceData(t *testing.T) {
	t.Parallel()

	startsAt := timestamp(time.Date(2026, time.August, 11, 0, 0, 0, 0, time.UTC))
	label := "查看说明"
	tests := []struct {
		name         string
		announcement dbgen.ContentAnnouncement
	}{
		{
			name: "unknown kind",
			announcement: dbgen.ContentAnnouncement{
				Kind: "MODAL", Status: string(content.StatusPublished), ActionType: string(content.ActionNone), StartsAt: startsAt,
			},
		},
		{
			name: "wrong kind",
			announcement: dbgen.ContentAnnouncement{
				Kind: string(content.KindBanner), Status: string(content.StatusPublished), ActionType: string(content.ActionNone), StartsAt: startsAt,
			},
		},
		{
			name: "unknown status",
			announcement: dbgen.ContentAnnouncement{
				Kind: string(content.KindMain), Status: "ACTIVE", ActionType: string(content.ActionNone), StartsAt: startsAt,
			},
		},
		{
			name: "wrong status",
			announcement: dbgen.ContentAnnouncement{
				Kind: string(content.KindMain), Status: string(content.StatusDraft), ActionType: string(content.ActionNone), StartsAt: startsAt,
			},
		},
		{
			name: "invalid action shape",
			announcement: dbgen.ContentAnnouncement{
				Kind: string(content.KindMain), Status: string(content.StatusPublished), ActionType: string(content.ActionInternal),
				ActionLabel: &label, StartsAt: startsAt,
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			service := New(queryStub{count: 0, announcement: testCase.announcement})
			_, err := service.Home(context.Background())
			var applicationError *apperror.Error
			if !errors.As(err, &applicationError) || applicationError.Kind() != apperror.KindInternal {
				t.Fatalf("Home() error = %v, want internal application error", err)
			}
		})
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
	id := [16]byte{1}
	copy(id[1:], shortID)
	return dbgen.DirectorySite{
		ID:             pgtype.UUID{Bytes: id, Valid: true},
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
	listRandom      func(context.Context, int32) ([]dbgen.DirectorySite, error)
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
	batchFeeds      []dbgen.DirectorySiteFeed
	batchFeedsErr   error
	resources       []dbgen.DirectorySiteResource
	resourcesErr    error
	batchSitemaps   []dbgen.DirectorySiteResource
	batchSitemapErr error
	tags            []dbgen.ListPublicSiteTagsRow
	tagsErr         error
	batchTags       []dbgen.ListPublicSiteTagsBySiteIDsRow
	batchTagsErr    error
	technologies    []dbgen.ListPublicSiteSoftwareComponentsRow
	technologiesErr error
}

func (stub queryStub) CountVisibleSites(context.Context) (int64, error) {
	return stub.count, stub.countErr
}

func (stub queryStub) ListRandomVisibleSites(ctx context.Context, limit int32) ([]dbgen.DirectorySite, error) {
	if stub.listRandom != nil {
		return stub.listRandom(ctx, limit)
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

func (stub queryStub) ListDefaultPublicSiteFeedsBySiteIDs(context.Context, []pgtype.UUID) ([]dbgen.DirectorySiteFeed, error) {
	return stub.batchFeeds, stub.batchFeedsErr
}

func (stub queryStub) ListSiteResources(context.Context, pgtype.UUID) ([]dbgen.DirectorySiteResource, error) {
	return stub.resources, stub.resourcesErr
}

func (stub queryStub) ListPublicSitemapsBySiteIDs(context.Context, []pgtype.UUID) ([]dbgen.DirectorySiteResource, error) {
	return stub.batchSitemaps, stub.batchSitemapErr
}

func (stub queryStub) ListPublicSiteTags(context.Context, pgtype.UUID) ([]dbgen.ListPublicSiteTagsRow, error) {
	return stub.tags, stub.tagsErr
}

func (stub queryStub) ListPublicSiteTagsBySiteIDs(context.Context, []pgtype.UUID) ([]dbgen.ListPublicSiteTagsBySiteIDsRow, error) {
	return stub.batchTags, stub.batchTagsErr
}

func (stub queryStub) ListPublicSiteSoftwareComponents(
	context.Context,
	pgtype.UUID,
) ([]dbgen.ListPublicSiteSoftwareComponentsRow, error) {
	return stub.technologies, stub.technologiesErr
}
