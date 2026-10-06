package publicview

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type SitemapKind string

const (
	SitemapSites         SitemapKind = "sites"
	SitemapAnnouncements SitemapKind = "announcements"
	sitemapPageSize                  = 1000
)

type SitemapQuery struct {
	Kind  SitemapKind
	After pgtype.UUID
}

type SitemapItem struct {
	ID          string     `json:"id" format:"uuid"`
	ShortID     string     `json:"shortId,omitempty"`
	StartsAt    *time.Time `json:"startsAt,omitempty"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
}

type SitemapPage struct {
	Items     []SitemapItem `json:"items"`
	NextAfter *string       `json:"nextAfter" format:"uuid"`
}

type SitemapQueries interface {
	ListSitemapSites(context.Context, pgtype.UUID) ([]dbgen.ListSitemapSitesRow, error)
	ListSitemapAnnouncements(context.Context, pgtype.UUID) ([]dbgen.ListSitemapAnnouncementsRow, error)
}

func (service *Service) Sitemap(ctx context.Context, query SitemapQuery) (SitemapPage, error) {
	page := SitemapPage{Items: []SitemapItem{}}
	switch query.Kind {
	case SitemapSites:
		rows, err := service.sitemap.ListSitemapSites(ctx, query.After)
		if err != nil {
			return SitemapPage{}, internalError(err, "list sitemap sites")
		}
		for index, row := range rows {
			if index == sitemapPageSize {
				page.NextAfter = &page.Items[index-1].ID
				break
			}
			page.Items = append(page.Items, SitemapItem{ID: row.ID.String(), ShortID: row.ShortID})
		}
	case SitemapAnnouncements:
		rows, err := service.sitemap.ListSitemapAnnouncements(ctx, query.After)
		if err != nil {
			return SitemapPage{}, internalError(err, "list sitemap announcements")
		}
		for index, row := range rows {
			if index == sitemapPageSize {
				page.NextAfter = &page.Items[index-1].ID
				break
			}
			if !row.StartsAt.Valid || !row.PublishedAt.Valid || !row.UpdatedAt.Valid {
				return SitemapPage{}, internalError(errors.New("announcement timestamps are invalid"), "map sitemap announcement")
			}
			page.Items = append(page.Items, SitemapItem{
				ID: row.ID.String(), StartsAt: &row.StartsAt.Time,
				PublishedAt: &row.PublishedAt.Time, UpdatedAt: &row.UpdatedAt.Time,
			})
		}
	default:
		return SitemapPage{}, internalError(errors.New("unsupported sitemap kind"), "list sitemap")
	}
	return page, nil
}
