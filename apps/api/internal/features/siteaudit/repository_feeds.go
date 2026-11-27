package siteaudit

import (
	"context"
	"fmt"

	"heyblog-api/internal/domain/site"
	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5/pgtype"
)

// Retain source identities so unrelated reviews do not delete fetched articles,
// conditional validators or queued work. A changed origin changes relative feeds.
func syncFeeds(ctx context.Context, queries *dbgen.Queries, siteID pgtype.UUID, current, snapshot Snapshot) error {
	address := site.Address{Scheme: snapshot.Scheme, NormalizedHost: snapshot.NormalizedHost, BasePath: snapshot.BasePath}
	feeds := make([]dbgen.UpsertSiteFeedParams, 0, len(snapshot.Feeds))
	retained := make(map[string]bool, len(snapshot.Feeds))
	for _, feed := range snapshot.Feeds {
		location, err := site.NormalizeLocation(feed.URL, address, false)
		if err != nil {
			return err
		}
		retained[location.URLKey] = true
		feeds = append(feeds, dbgen.UpsertSiteFeedParams{SiteID: siteID, Name: feed.Name, LocationType: location.Type, UrlRef: stringPointer(location.URLRef), ExternalUrl: stringPointer(location.ExternalURL), UrlKey: location.URLKey, Format: feed.Format, IsEnabled: true, IsDefault: feed.IsDefault})
	}
	existing, err := queries.ListSiteFeeds(ctx, siteID)
	if err != nil {
		return fmt.Errorf("read reviewed feeds: %w", err)
	}
	originChanged := current.Scheme != snapshot.Scheme || current.NormalizedHost != snapshot.NormalizedHost
	for _, feed := range existing {
		if !feed.IsEnabled {
			continue
		}
		if retained[feed.UrlKey] && (!originChanged || feed.LocationType != "RELATIVE") {
			continue
		}
		if err := queries.DeleteSiteFeed(ctx, dbgen.DeleteSiteFeedParams{ID: feed.ID, SiteID: siteID}); err != nil {
			return fmt.Errorf("remove reviewed feed: %w", err)
		}
	}
	// The unique default index is immediate; clear it before switching defaults.
	// The deferred feed-set constraint still requires exactly one at commit.
	if err := queries.ClearSiteFeedDefaults(ctx, siteID); err != nil {
		return fmt.Errorf("clear reviewed feed default: %w", err)
	}
	for _, feed := range feeds {
		if _, err := queries.UpsertSiteFeed(ctx, feed); err != nil {
			return fmt.Errorf("write reviewed feed: %w", err)
		}
	}
	return nil
}
