package publicview

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/domain/site"
)

const homeSiteLimit int32 = 6

type Home struct {
	SiteCount    int64          `json:"siteCount"`
	Announcement *Announcement  `json:"announcement"`
	Sites        []HomeSiteCard `json:"sites"`
}

type HomeSiteCard struct {
	SiteCard
	Topics      []HomeSiteTopic `json:"topics"`
	Warnings    []Warning       `json:"warnings"`
	DefaultFeed *HomeSiteFeed   `json:"defaultFeed"`
	SitemapURL  *string         `json:"sitemapUrl"`
}

type HomeSiteTopic struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
	Role string `json:"role"`
}

type HomeSiteFeed struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Format string `json:"format"`
}

func (service *Service) Home(ctx context.Context) (Home, error) {
	count, err := service.queries.CountVisibleSites(ctx)
	if err != nil {
		return Home{}, internalError(err, "count visible sites")
	}
	if count < 0 {
		return Home{}, internalError(
			errors.New("visible site count is out of range"),
			"validate visible site count",
		)
	}

	sites, err := service.loadRandomSites(ctx, count)
	if err != nil {
		return Home{}, err
	}
	announcement, err := service.loadAnnouncement(ctx)
	if err != nil {
		return Home{}, err
	}
	return Home{SiteCount: count, Announcement: announcement, Sites: sites}, nil
}

func (service *Service) loadRandomSites(ctx context.Context, count int64) ([]HomeSiteCard, error) {
	if count == 0 {
		return []HomeSiteCard{}, nil
	}
	rows, err := service.queries.ListRandomVisibleSites(ctx, homeSiteLimit)
	if err != nil {
		return nil, internalError(err, "list random visible sites")
	}
	if len(rows) > int(homeSiteLimit) {
		return nil, internalError(errors.New("random site query exceeded its limit"), "validate random visible sites")
	}

	cards := make([]HomeSiteCard, len(rows))
	siteIDs := make([]pgtype.UUID, len(rows))
	cardIndex := make(map[pgtype.UUID]int, len(rows))
	addresses := make(map[pgtype.UUID]site.Address, len(rows))
	for index, row := range rows {
		if !row.ID.Valid {
			return nil, internalError(errors.New("random site ID is invalid"), "map random visible site")
		}
		if _, exists := cardIndex[row.ID]; exists {
			return nil, internalError(errors.New("random site query returned a duplicate"), "map random visible site")
		}
		card, mapErr := mapSiteCard(row)
		if mapErr != nil {
			return nil, internalError(mapErr, "map random visible site")
		}
		cards[index] = HomeSiteCard{SiteCard: card, Topics: []HomeSiteTopic{}, Warnings: []Warning{}}
		siteIDs[index] = row.ID
		cardIndex[row.ID] = index
		addresses[row.ID] = site.Address{
			Scheme: row.Scheme, NormalizedHost: row.NormalizedHost, BasePath: row.BasePath,
		}
	}

	tags, err := service.queries.ListPublicSiteTagsBySiteIDs(ctx, siteIDs)
	if err != nil {
		return nil, internalError(err, "list random site tags")
	}
	for _, tag := range tags {
		index, exists := cardIndex[tag.SiteID]
		if !exists {
			return nil, internalError(errors.New("tag references an unexpected site"), "map random site tags")
		}
		if tag.Role == "WARNING" {
			cards[index].Warnings = append(cards[index].Warnings, Warning{
				Name: tag.Name, Slug: tag.Slug, Description: tag.Description,
			})
			continue
		}
		cards[index].Topics = append(cards[index].Topics, HomeSiteTopic{
			Name: tag.Name, Slug: tag.Slug, Role: tag.Role,
		})
	}

	feeds, err := service.queries.ListDefaultPublicSiteFeedsBySiteIDs(ctx, siteIDs)
	if err != nil {
		return nil, internalError(err, "list random site feeds")
	}
	for _, feed := range feeds {
		index, exists := cardIndex[feed.SiteID]
		address, addressExists := addresses[feed.SiteID]
		if !exists || !addressExists {
			return nil, internalError(errors.New("feed references an unexpected site"), "map random site feeds")
		}
		locationURL, mapErr := address.LocationURL(
			locationFromValues(feed.LocationType, feed.UrlRef, feed.ExternalUrl),
		)
		if mapErr != nil {
			return nil, internalError(mapErr, "map random site feed")
		}
		cards[index].DefaultFeed = &HomeSiteFeed{Name: feed.Name, URL: locationURL, Format: feed.Format}
	}

	sitemaps, err := service.queries.ListPublicSitemapsBySiteIDs(ctx, siteIDs)
	if err != nil {
		return nil, internalError(err, "list random site sitemaps")
	}
	for _, sitemap := range sitemaps {
		index, exists := cardIndex[sitemap.SiteID]
		address, addressExists := addresses[sitemap.SiteID]
		if !exists || !addressExists {
			return nil, internalError(errors.New("sitemap references an unexpected site"), "map random site sitemaps")
		}
		locationURL, mapErr := address.LocationURL(
			locationFromValues(sitemap.LocationType, sitemap.UrlRef, sitemap.ExternalUrl),
		)
		if mapErr != nil {
			return nil, internalError(mapErr, "map random site sitemap")
		}
		cards[index].SitemapURL = &locationURL
	}
	return cards, nil
}
