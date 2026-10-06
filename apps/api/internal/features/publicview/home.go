package publicview

import (
	"context"
	"errors"
)

const homeSiteLimit int32 = 6

type Home struct {
	SiteCount     int64          `json:"siteCount"`
	Announcements []Announcement `json:"announcements"`
	Sites         []SiteCardView `json:"sites"`
}

type SiteCardView struct {
	SiteCard
	Classification *SiteClassification `json:"classification"`
	TertiaryTags   []HomeSiteTopic     `json:"tertiaryTags"`
	Warnings       []Warning           `json:"warnings"`
	DefaultFeed    *HomeSiteFeed       `json:"defaultFeed"`
	SitemapURL     *string             `json:"sitemapUrl"`
}

type SiteClassification struct {
	Level1 HomeSiteTopic `json:"level1"`
	Level2 HomeSiteTopic `json:"level2"`
}

type HomeSiteCard = SiteCardView

type HomeSiteTopic struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type HomeSiteFeed struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Format string `json:"format"`
}

func (service *Service) Home(ctx context.Context) (Home, error) {
	count, err := service.home.CountVisibleSites(ctx)
	if err != nil {
		return Home{}, internalError(err, "count visible sites")
	}
	if count < 0 {
		return Home{}, internalError(
			errors.New("visible site count is out of range"),
			"validate visible site count",
		)
	}

	sites, err := loadRandomSites(ctx, service.home, count, service.metrics)
	if err != nil {
		return Home{}, err
	}
	announcements, err := loadAnnouncements(ctx, service.home)
	if err != nil {
		return Home{}, err
	}
	service.attachMetrics(ctx, sites)
	return Home{SiteCount: count, Announcements: announcements, Sites: sites}, nil
}

func loadRandomSites(ctx context.Context, queries HomeSiteQueries, count int64, recorder MetricsRecorder) ([]HomeSiteCard, error) {
	if count == 0 {
		return []HomeSiteCard{}, nil
	}
	rows, err := queries.ListRandomVisibleSites(ctx, homeSiteLimit)
	if err != nil {
		return nil, internalError(err, "list random visible sites")
	}
	if len(rows) > int(homeSiteLimit) {
		return nil, internalError(errors.New("random site query exceeded its limit"), "validate random visible sites")
	}

	return loadSiteCards(ctx, queries, rows, recorder)
}
