package publicview

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type IdentifierKind uint8

const (
	IdentifierUUID IdentifierKind = iota + 1
	IdentifierShortID
)

type SiteIdentifier struct {
	Kind  IdentifierKind
	Value string
}

type Reader interface {
	AnnouncementReader
	Graph(context.Context) (FriendGraph, error)
	SiteGraphByIdentifier(context.Context, SiteIdentifier) (FriendGraph, error)
	Home(context.Context) (Home, error)
	Directory(context.Context, DirectoryQuery) (DirectoryView, error)
	DirectoryOptions(context.Context) (DirectoryOptions, error)
	RandomSite(context.Context, RandomSiteQuery) (RandomSiteView, error)
	SiteByIdentifier(context.Context, SiteIdentifier) (SiteProfile, error)
	SiteIconByIdentifier(context.Context, SiteIdentifier) (SiteIcon, error)
	SiteByCustomID(context.Context, string) (SiteProfile, error)
}

type CardTagQueries interface {
	ListPublicSiteTagsBySiteIDs(context.Context, []pgtype.UUID) ([]dbgen.ListPublicSiteTagsBySiteIDsRow, error)
}

type CardFeedQueries interface {
	ListDefaultPublicSiteFeedsBySiteIDs(context.Context, []pgtype.UUID) ([]dbgen.DirectorySiteFeed, error)
}

type CardSitemapQueries interface {
	ListPublicSitemapsBySiteIDs(context.Context, []pgtype.UUID) ([]dbgen.DirectorySiteResource, error)
}

type CardQueries interface {
	CardTagQueries
	CardFeedQueries
	CardSitemapQueries
}

type AnnouncementQueries interface {
	ListActiveMainAnnouncements(context.Context) ([]dbgen.ContentAnnouncement, error)
}

type HomeSiteQueries interface {
	CardQueries
	ListRandomVisibleSites(context.Context, int32) ([]dbgen.DirectorySite, error)
}

type HomeQueries interface {
	HomeSiteQueries
	AnnouncementQueries
	CountVisibleSites(context.Context) (int64, error)
}

type DirectoryQueries interface {
	CardQueries
	CountDirectorySitesByStatus(
		context.Context,
		dbgen.CountDirectorySitesByStatusParams,
	) (dbgen.CountDirectorySitesByStatusRow, error)
	ListDirectorySites(context.Context, dbgen.ListDirectorySitesParams) ([]dbgen.DirectorySite, error)
}

type ClassificationQueries interface {
	ListEnabledSiteTagCascades(context.Context) ([]dbgen.ListEnabledSiteTagCascadesRow, error)
}

type DirectoryOptionsQueries interface {
	ClassificationQueries
	ListDirectoryTagOptions(context.Context) ([]dbgen.ListDirectoryTagOptionsRow, error)
	ListDirectoryTechnologyOptions(context.Context) ([]dbgen.ListDirectoryTechnologyOptionsRow, error)
}

type RandomQueries interface {
	CardQueries
	ClassificationQueries
	PickRandomVisibleSite(context.Context, dbgen.PickRandomVisibleSiteParams) (dbgen.DirectorySite, error)
}

type IdentifierQueries interface {
	GetSiteByID(context.Context, pgtype.UUID) (dbgen.DirectorySite, error)
	GetSiteByShortID(context.Context, string) (dbgen.DirectorySite, error)
}

type LookupQueries interface {
	IdentifierQueries
	GetSiteByCustomID(context.Context, *string) (dbgen.DirectorySite, error)
}

type ProfileQueries interface {
	ListPublicSiteFeeds(context.Context, pgtype.UUID) ([]dbgen.DirectorySiteFeed, error)
	ListSiteResources(context.Context, pgtype.UUID) ([]dbgen.DirectorySiteResource, error)
	ListPublicSiteTags(context.Context, pgtype.UUID) ([]dbgen.ListPublicSiteTagsRow, error)
	ListPublicSiteSoftwareComponents(context.Context, pgtype.UUID) ([]dbgen.ListPublicSiteSoftwareComponentsRow, error)
	GetSiteIconHash(context.Context, pgtype.UUID) ([]byte, error)
}

type IconQueries interface {
	GetSiteIcon(context.Context, pgtype.UUID) (dbgen.DirectorySiteIcon, error)
}

type Queries interface {
	AnnouncementReadQueries
	GraphQueries
	HomeQueries
	DirectoryQueries
	DirectoryOptionsQueries
	RandomQueries
	LookupQueries
	ProfileQueries
	IconQueries
}

type GraphQueries interface {
	GetPublicFriendGraph(context.Context, pgtype.UUID) ([]byte, error)
}
