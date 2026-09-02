package siteaudit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/domain/site"
)

func (service *Service) applySnapshot(
	ctx context.Context,
	queries *dbgen.Queries,
	action Action,
	current Snapshot,
	final Snapshot,
	siteID pgtype.UUID,
	reviewerID pgtype.UUID,
	createsProgramDependencies bool,
) (pgtype.UUID, int64, error) {
	var row dbgen.DirectorySite
	var err error
	if action == ActionCreate {
		row, err = service.createSite(ctx, queries, final)
		siteID = row.ID
	} else {
		row, err = queries.ApplySiteSnapshot(ctx, dbgen.ApplySiteSnapshotParams{
			ID: siteID, Name: final.Name, Scheme: final.Scheme, NormalizedHost: final.NormalizedHost,
			BasePath: final.BasePath, Summary: final.Summary, AccessScope: final.AccessScope,
			Visibility: final.Visibility, VisibilityReason: stringPointer(final.VisibilityReason), Revision: current.Revision,
		})
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, 0, newServiceError("site_revision_changed", http.StatusConflict, "the site changed while the review was being applied")
		}
		return pgtype.UUID{}, 0, fmt.Errorf("apply reviewed site snapshot: %w", err)
	}
	final.Revision = row.Revision
	if action == ActionCreate || action == ActionUpdate {
		if err := syncAssociations(ctx, queries, siteID, final, reviewerID, createsProgramDependencies); err != nil {
			return pgtype.UUID{}, 0, err
		}
	}
	if action == ActionCreate {
		if err := addSubmissionOrigin(ctx, queries, siteID); err != nil {
			return pgtype.UUID{}, 0, err
		}
	}
	return siteID, row.Revision, nil
}

func (service *Service) createSite(ctx context.Context, queries *dbgen.Queries, snapshot Snapshot) (dbgen.DirectorySite, error) {
	for range site.ShortIDCollisionRetries {
		shortID, err := service.newShortID()
		if err != nil {
			return dbgen.DirectorySite{}, fmt.Errorf("generate site short ID: %w", err)
		}
		row, err := queries.CreateSite(ctx, dbgen.CreateSiteParams{ShortID: shortID, Name: snapshot.Name, Scheme: snapshot.Scheme, NormalizedHost: snapshot.NormalizedHost, BasePath: snapshot.BasePath, Summary: snapshot.Summary, AccessScope: snapshot.AccessScope})
		if err == nil {
			return row, nil
		}
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.ConstraintName != "sites_short_id_unique_idx" {
			return dbgen.DirectorySite{}, err
		}
	}
	return dbgen.DirectorySite{}, errors.New("site short ID collision retry limit reached")
}

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
			return fmt.Errorf("write reviewed resource: %w", err)
		}
	}
	return nil
}

func syncTags(ctx context.Context, queries *dbgen.Queries, siteID pgtype.UUID, tags []TagSnapshot) error {
	if err := queries.UnassignAllSiteTags(ctx, siteID); err != nil {
		return fmt.Errorf("replace reviewed tags: %w", err)
	}
	for _, tag := range tags {
		tagID, err := parseUUID(tag.ID)
		if err != nil {
			return err
		}
		if _, err := queries.AssignSiteTag(ctx, dbgen.AssignSiteTagParams{SiteID: siteID, TagID: tagID, Role: tag.Role, AssignmentSource: "MANUAL"}); err != nil {
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

func addSubmissionOrigin(ctx context.Context, queries *dbgen.Queries, siteID pgtype.UUID) error {
	source, err := queries.UpsertSiteSource(ctx, dbgen.UpsertSiteSourceParams{SourceKey: "WEB_SUBMISSION", Name: "Web submission", IsEnabled: true})
	if err != nil {
		return fmt.Errorf("ensure web submission source: %w", err)
	}
	metadata, _ := json.Marshal(map[string]string{"channel": "anonymous_form"})
	if _, err := queries.AddSiteOrigin(ctx, dbgen.AddSiteOriginParams{SiteID: siteID, SourceID: source.ID, FirstDiscoveredAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, Metadata: metadata}); err != nil {
		return fmt.Errorf("record web submission origin: %w", err)
	}
	return nil
}
