package publicview

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"

	"heyblog-api/internal/domain/site"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type profileLookup struct {
	row dbgen.DirectorySite
	err error
}

func loadProfile(
	ctx context.Context,
	queries ProfileQueries,
	lookup profileLookup,
) (SiteProfile, error) {
	row, lookupErr := lookup.row, lookup.err
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
	feeds, err := queries.ListPublicSiteFeeds(ctx, row.ID)
	if err != nil {
		return SiteProfile{}, internalError(err, "list public site feeds")
	}
	resources, err := queries.ListSiteResources(ctx, row.ID)
	if err != nil {
		return SiteProfile{}, internalError(err, "list public site resources")
	}
	tags, err := queries.ListPublicSiteTags(ctx, row.ID)
	if err != nil {
		return SiteProfile{}, internalError(err, "list public site tags")
	}
	technologies, err := queries.ListPublicSiteSoftwareComponents(ctx, row.ID)
	if err != nil {
		return SiteProfile{}, internalError(err, "list public site technologies")
	}

	address := site.Address{
		Scheme: row.Scheme, NormalizedHost: row.NormalizedHost, BasePath: row.BasePath,
	}
	profile := SiteProfile{
		SiteCard:     card,
		IsIndexable:  row.Visibility == "VISIBLE",
		TertiaryTags: []Topic{},
		Warnings:     []Warning{},
		Feeds:        make([]Feed, 0, len(feeds)),
		Resources:    make([]Resource, 0, len(resources)),
		Technologies: make([]Technology, 0, len(technologies)),
	}
	hash, err := queries.GetSiteIconHash(ctx, row.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return SiteProfile{}, internalError(err, "load site icon hash")
	}
	if err == nil {
		encoded := hex.EncodeToString(hash)
		profile.IconHash = &encoded
	}
	for _, tag := range tags {
		topic := Topic{Name: tag.Name, Description: tag.Description}
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
				Name: tag.Name, Description: tag.Description,
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
