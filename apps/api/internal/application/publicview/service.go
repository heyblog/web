package publicview

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/apperror"
	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/domain/site"
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
	Home(context.Context) (Home, error)
	Directory(context.Context, DirectoryQuery) (DirectoryView, error)
	DirectoryOptions(context.Context) (DirectoryOptions, error)
	SiteByIdentifier(context.Context, SiteIdentifier) (SiteProfile, error)
	SiteIconByIdentifier(context.Context, SiteIdentifier) (SiteIcon, error)
	SiteByCustomID(context.Context, string) (SiteProfile, error)
}

type Queries interface {
	GetSiteIcon(context.Context, pgtype.UUID) (dbgen.DirectorySiteIcon, error)
	GetSiteIconHash(context.Context, pgtype.UUID) ([]byte, error)
	CountDirectorySitesByStatus(
		context.Context,
		dbgen.CountDirectorySitesByStatusParams,
	) (dbgen.CountDirectorySitesByStatusRow, error)
	CountVisibleSites(context.Context) (int64, error)
	GetLeadingActiveMainAnnouncement(context.Context) (dbgen.ContentAnnouncement, error)
	GetSiteByCustomID(context.Context, *string) (dbgen.DirectorySite, error)
	GetSiteByID(context.Context, pgtype.UUID) (dbgen.DirectorySite, error)
	GetSiteByShortID(context.Context, string) (dbgen.DirectorySite, error)
	ListDefaultPublicSiteFeedsBySiteIDs(context.Context, []pgtype.UUID) ([]dbgen.DirectorySiteFeed, error)
	ListDirectorySites(context.Context, dbgen.ListDirectorySitesParams) ([]dbgen.DirectorySite, error)
	ListDirectoryTagOptions(context.Context) ([]dbgen.ListDirectoryTagOptionsRow, error)
	ListEnabledSiteTagCascades(context.Context) ([]dbgen.ListEnabledSiteTagCascadesRow, error)
	ListDirectoryTechnologyOptions(context.Context) ([]dbgen.ListDirectoryTechnologyOptionsRow, error)
	ListPublicSiteFeeds(context.Context, pgtype.UUID) ([]dbgen.DirectorySiteFeed, error)
	ListPublicSiteSoftwareComponents(context.Context, pgtype.UUID) ([]dbgen.ListPublicSiteSoftwareComponentsRow, error)
	ListPublicSiteTags(context.Context, pgtype.UUID) ([]dbgen.ListPublicSiteTagsRow, error)
	ListPublicSiteTagsBySiteIDs(context.Context, []pgtype.UUID) ([]dbgen.ListPublicSiteTagsBySiteIDsRow, error)
	ListPublicSitemapsBySiteIDs(context.Context, []pgtype.UUID) ([]dbgen.DirectorySiteResource, error)
	ListRandomVisibleSites(context.Context, int32) ([]dbgen.DirectorySite, error)
	ListSiteResources(context.Context, pgtype.UUID) ([]dbgen.DirectorySiteResource, error)
}

type Service struct {
	queries Queries
}

func New(queries Queries) *Service {
	return &Service{queries: queries}
}

func (service *Service) SiteByIdentifier(
	ctx context.Context,
	identifier SiteIdentifier,
) (SiteProfile, error) {
	row, err := service.siteRowByIdentifier(ctx, identifier)
	var applicationError *apperror.Error
	if errors.As(err, &applicationError) {
		return SiteProfile{}, err
	}
	return service.loadProfile(ctx, row, err)
}

func (service *Service) siteRowByIdentifier(ctx context.Context, identifier SiteIdentifier) (dbgen.DirectorySite, error) {
	var (
		row dbgen.DirectorySite
		err error
	)
	switch identifier.Kind {
	case IdentifierUUID:
		var id pgtype.UUID
		if scanErr := id.Scan(identifier.Value); scanErr != nil || !id.Valid {
			return dbgen.DirectorySite{}, badIdentifier("identifier")
		}
		row, err = service.queries.GetSiteByID(ctx, id)
	case IdentifierShortID:
		if validateErr := site.ValidateShortID(identifier.Value); validateErr != nil {
			return dbgen.DirectorySite{}, badIdentifier("identifier")
		}
		row, err = service.queries.GetSiteByShortID(ctx, identifier.Value)
	default:
		return dbgen.DirectorySite{}, badIdentifier("identifier")
	}
	return row, err
}

