package siteaudit

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/domain/site"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func syncAssociations(ctx context.Context, queries *dbgen.Queries, siteID pgtype.UUID, snapshot Snapshot, reviewerID pgtype.UUID, createsProgramDependencies bool) error {
	address := site.Address{Scheme: snapshot.Scheme, NormalizedHost: snapshot.NormalizedHost, BasePath: snapshot.BasePath}
	if err := queries.DeleteSiteFeeds(ctx, siteID); err != nil {
		return fmt.Errorf("replace reviewed feeds: %w", err)
	}
	for _, feed := range snapshot.Feeds {
		location, err := site.NormalizeLocation(feed.URL, address, false)
		if err != nil {
			return err
		}
		if _, err := queries.UpsertSiteFeed(ctx, dbgen.UpsertSiteFeedParams{SiteID: siteID, Name: feed.Name, LocationType: location.Type, UrlRef: stringPointer(location.URLRef), ExternalUrl: stringPointer(location.ExternalURL), UrlKey: location.URLKey, Format: feed.Format, IsEnabled: true, IsDefault: feed.IsDefault}); err != nil {
			return fmt.Errorf("write reviewed feed: %w", err)
		}
	}
	if err := syncResources(ctx, queries, siteID, address, snapshot.Resources); err != nil {
		return err
	}
	if err := syncTags(ctx, queries, siteID, snapshot.Tags); err != nil {
		return err
	}
	if err := syncComponents(ctx, queries, siteID, snapshot.Components, reviewerID); err != nil {
		return err
	}
	if createsProgramDependencies {
		return syncProgramDependencies(ctx, queries, snapshot.Components, snapshot.ProgramDependencies)
	}
	return nil
}

func syncResources(ctx context.Context, queries *dbgen.Queries, siteID pgtype.UUID, address site.Address, resources []ResourceSnapshot) error {
	if err := queries.DeleteSiteResources(ctx, siteID); err != nil {
		return fmt.Errorf("replace reviewed resources: %w", err)
	}
	for _, resource := range resources {
		location, err := site.NormalizeLocation(resource.URL, address, false)
		if err != nil {
			return err
		}
		if _, err := queries.UpsertSiteResource(ctx, dbgen.UpsertSiteResourceParams{SiteID: siteID, Kind: resource.Kind, LocationType: location.Type, UrlRef: stringPointer(location.URLRef), ExternalUrl: stringPointer(location.ExternalURL), UrlKey: location.URLKey}); err != nil {
			if isSiteResourceURLConflict(err) {
				return siteURLPurposeConflict("resource addresses must be unique")
			}
			return fmt.Errorf("write reviewed resource: %w", err)
		}
	}
	return nil
}

func isSiteResourceURLConflict(err error) bool {
	var databaseError *pgconn.PgError
	return errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == "site_resources_site_url_unique"
}

func syncTags(ctx context.Context, queries *dbgen.Queries, siteID pgtype.UUID, tags []TagSnapshot) error {
	if err := queries.UnassignSiteTertiaryTags(ctx, siteID); err != nil {
		return fmt.Errorf("replace reviewed tags: %w", err)
	}
	position := int16(0)
	for _, tag := range tags {
		if tag.Level == 1 || tag.Level == 2 || tag.Role != "TERTIARY" {
			continue
		}
		tagID, err := parseUUID(tag.ID)
		if err != nil {
			return err
		}
		labelID, err := parseOptionalUUID(tag.LabelID)
		if err != nil {
			return err
		}
		var tagPosition *int16
		position++
		tagPosition = &position
		if _, err := queries.AssignSiteTag(ctx, dbgen.AssignSiteTagParams{SiteID: siteID, TagID: tagID, LabelID: labelID, Role: tag.Role, AssignmentSource: "MANUAL", Position: tagPosition}); err != nil {
			return fmt.Errorf("assign reviewed tag: %w", err)
		}
	}
	return nil
}

func syncComponents(ctx context.Context, queries *dbgen.Queries, siteID pgtype.UUID, components []ComponentSnapshot, reviewerID pgtype.UUID) error {
	if err := queries.UnassignAllSiteSoftwareComponents(ctx, siteID); err != nil {
		return fmt.Errorf("replace reviewed components: %w", err)
	}
	for _, component := range components {
		componentID, err := parseUUID(component.ID)
		if err != nil {
			return err
		}
		if _, err := queries.AssignSiteSoftwareComponent(ctx, dbgen.AssignSiteSoftwareComponentParams{SiteID: siteID, ComponentID: componentID, Role: component.Role, EvidenceSource: "MANUAL", IdentifiedBy: reviewerID}); err != nil {
			return fmt.Errorf("assign reviewed software component: %w", err)
		}
	}
	return nil
}

func syncProgramDependencies(ctx context.Context, queries *dbgen.Queries, components, dependencies []ComponentSnapshot) error {
	var programID pgtype.UUID
	for _, component := range components {
		if component.Role != "SITE_PROGRAM" {
			continue
		}
		parsed, err := parseUUID(component.ID)
		if err != nil {
			return fmt.Errorf("parse reviewed site program: %w", err)
		}
		programID = parsed
		break
	}
	if !programID.Valid {
		return nil
	}
	for _, dependency := range dependencies {
		dependencyID, err := parseUUID(dependency.ID)
		if err != nil {
			return fmt.Errorf("parse reviewed program dependency: %w", err)
		}
		if _, err := queries.AddSoftwareComponentDependency(ctx, dbgen.AddSoftwareComponentDependencyParams{ComponentID: programID, DependencyComponentID: dependencyID, Role: dependency.Role}); err != nil {
			return fmt.Errorf("write reviewed program dependency: %w", err)
		}
	}
	return nil
}
