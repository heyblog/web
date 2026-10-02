package dataimport

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func insertPlan(ctx context.Context, queries *dbgen.Queries, plan Plan) error {
	if err := insertProfiles(ctx, queries, plan); err != nil {
		return err
	}
	if err := insertClassification(ctx, queries, plan); err != nil {
		return err
	}
	if err := insertOrigins(ctx, queries, plan); err != nil {
		return err
	}
	return insertFriendLinks(ctx, queries, plan)
}
func insertProfiles(ctx context.Context, queries *dbgen.Queries, plan Plan) error {
	for _, row := range plan.Sites {
		if err := queries.InsertSite(ctx, dbgen.InsertSiteParams{
			ID: mustUUID(row.ID), ShortID: row.ShortID, Name: row.Name,
			Scheme: row.Scheme, NormalizedHost: row.NormalizedHost, BasePath: row.BasePath,
			Summary: row.Summary, AccessScope: row.AccessScope, Visibility: row.Visibility,
			VisibilityReason: nullableText(row.VisibilityReason),
			JoinedAt:         timestamp(row.JoinedAt), UpdatedAt: timestamp(row.UpdatedAt),
		}); err != nil {
			return fmt.Errorf("insert sites: %w", err)
		}
	}
	for _, row := range plan.Feeds {
		if err := queries.InsertFeed(ctx, dbgen.InsertFeedParams{
			SiteID: mustUUID(row.SiteID), Name: row.Name, LocationType: row.LocationType,
			UrlRef:      nullableLocation(row.LocationType == "RELATIVE", row.URLRef),
			ExternalUrl: nullableLocation(row.LocationType == "EXTERNAL", row.ExternalURL),
			UrlKey:      row.URLKey, Format: row.Format, IsEnabled: row.IsEnabled, IsDefault: row.IsDefault,
		}); err != nil {
			return fmt.Errorf("insert site feeds: %w", err)
		}
	}
	for _, row := range plan.Resources {
		if err := queries.InsertResource(ctx, dbgen.InsertResourceParams{
			SiteID: mustUUID(row.SiteID), Kind: row.Kind, LocationType: row.LocationType,
			UrlRef:      nullableLocation(row.LocationType == "RELATIVE", row.URLRef),
			ExternalUrl: nullableLocation(row.LocationType == "EXTERNAL", row.ExternalURL),
			UrlKey:      row.URLKey,
		}); err != nil {
			return fmt.Errorf("insert site resources: %w", err)
		}
	}
	return nil
}

func insertClassification(ctx context.Context, queries *dbgen.Queries, plan Plan) error {
	for _, row := range plan.Tags {
		if err := queries.InsertTag(ctx, dbgen.InsertTagParams{
			ID: mustUUID(row.ID), Name: row.Name, NormalizedName: row.NormalizedName,
			Slug: row.Slug, Description: row.Description, IsEnabled: row.IsEnabled,
		}); err != nil {
			return fmt.Errorf("insert tags: %w", err)
		}
	}
	for _, row := range plan.SiteTags {
		if row.Role != "WARNING" {
			continue
		}
		if err := queries.InsertSiteTag(ctx, dbgen.InsertSiteTagParams{
			SiteID: mustUUID(row.SiteID), TagID: mustUUID(row.TagID), Role: row.Role,
			Position: nil, Note: nullableText(row.Note),
		}); err != nil {
			return fmt.Errorf("insert site tags: %w", err)
		}
	}
	for _, row := range plan.Components {
		if err := queries.InsertSoftwareComponent(ctx, dbgen.InsertSoftwareComponentParams{
			ID: mustUUID(row.ID), Name: row.Name, NormalizedName: row.NormalizedName,
			Description: row.Description, HomepageUrl: nullableText(row.HomepageURL),
			RepositoryUrl: nullableText(row.RepositoryURL), IsOpenSource: row.IsOpenSource,
			IsEnabled: row.IsEnabled,
		}); err != nil {
			return fmt.Errorf("insert software components: %w", err)
		}
	}
	for _, row := range plan.Dependencies {
		if err := queries.InsertSoftwareDependency(ctx, dbgen.InsertSoftwareDependencyParams{
			ComponentID: mustUUID(row.ComponentID), DependencyComponentID: mustUUID(row.DependencyComponentID), Role: row.Role,
		}); err != nil {
			return fmt.Errorf("insert software dependencies: %w", err)
		}
	}
	for _, row := range plan.SiteComponents {
		if err := queries.InsertSiteSoftwareComponent(ctx, dbgen.InsertSiteSoftwareComponentParams{
			SiteID: mustUUID(row.SiteID), ComponentID: mustUUID(row.ComponentID),
			Role: row.Role, IdentifiedAt: timestamp(row.IdentifiedAt),
		}); err != nil {
			return fmt.Errorf("insert site software components: %w", err)
		}
	}
	return nil
}

func insertOrigins(ctx context.Context, queries *dbgen.Queries, plan Plan) error {
	sourceIDs := make(map[string]pgtype.UUID, len(plan.Sources))
	for _, row := range plan.Sources {
		id, err := queries.InsertSource(ctx, dbgen.InsertSourceParams{SourceKey: row.Key, Name: row.Name})
		if err != nil {
			return fmt.Errorf("insert site sources: %w", err)
		}
		sourceIDs[row.Key] = id
	}
	for _, row := range plan.Origins {
		externalReference := row.ExternalReference
		sourceID, exists := sourceIDs[row.SourceKey]
		if !exists {
			return fmt.Errorf("insert site origins: source %q is missing", row.SourceKey)
		}
		if err := queries.InsertOrigin(ctx, dbgen.InsertOriginParams{
			SiteID: mustUUID(row.SiteID), SourceID: sourceID,
			ExternalReference: &externalReference, FirstDiscoveredAt: timestamp(row.FirstDiscoveredAt),
			Metadata: row.Metadata,
		}); err != nil {
			return fmt.Errorf("insert site origins: %w", err)
		}
	}
	return nil
}
