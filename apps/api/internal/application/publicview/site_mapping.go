package publicview

import (
	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/domain/site"
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
	return SiteCard{
		ShortID: row.ShortID, CustomID: row.CustomID, Name: row.Name, Summary: row.Summary,
		Host: row.NormalizedHost, HomepageURL: homepageURL, AccessScope: row.AccessScope,
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
