package publicview

import (
	"context"
	"errors"
)

const homeSiteLimit int32 = 6

type Home struct {
	SiteCount    int64          `json:"siteCount"`
	Announcement *Announcement  `json:"announcement"`
	Sites        []SiteCardView `json:"sites"`
}

type SiteCardView struct {
	SiteCard
	Topics      []HomeSiteTopic `json:"topics"`
	Warnings    []Warning       `json:"warnings"`
	DefaultFeed *HomeSiteFeed   `json:"defaultFeed"`
	SitemapURL  *string         `json:"sitemapUrl"`
}

type HomeSiteCard = SiteCardView

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

	return service.loadSiteCards(ctx, rows)
}
