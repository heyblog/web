package siteaudit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/domain/site"
)

type Repository struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
}

type snapshotQueries interface {
	GetSiteByID(context.Context, pgtype.UUID) (dbgen.DirectorySite, error)
	ListSiteFeeds(context.Context, pgtype.UUID) ([]dbgen.DirectorySiteFeed, error)
	ListSiteResources(context.Context, pgtype.UUID) ([]dbgen.DirectorySiteResource, error)
	ListSiteTags(context.Context, pgtype.UUID) ([]dbgen.ListSiteTagsRow, error)
	ListSiteSoftwareComponents(context.Context, pgtype.UUID) ([]dbgen.ListSiteSoftwareComponentsRow, error)
	ListSoftwareComponentDependencies(context.Context, pgtype.UUID) ([]dbgen.ListSoftwareComponentDependenciesRow, error)
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: dbgen.New(pool)}
}

func (repository *Repository) InTransaction(
	ctx context.Context,
	operation func(*dbgen.Queries) error,
) error {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin site audit transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if err := operation(repository.queries.WithTx(transaction)); err != nil {
		return err
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit site audit transaction: %w", err)
	}
	return nil
}

func loadSnapshot(ctx context.Context, queries snapshotQueries, siteID pgtype.UUID) (Snapshot, error) {
	row, err := queries.GetSiteByID(ctx, siteID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("get site for snapshot: %w", err)
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
	return mapSnapshot(row, feeds, resources, tags, components, dependencies)
}

func mapSnapshot(
	row dbgen.DirectorySite,
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
		snapshot.Tags = append(snapshot.Tags, TagSnapshot{ID: id, Name: tag.Name, Slug: tag.Slug, Description: tag.Description, Role: tag.Role})
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

func auditFromRow(row dbgen.DirectorySiteAudit) (Audit, error) {
	audit := Audit{Action: Action(row.Action), Status: Status(row.Status), RequestReason: row.RequestReason, NotifyByEmail: row.NotifyByEmail}
	var err error
	if audit.ID, err = uuidString(row.ID); err != nil {
		return Audit{}, err
	}
	if row.SiteID.Valid {
		if audit.SiteID, err = uuidString(row.SiteID); err != nil {
			return Audit{}, err
		}
	}
	if row.BaseRevision != nil {
		audit.BaseRevision = *row.BaseRevision
	}
	if err := decodeSnapshot(row.BaseSnapshot, &audit.BaseSnapshot); err != nil {
		return Audit{}, fmt.Errorf("decode base snapshot: %w", err)
	}
	if err := decodeSnapshot(row.ProposedSnapshot, &audit.ProposedSnapshot); err != nil {
		return Audit{}, fmt.Errorf("decode proposed snapshot: %w", err)
	}
	if len(row.ReviewDraftSnapshot) > 0 {
		reviewDraft := Snapshot{}
		if err := decodeSnapshot(row.ReviewDraftSnapshot, &reviewDraft); err != nil {
			return Audit{}, fmt.Errorf("decode review draft snapshot: %w", err)
		}
		audit.ReviewDraftSnapshot = &reviewDraft
	}
	audit.ReviewDraftRevision = row.ReviewDraftRevision
	if row.ReviewDraftUpdatedBy.Valid {
		audit.ReviewDraftUpdatedBy, _ = uuidString(row.ReviewDraftUpdatedBy)
	}
	if row.ReviewDraftUpdatedAt.Valid {
		updatedAt := row.ReviewDraftUpdatedAt.Time
		audit.ReviewDraftUpdatedAt = &updatedAt
	}
	if err := decodeSnapshot(row.FinalSnapshot, &audit.FinalSnapshot); err != nil {
		return Audit{}, fmt.Errorf("decode final snapshot: %w", err)
	}
	audit.SubmitterName = stringValue(row.SubmitterName)
	audit.SubmitterEmail = stringValue(row.SubmitterEmail)
	audit.ReviewerComment = stringValue(row.ReviewerComment)
	if row.ReviewedBy.Valid {
		audit.ReviewedBy, _ = uuidString(row.ReviewedBy)
	}
	if row.ReviewedAt.Valid {
		reviewedAt := row.ReviewedAt.Time
		audit.ReviewedAt = &reviewedAt
	}
	audit.CreatedAt = row.CreatedAt.Time
	audit.UpdatedAt = row.UpdatedAt.Time
	return audit, nil
}

func decodeSnapshot(data []byte, destination *Snapshot) error {
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("decode snapshot JSON: %w", err)
	}
	return nil
}

func encodeSnapshot(snapshot Snapshot) ([]byte, error) {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("encode snapshot JSON: %w", err)
	}
	return data, nil
}

func parseUUID(value string) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(value); err != nil {
		return pgtype.UUID{}, fmt.Errorf("parse UUID: %w", err)
	}
	return parsed, nil
}

func uuidString(value pgtype.UUID) (string, error) {
	if !value.Valid {
		return "", errors.New("UUID is null")
	}
	driverValue, err := value.Value()
	if err != nil {
		return "", fmt.Errorf("format UUID: %w", err)
	}
	text, ok := driverValue.(string)
	if !ok {
		return "", fmt.Errorf("format UUID: unexpected value type %T", driverValue)
	}
	return text, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copyOfValue := value
	return &copyOfValue
}

func boolPointer(value bool) *bool {
	copyOfValue := value
	return &copyOfValue
}

func timestampPointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	copyOfValue := value.Time
	return &copyOfValue
}

func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
