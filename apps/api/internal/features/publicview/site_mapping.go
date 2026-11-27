package publicview

import (
	"heyblog-api/internal/domain/site"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"time"
)

func mapSiteCard(row dbgen.DirectorySite) (SiteCard, error) {
	directoryStatus, err := directoryStatusFromVisibility(row.Visibility)
	if err != nil {
		return SiteCard{}, err
	}
	if !row.JoinedAt.Valid {
		return SiteCard{}, invalidSiteTimestamp("join")
	}
	if !row.UpdatedAt.Valid {
		return SiteCard{}, invalidSiteTimestamp("update")
	}
	homepageURL, err := (site.Address{
		Scheme: row.Scheme, NormalizedHost: row.NormalizedHost, BasePath: row.BasePath,
	}).HomepageURL()
	if err != nil {
		return SiteCard{}, err
	}
	var checkedAt *time.Time
	if row.LastCheckedAt.Valid {
		checkedAt = &row.LastCheckedAt.Time
	}
	return SiteCard{
		ShortID: row.ShortID, CustomID: row.CustomID, Name: row.Name, Summary: row.Summary,
		Host: row.NormalizedHost, HomepageURL: homepageURL, AccessScope: row.AccessScope, HealthStatus: row.Status, LastCheckedAt: checkedAt,
		DirectoryStatus: directoryStatus, JoinedAt: row.JoinedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func locationFromValues(locationType string, urlRef, externalURL *string) site.Location {
	location := site.Location{Type: locationType}
	if urlRef != nil {
		location.URLRef = *urlRef
	}
	if externalURL != nil {
		location.ExternalURL = *externalURL
	}
	return location
}
