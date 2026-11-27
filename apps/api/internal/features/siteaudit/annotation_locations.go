package siteaudit

import (
	"context"
	"slices"
	"strings"

	"heyblog-api/internal/domain/site"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type annotationLocationChanges struct {
	feeds     []dbgen.UpsertSiteFeedParams
	resources []dbgen.UpsertSiteResourceParams
}

func (write annotationWrite) saveLocations(ctx context.Context, before AnnotationDetail, address site.Address) error {
	storedFeeds, err := write.q.ListSiteFeeds(ctx, write.row.ID)
	if err != nil {
		return err
	}
	feeds := slices.Clone(write.patch.Feeds)
	if feeds == nil {
		feeds = make([]AnnotationFeed, 0, len(storedFeeds))
		for _, row := range storedFeeds {
			url, err := address.LocationURL(site.Location{Type: row.LocationType, URLRef: stringValue(row.UrlRef), ExternalURL: stringValue(row.ExternalUrl)})
			if err != nil {
				return err
			}
			feeds = append(feeds, AnnotationFeed{
				FeedInput: FeedInput{Name: row.Name, URL: url, Format: row.Format, IsDefault: row.IsDefault},
				ID:        row.ID.String(), IsEnabled: row.IsEnabled,
			})
		}
	} else {
		for index, feed := range feeds {
			for _, row := range storedFeeds {
				if row.ID.String() == feed.ID && feed.URL == originalFeedURL(before, feed.ID) {
					url, err := address.LocationURL(site.Location{Type: row.LocationType, URLRef: stringValue(row.UrlRef), ExternalURL: stringValue(row.ExternalUrl)})
					if err != nil {
						return err
					}
					feeds[index].URL = url
					break
				}
			}
		}
	}
	storedResources, err := write.q.ListSiteResources(ctx, write.row.ID)
	if err != nil {
		return err
	}
	resources := slices.Clone(write.patch.Resources)
	if resources == nil {
		resources = make([]ResourceInput, 0, len(storedResources))
		for _, row := range storedResources {
			url, err := address.LocationURL(site.Location{Type: row.LocationType, URLRef: stringValue(row.UrlRef), ExternalURL: stringValue(row.ExternalUrl)})
			if err != nil {
				return err
			}
			resources = append(resources, ResourceInput{Kind: row.Kind, URL: url})
		}
	} else {
		for index, resource := range resources {
			if !slices.Contains(before.Resources, ResourceSnapshot(resource)) {
				continue
			}
			for _, row := range storedResources {
				if row.Kind == resource.Kind {
					url, err := address.LocationURL(site.Location{Type: row.LocationType, URLRef: stringValue(row.UrlRef), ExternalURL: stringValue(row.ExternalUrl)})
					if err != nil {
						return err
					}
					resources[index].URL = url
					break
				}
			}
		}
	}
	next, err := write.prepareLocations(feeds, resources, address)
	if err != nil {
		return err
	}
	if write.patch.Feeds != nil {
		if err := write.applyFeeds(ctx, storedFeeds, next.feeds); err != nil {
			return err
		}
	}
	if write.patch.Resources != nil {
		return write.applyResources(ctx, storedResources, next.resources)
	}
	return nil
}

func originalFeedURL(before AnnotationDetail, id string) string {
	for _, feed := range before.Feeds {
		if feed.ID == id {
			return feed.URL
		}
	}
	return ""
}

func (write annotationWrite) prepareLocations(feeds []AnnotationFeed, resources []ResourceInput, address site.Address) (annotationLocationChanges, error) {
	next := annotationLocationChanges{}
	if len(feeds) > 8 {
		return next, invalidAnnotation("a site can have at most eight feeds")
	}
	seenURLs := make(map[string]bool, len(feeds)+len(resources))
	enabled, defaults := 0, 0
	for _, feed := range feeds {
		name := strings.TrimSpace(feed.Name)
		format := strings.ToUpper(strings.TrimSpace(feed.Format))
		switch format {
		case "":
			format = "UNKNOWN"
		case "UNKNOWN", "RSS", "ATOM", "JSON":
		default:
			return next, invalidAnnotation("a feed format is invalid")
		}
		loc, err := site.NormalizeLocation(feed.URL, address, false)
		if err != nil || name == "" || len(name) > 100 || feed.IsDefault && !feed.IsEnabled {
			return next, invalidAnnotation("a feed is invalid")
		}
		if loc.URLKey == address.BasePath || seenURLs[loc.URLKey] {
			return next, siteURLPurposeConflict("feed addresses must be unique and differ from the homepage")
		}
		seenURLs[loc.URLKey] = true
		if feed.IsEnabled {
			enabled++
		}
		if feed.IsDefault {
			defaults++
		}
		next.feeds = append(next.feeds, dbgen.UpsertSiteFeedParams{
			SiteID: write.row.ID, Name: name, LocationType: loc.Type, UrlRef: stringPointer(loc.URLRef), ExternalUrl: stringPointer(loc.ExternalURL),
			UrlKey: loc.URLKey, Format: format, IsEnabled: feed.IsEnabled, IsDefault: feed.IsDefault,
		})
	}
	if enabled > 0 && defaults != 1 {
		return next, invalidAnnotation("enabled feeds require exactly one default")
	}
	normalized, err := normalizeResources(resources, address)
	if err != nil {
		return next, err
	}
	for _, resource := range normalized {
		loc, err := site.NormalizeLocation(resource.URL, address, false)
		if err != nil {
			return next, err
		}
		if seenURLs[loc.URLKey] {
			return next, siteURLPurposeConflict("a resource address cannot also be a feed")
		}
		next.resources = append(next.resources, dbgen.UpsertSiteResourceParams{
			SiteID: write.row.ID, Kind: resource.Kind, LocationType: loc.Type, UrlRef: stringPointer(loc.URLRef), ExternalUrl: stringPointer(loc.ExternalURL), UrlKey: loc.URLKey,
		})
	}
	return next, nil
}

func (write annotationWrite) applyFeeds(ctx context.Context, prior []dbgen.DirectorySiteFeed, next []dbgen.UpsertSiteFeedParams) error {
	desired := make(map[string]dbgen.UpsertSiteFeedParams, len(next))
	for _, feed := range next {
		desired[feed.UrlKey] = feed
	}
	for _, feed := range prior {
		want, exists := desired[feed.UrlKey]
		switch {
		case !exists:
			if err := write.q.DeleteSiteFeed(ctx, dbgen.DeleteSiteFeedParams{SiteID: feed.SiteID, ID: feed.ID}); err != nil {
				return err
			}
		case want.Name == feed.Name && want.Format == feed.Format && want.IsEnabled == feed.IsEnabled && want.IsDefault == feed.IsDefault &&
			want.LocationType == feed.LocationType && stringValue(want.UrlRef) == stringValue(feed.UrlRef) && stringValue(want.ExternalUrl) == stringValue(feed.ExternalUrl):
			delete(desired, feed.UrlKey)
		case feed.IsDefault && !want.IsDefault:
			if _, err := write.q.UpsertSiteFeed(ctx, want); err != nil {
				return err
			}
			delete(desired, feed.UrlKey)
		}
	}
	for _, feed := range next {
		if _, changed := desired[feed.UrlKey]; changed {
			if _, err := write.q.UpsertSiteFeed(ctx, feed); err != nil {
				return err
			}
		}
	}
	return nil
}

func (write annotationWrite) applyResources(ctx context.Context, prior []dbgen.DirectorySiteResource, next []dbgen.UpsertSiteResourceParams) error {
	desired := make(map[string]dbgen.UpsertSiteResourceParams, len(next))
	for _, resource := range next {
		desired[resource.Kind] = resource
	}
	for _, resource := range prior {
		want, exists := desired[resource.Kind]
		if exists && want.UrlKey == resource.UrlKey && want.LocationType == resource.LocationType &&
			stringValue(want.UrlRef) == stringValue(resource.UrlRef) && stringValue(want.ExternalUrl) == stringValue(resource.ExternalUrl) {
			delete(desired, resource.Kind)
			continue
		}
		if err := write.q.DeleteSiteResource(ctx, dbgen.DeleteSiteResourceParams{SiteID: resource.SiteID, Kind: resource.Kind}); err != nil {
			return err
		}
	}
	for _, resource := range next {
		if _, changed := desired[resource.Kind]; changed {
			if _, err := write.q.UpsertSiteResource(ctx, resource); err != nil {
				return err
			}
		}
	}
	return nil
}
