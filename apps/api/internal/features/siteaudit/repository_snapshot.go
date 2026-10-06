package siteaudit

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/domain/site"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type snapshotQueries interface {
	GetSiteByID(context.Context, pgtype.UUID) (dbgen.DirectorySite, error)
	ListSiteFeeds(context.Context, pgtype.UUID) ([]dbgen.DirectorySiteFeed, error)
	ListSiteResources(context.Context, pgtype.UUID) ([]dbgen.DirectorySiteResource, error)
	ListSiteTags(context.Context, pgtype.UUID) ([]dbgen.ListSiteTagsRow, error)
	ListSiteSoftwareComponents(context.Context, pgtype.UUID) ([]dbgen.ListSiteSoftwareComponentsRow, error)
	ListSoftwareComponentDependencies(context.Context, pgtype.UUID) ([]dbgen.ListSoftwareComponentDependenciesRow, error)
}

func loadSnapshot(ctx context.Context, queries snapshotQueries, siteID pgtype.UUID) (Snapshot, error) {
	row, err := queries.GetSiteByID(ctx, siteID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("get site for snapshot: %w", err)
	}
	var cascade dbgen.GetReadableSiteTagCascadeRow
	if cascadeQueries, ok := queries.(interface {
		GetReadableSiteTagCascade(context.Context, pgtype.UUID) (dbgen.GetReadableSiteTagCascadeRow, error)
	}); ok {
		cascade, err = cascadeQueries.GetReadableSiteTagCascade(ctx, row.TagCascadeID)
		if err != nil {
			return Snapshot{}, fmt.Errorf("get site classification for snapshot: %w", err)
		}
	}
	feeds, err := queries.ListSiteFeeds(ctx, siteID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list site feeds for snapshot: %w", err)
	}
	resources, err := queries.ListSiteResources(ctx, siteID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list site resources for snapshot: %w", err)
	}
	tags, err := queries.ListSiteTags(ctx, siteID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list site tags for snapshot: %w", err)
	}
	components, err := queries.ListSiteSoftwareComponents(ctx, siteID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list site software components for snapshot: %w", err)
	}
	dependencies := make([]dbgen.ListSoftwareComponentDependenciesRow, 0)
	for _, component := range components {
		if component.Role != "SITE_PROGRAM" {
			continue
		}
		dependencies, err = queries.ListSoftwareComponentDependencies(ctx, component.ComponentID)
		if err != nil {
			return Snapshot{}, fmt.Errorf("list site program dependencies for snapshot: %w", err)
		}
		break
	}
	snapshot, err := mapSnapshot(row, cascade, feeds, resources, tags, components, dependencies)
	if err != nil {
		return Snapshot{}, err
	}
	if labels, ok := queries.(interface {
		GetTagLabel(context.Context, pgtype.UUID) (dbgen.DirectoryTagLabel, error)
	}); ok {
		for i, tag := range snapshot.Tags {
			if tag.LabelID == "" {
				continue
			}
			id, parseErr := parseUUID(tag.LabelID)
			if parseErr != nil {
				return Snapshot{}, parseErr
			}
			label, labelErr := labels.GetTagLabel(ctx, id)
			if labelErr != nil {
				return Snapshot{}, labelErr
			}
			snapshot.Tags[i].Name = label.Name
		}
		if snapshot.Classification != nil {
			snapshot.Classification.Level1 = snapshot.Tags[0]
			snapshot.Classification.Level2 = snapshot.Tags[1]
		}
	}
	return snapshot, nil
}

