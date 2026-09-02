package publicview

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/domain/site"
)

type siteCardBatch struct {
	cards     []HomeSiteCard
	siteIDs   []pgtype.UUID
	cardIndex map[pgtype.UUID]int
	addresses map[pgtype.UUID]site.Address
}

func (service *Service) loadSiteCards(
	ctx context.Context,
	rows []dbgen.DirectorySite,
) ([]HomeSiteCard, error) {
	cards := make([]HomeSiteCard, len(rows))
	siteIDs := make([]pgtype.UUID, len(rows))
	cardIndex := make(map[pgtype.UUID]int, len(rows))
	addresses := make(map[pgtype.UUID]site.Address, len(rows))
	for index, row := range rows {
		if !row.ID.Valid {
			return nil, internalError(errors.New("site ID is invalid"), "map site card")
		}
		if _, exists := cardIndex[row.ID]; exists {
			return nil, internalError(errors.New("site query returned a duplicate"), "map site card")
		}
		card, err := mapSiteCard(row)
		if err != nil {
			return nil, internalError(err, "map site card")
		}
		cards[index] = HomeSiteCard{
			SiteCard: card,
			Topics:   []HomeSiteTopic{},
			Warnings: []Warning{},
		}
		siteIDs[index] = row.ID
		cardIndex[row.ID] = index
		addresses[row.ID] = site.Address{
			Scheme: row.Scheme, NormalizedHost: row.NormalizedHost, BasePath: row.BasePath,
		}
	}

	batch := siteCardBatch{
		cards: cards, siteIDs: siteIDs, cardIndex: cardIndex, addresses: addresses,
	}
	if err := service.attachSiteCardTags(ctx, batch); err != nil {
		return nil, err
	}
	if err := service.attachSiteCardFeeds(ctx, batch); err != nil {
		return nil, err
	}
	if err := service.attachSiteCardSitemaps(ctx, batch); err != nil {
		return nil, err
	}
	return cards, nil
}

func (service *Service) attachSiteCardTags(
	ctx context.Context,
	batch siteCardBatch,
) error {
	tags, err := service.queries.ListPublicSiteTagsBySiteIDs(ctx, batch.siteIDs)
	if err != nil {
		return internalError(err, "list site card tags")
	}
	for _, tag := range tags {
		index, exists := batch.cardIndex[tag.SiteID]
		if !exists {
			return internalError(errors.New("tag references an unexpected site"), "map site card tags")
		}
		if tag.Role == "WARNING" {
			batch.cards[index].Warnings = append(batch.cards[index].Warnings, Warning{
				Name: tag.Name, Slug: tag.Slug, Description: tag.Description,
			})
			continue
		}
		batch.cards[index].Topics = append(batch.cards[index].Topics, HomeSiteTopic{
			Name: tag.Name, Slug: tag.Slug, Role: tag.Role,
		})
	}
	return nil
}

func (service *Service) attachSiteCardFeeds(
	ctx context.Context,
	batch siteCardBatch,
) error {
	feeds, err := service.queries.ListDefaultPublicSiteFeedsBySiteIDs(ctx, batch.siteIDs)
	if err != nil {
		return internalError(err, "list site card feeds")
	}
	for _, feed := range feeds {
		index, exists := batch.cardIndex[feed.SiteID]
		address, addressExists := batch.addresses[feed.SiteID]
		if !exists || !addressExists {
			return internalError(errors.New("feed references an unexpected site"), "map site card feeds")
		}
		locationURL, locationErr := address.LocationURL(
			locationFromValues(feed.LocationType, feed.UrlRef, feed.ExternalUrl),
		)
		if locationErr != nil {
			return internalError(locationErr, "map site card feed")
		}
		batch.cards[index].DefaultFeed = &HomeSiteFeed{
			Name: feed.Name, URL: locationURL, Format: feed.Format,
		}
	}
	return nil
}

func (service *Service) attachSiteCardSitemaps(
	ctx context.Context,
	batch siteCardBatch,
) error {
	sitemaps, err := service.queries.ListPublicSitemapsBySiteIDs(ctx, batch.siteIDs)
	if err != nil {
		return internalError(err, "list site card sitemaps")
	}
	for _, sitemap := range sitemaps {
		index, exists := batch.cardIndex[sitemap.SiteID]
		address, addressExists := batch.addresses[sitemap.SiteID]
		if !exists || !addressExists {
			return internalError(
				errors.New("sitemap references an unexpected site"),
				"map site card sitemaps",
			)
		}
		locationURL, locationErr := address.LocationURL(
			locationFromValues(sitemap.LocationType, sitemap.UrlRef, sitemap.ExternalUrl),
		)
		if locationErr != nil {
			return internalError(locationErr, "map site card sitemap")
		}
		batch.cards[index].SitemapURL = &locationURL
	}
	return nil
}
