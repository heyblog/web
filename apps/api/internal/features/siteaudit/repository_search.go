package siteaudit

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"heyblog-api/internal/domain/site"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func (repository *Repository) SearchSites(ctx context.Context, query string) ([]SiteSearchResult, error) {
	rows, err := repository.queries.SearchSitesForSubmission(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search sites for submission: %w", err)
	}
	results := make([]SiteSearchResult, 0, len(rows))
	for _, row := range rows {
		result, mapErr := siteSearchResult(row)
		if mapErr != nil {
			return nil, mapErr
		}
		results = append(results, result)
	}
	return results, nil
}

func (repository *Repository) ExistingSiteForHost(ctx context.Context, normalizedHost string) (*SiteSearchResult, error) {
	row, err := repository.queries.GetSiteByHost(ctx, normalizedHost)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find site by normalized host: %w", err)
	}
	result, err := siteSearchResult(row)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func siteSearchResult(row dbgen.DirectorySite) (SiteSearchResult, error) {
	urlValue, err := (site.Address{Scheme: row.Scheme, NormalizedHost: row.NormalizedHost, BasePath: row.BasePath}).HomepageURL()
	if err != nil {
		return SiteSearchResult{}, fmt.Errorf("map searched site address: %w", err)
	}
	return SiteSearchResult{ShortID: row.ShortID, Name: row.Name, URL: urlValue, Visibility: row.Visibility}, nil
}