func mapSnapshot(
	row dbgen.DirectorySite,
	cascade dbgen.GetReadableSiteTagCascadeRow,
	feeds []dbgen.DirectorySiteFeed,
	resources []dbgen.DirectorySiteResource,
	tags []dbgen.ListSiteTagsRow,
	components []dbgen.ListSiteSoftwareComponentsRow,
	dependencies []dbgen.ListSoftwareComponentDependenciesRow,
) (Snapshot, error) {
	identifier, err := uuidString(row.ID)
	if err != nil {
		return Snapshot{}, err
	}
	address := site.Address{Scheme: row.Scheme, NormalizedHost: row.NormalizedHost, BasePath: row.BasePath}
	snapshot := Snapshot{
		SiteID: identifier, Revision: row.Revision, ShortID: row.ShortID, Name: row.Name,
		Scheme: row.Scheme, NormalizedHost: row.NormalizedHost, BasePath: row.BasePath,
		Summary: row.Summary, AccessScope: row.AccessScope, Visibility: row.Visibility,
		Feeds: make([]FeedSnapshot, 0, len(feeds)), Resources: make([]ResourceSnapshot, 0, len(resources)),
		Tags: make([]TagSnapshot, 0, len(tags)), Components: make([]ComponentSnapshot, 0, len(components)),
		ProgramDependencies: make([]ComponentSnapshot, 0, len(dependencies)),
	}
	if row.TagCascadeID.Valid {
		snapshot.TagCascadeID, _ = uuidString(row.TagCascadeID)
	}
	if cascade.ID.Valid {
		level1ID, level1Err := uuidString(cascade.Level1ID)
		level2ID, level2Err := uuidString(cascade.Level2ID)
		if level1Err != nil || level2Err != nil {
			return Snapshot{}, fmt.Errorf("map site classification identifiers")
		}
		level1 := TagSnapshot{ID: level1ID, LabelID: optionalUUIDString(row.PrimaryLabelID), Name: cascade.Level1Name, Slug: cascade.Level1Slug, Description: cascade.Level1Description, Role: "PRIMARY", Level: 1}
		level2 := TagSnapshot{ID: level2ID, LabelID: optionalUUIDString(row.SecondaryLabelID), Name: cascade.Level2Name, Slug: cascade.Level2Slug, Description: cascade.Level2Description, Role: "SECONDARY", Level: 2, ParentID: level1ID}
		snapshot.Tags = append(snapshot.Tags, level1, level2)
		snapshot.Classification = &CascadeSnapshot{
			ID: snapshot.TagCascadeID, TaxonomyKey: cascade.TaxonomyKey,
			Level1: level1, Level2: level2,
		}
	}
	if row.CustomID != nil {
		snapshot.CustomID = *row.CustomID
	}
	if row.VisibilityReason != nil {
		snapshot.VisibilityReason = *row.VisibilityReason
	}
	for _, feed := range feeds {
		urlValue, locationErr := address.LocationURL(site.Location{Type: feed.LocationType, URLRef: stringValue(feed.UrlRef), ExternalURL: stringValue(feed.ExternalUrl)})
		if locationErr != nil {
			return Snapshot{}, fmt.Errorf("map feed location: %w", locationErr)
		}
		id, idErr := uuidString(feed.ID)
		if idErr != nil {
			return Snapshot{}, idErr
		}
		snapshot.Feeds = append(snapshot.Feeds, FeedSnapshot{ID: id, Name: feed.Name, URL: urlValue, Format: feed.Format, IsDefault: feed.IsDefault})
	}
	for _, resource := range resources {
		urlValue, locationErr := address.LocationURL(site.Location{Type: resource.LocationType, URLRef: stringValue(resource.UrlRef), ExternalURL: stringValue(resource.ExternalUrl)})
		if locationErr != nil {
			return Snapshot{}, fmt.Errorf("map resource location: %w", locationErr)
		}
		snapshot.Resources = append(snapshot.Resources, ResourceSnapshot{Kind: resource.Kind, URL: urlValue})
	}
	for _, tag := range tags {
		id, idErr := uuidString(tag.TagID)
		if idErr != nil {
			return Snapshot{}, idErr
		}
		snapshot.Tags = append(snapshot.Tags, TagSnapshot{ID: id, LabelID: optionalUUIDString(tag.LabelID), Name: tag.Name, Slug: tag.Slug, Description: tag.Description, Role: tag.Role, Level: 3})
	}
	for _, component := range components {
		id, idErr := uuidString(component.ComponentID)
		if idErr != nil {
			return Snapshot{}, idErr
		}
		snapshot.Components = append(snapshot.Components, ComponentSnapshot{ID: id, Name: component.Name, Role: component.Role, HomepageURL: stringValue(component.HomepageUrl), RepositoryURL: stringValue(component.RepositoryUrl), IsOpenSource: boolPointer(component.IsOpenSource)})
	}
	for _, dependency := range dependencies {
		id, idErr := uuidString(dependency.DependencyComponentID)
		if idErr != nil {
			return Snapshot{}, idErr
		}
		snapshot.ProgramDependencies = append(snapshot.ProgramDependencies, ComponentSnapshot{ID: id, Name: dependency.Name, Role: dependency.Role, HomepageURL: stringValue(dependency.HomepageUrl), RepositoryURL: stringValue(dependency.RepositoryUrl), IsOpenSource: boolPointer(dependency.IsOpenSource)})
	}
	return snapshot, nil
}