func (service *Service) SiteByCustomID(ctx context.Context, customID string) (SiteProfile, error) {
	if err := site.ValidateCustomID(customID); err != nil {
		return SiteProfile{}, badIdentifier("customId")
	}
	row, err := service.queries.GetSiteByCustomID(ctx, &customID)
	return service.loadProfile(ctx, row, err)
}

func (service *Service) loadProfile(
	ctx context.Context,
	row dbgen.DirectorySite,
	lookupErr error,
) (SiteProfile, error) {
	if errors.Is(lookupErr, pgx.ErrNoRows) || lookupErr == nil && row.Visibility == "REMOVED" {
		return SiteProfile{}, notFound()
	}
	if lookupErr != nil {
		return SiteProfile{}, internalError(lookupErr, "load site profile")
	}

	card, err := mapSiteCard(row)
	if err != nil {
		return SiteProfile{}, internalError(err, "map site profile")
	}
	feeds, err := service.queries.ListPublicSiteFeeds(ctx, row.ID)
	if err != nil {
		return SiteProfile{}, internalError(err, "list public site feeds")
	}
	resources, err := service.queries.ListSiteResources(ctx, row.ID)
	if err != nil {
		return SiteProfile{}, internalError(err, "list public site resources")
	}
	tags, err := service.queries.ListPublicSiteTags(ctx, row.ID)
	if err != nil {
		return SiteProfile{}, internalError(err, "list public site tags")
	}
	technologies, err := service.queries.ListPublicSiteSoftwareComponents(ctx, row.ID)
	if err != nil {
		return SiteProfile{}, internalError(err, "list public site technologies")
	}

	address := site.Address{
		Scheme: row.Scheme, NormalizedHost: row.NormalizedHost, BasePath: row.BasePath,
	}
	profile := SiteProfile{
		SiteCard:     card,
		TertiaryTags: []Topic{},
		Warnings:     []Warning{},
		Feeds:        make([]Feed, 0, len(feeds)),
		Resources:    make([]Resource, 0, len(resources)),
		Technologies: make([]Technology, 0, len(technologies)),
	}
	hash, err := service.queries.GetSiteIconHash(ctx, row.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return SiteProfile{}, internalError(err, "load site icon hash")
	}
	if err == nil {
		encoded := hex.EncodeToString(hash)
		profile.IconHash = &encoded
	}
	for _, tag := range tags {
		topic := Topic{Name: tag.Name, Slug: tag.Slug, Description: tag.Description}
		switch tag.Role {
		case "PRIMARY":
			if profile.Classification == nil {
				profile.Classification = &SiteProfileClassification{}
			}
			profile.Classification.Level1 = topic
		case "SECONDARY":
			if profile.Classification == nil {
				profile.Classification = &SiteProfileClassification{}
			}
			profile.Classification.Level2 = topic
		case "TERTIARY":
			profile.TertiaryTags = append(profile.TertiaryTags, topic)
		case "WARNING":
			profile.Warnings = append(profile.Warnings, Warning{
				Name: tag.Name, Slug: tag.Slug, Description: tag.Description,
			})
		default:
			return SiteProfile{}, internalError(errors.New("tag has unsupported role"), "map public site tags")
		}
	}
	for _, feed := range feeds {
		locationURL, mapErr := address.LocationURL(
			locationFromValues(feed.LocationType, feed.UrlRef, feed.ExternalUrl),
		)
		if mapErr != nil {
			return SiteProfile{}, internalError(mapErr, "map public site feed")
		}
		profile.Feeds = append(profile.Feeds, Feed{
			Name: feed.Name, URL: locationURL, Format: feed.Format, IsDefault: feed.IsDefault,
		})
	}
	for _, resource := range resources {
		locationURL, mapErr := address.LocationURL(
			locationFromValues(resource.LocationType, resource.UrlRef, resource.ExternalUrl),
		)
		if mapErr != nil {
			return SiteProfile{}, internalError(mapErr, "map public site resource")
		}
		profile.Resources = append(profile.Resources, Resource{Kind: resource.Kind, URL: locationURL})
	}
	for _, technology := range technologies {
		profile.Technologies = append(profile.Technologies, Technology{
			Name:          technology.Name,
			Role:          technology.Role,
			HomepageURL:   technology.HomepageUrl,
			RepositoryURL: technology.RepositoryUrl,
			IsOpenSource:  technology.IsOpenSource,
		})
	}
	return profile, nil
}

var _ Reader = (*Service)(nil)
