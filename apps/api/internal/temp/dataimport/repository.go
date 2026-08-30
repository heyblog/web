package dataimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	tempdb "heyblog-api/internal/temp/dataimport/gen"
)

const (
	importLockName            = "heyblog:data-import:v2"
	minimumImportLockCapacity = 512
	friendLinkBatchSize       = 1000
)

type friendLinkInsertRow struct {
	SourceSiteID string `json:"source_site_id"`
	TargetURL    string `json:"target_url"`
	TargetHost   string `json:"target_host"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) Import(ctx context.Context, plan Plan) (Counts, error) {
	if repository.pool == nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, errors.New("database pool is unavailable"))
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, fmt.Errorf("begin import transaction: %w", err))
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		rollbackContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackContext)
	}()

	queries := tempdb.New(tx)
	locked, err := queries.TryAcquireImportLock(ctx, importLockName)
	if err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, fmt.Errorf("acquire import lock: %w", err))
	}
	if !locked {
		return Counts{}, ErrImportRunning
	}
	lockCapacity, err := queries.ImportLockCapacity(ctx)
	if err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, fmt.Errorf("check database lock capacity: %w", err))
	}
	if lockCapacity < minimumImportLockCapacity {
		return Counts{}, errors.Join(
			ErrDependencyUnavailable,
			fmt.Errorf("max_locks_per_transaction is %d; temporary import requires at least %d", lockCapacity, minimumImportLockCapacity),
		)
	}
	empty, err := queries.DirectoryIsEmpty(ctx)
	if err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, fmt.Errorf("check directory state: %w", err))
	}
	if !empty {
		return Counts{}, ErrDirectoryNotEmpty
	}
	if err := insertPlan(ctx, queries, plan); err != nil {
		return Counts{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, fmt.Errorf("commit import transaction: %w", err))
	}
	committed = true
	return plan.Counts(), nil
}

func insertPlan(ctx context.Context, queries *tempdb.Queries, plan Plan) error {
	for _, row := range plan.Sites {
		if err := queries.InsertSite(ctx, tempdb.InsertSiteParams{
			ID: mustUUID(row.ID), ShortID: row.ShortID, Name: row.Name,
			Scheme: row.Scheme, NormalizedHost: row.NormalizedHost, BasePath: row.BasePath,
			Summary: row.Summary, AccessScope: row.AccessScope, Visibility: row.Visibility,
			VisibilityReason: nullableText(row.VisibilityReason),
			JoinedAt:         timestamp(row.JoinedAt), CreatedAt: timestamp(row.CreatedAt), UpdatedAt: timestamp(row.UpdatedAt),
		}); err != nil {
			return fmt.Errorf("insert sites: %w", err)
		}
	}
	for _, row := range plan.Feeds {
		if err := queries.InsertFeed(ctx, tempdb.InsertFeedParams{
			SiteID: mustUUID(row.SiteID), Name: row.Name, LocationType: row.LocationType,
			UrlRef:      nullableLocation(row.LocationType == "RELATIVE", row.URLRef),
			ExternalUrl: nullableLocation(row.LocationType == "EXTERNAL", row.ExternalURL),
			UrlKey:      row.URLKey, Format: row.Format, IsEnabled: row.IsEnabled, IsDefault: row.IsDefault,
		}); err != nil {
			return fmt.Errorf("insert site feeds: %w", err)
		}
	}
	for _, row := range plan.Resources {
		if err := queries.InsertResource(ctx, tempdb.InsertResourceParams{
			SiteID: mustUUID(row.SiteID), Kind: row.Kind, LocationType: row.LocationType,
			UrlRef:      nullableLocation(row.LocationType == "RELATIVE", row.URLRef),
			ExternalUrl: nullableLocation(row.LocationType == "EXTERNAL", row.ExternalURL),
			UrlKey:      row.URLKey,
		}); err != nil {
			return fmt.Errorf("insert site resources: %w", err)
		}
	}
	for _, row := range plan.Tags {
		if err := queries.InsertTag(ctx, tempdb.InsertTagParams{
			ID: mustUUID(row.ID), Name: row.Name, NormalizedName: row.NormalizedName,
			Slug: row.Slug, Description: row.Description, IsEnabled: row.IsEnabled,
		}); err != nil {
			return fmt.Errorf("insert tags: %w", err)
		}
	}
	for _, row := range plan.SiteTags {
		if err := queries.InsertSiteTag(ctx, tempdb.InsertSiteTagParams{
			SiteID: mustUUID(row.SiteID), TagID: mustUUID(row.TagID), Role: row.Role,
			Note: nullableText(row.Note),
		}); err != nil {
			return fmt.Errorf("insert site tags: %w", err)
		}
	}
	for _, row := range plan.Components {
		if err := queries.InsertSoftwareComponent(ctx, tempdb.InsertSoftwareComponentParams{
			ID: mustUUID(row.ID), Name: row.Name, NormalizedName: row.NormalizedName,
			Description: row.Description, HomepageUrl: nullableText(row.HomepageURL),
			RepositoryUrl: nullableText(row.RepositoryURL), IsOpenSource: row.IsOpenSource,
			IsEnabled: row.IsEnabled,
		}); err != nil {
			return fmt.Errorf("insert software components: %w", err)
		}
	}
	for _, row := range plan.Dependencies {
		if err := queries.InsertSoftwareDependency(ctx, tempdb.InsertSoftwareDependencyParams{
			ComponentID: mustUUID(row.ComponentID), DependencyComponentID: mustUUID(row.DependencyComponentID), Role: row.Role,
		}); err != nil {
			return fmt.Errorf("insert software dependencies: %w", err)
		}
	}
	for _, row := range plan.SiteComponents {
		if err := queries.InsertSiteSoftwareComponent(ctx, tempdb.InsertSiteSoftwareComponentParams{
			SiteID: mustUUID(row.SiteID), ComponentID: mustUUID(row.ComponentID),
			Role: row.Role, IdentifiedAt: timestamp(row.IdentifiedAt),
		}); err != nil {
			return fmt.Errorf("insert site software components: %w", err)
		}
	}
	sourceIDs := make(map[string]pgtype.UUID, len(plan.Sources))
	for _, row := range plan.Sources {
		id, err := queries.InsertSource(ctx, tempdb.InsertSourceParams{SourceKey: row.Key, Name: row.Name})
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
		if err := queries.InsertOrigin(ctx, tempdb.InsertOriginParams{
			SiteID: mustUUID(row.SiteID), SourceID: sourceID,
			ExternalReference: &externalReference, FirstDiscoveredAt: timestamp(row.FirstDiscoveredAt),
			Metadata: row.Metadata,
		}); err != nil {
			return fmt.Errorf("insert site origins: %w", err)
		}
	}
	for start := 0; start < len(plan.FriendLinks); start += friendLinkBatchSize {
		end := min(start+friendLinkBatchSize, len(plan.FriendLinks))
		batch := make([]friendLinkInsertRow, 0, end-start)
		for _, row := range plan.FriendLinks[start:end] {
			batch = append(batch, friendLinkInsertRow(row))
		}
		encoded, err := json.Marshal(batch)
		if err != nil {
			return fmt.Errorf("encode friend links batch starting at %d: %w", start, err)
		}
		if err := queries.InsertFriendLinks(ctx, encoded); err != nil {
			return fmt.Errorf("insert friend links batch starting at %d: %w", start, err)
		}
	}
	return nil
}

func mustUUID(value string) pgtype.UUID {
	var result pgtype.UUID
	if err := result.Scan(value); err != nil {
		panic(fmt.Sprintf("validated import UUID %q is invalid: %v", value, err))
	}
	return result
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func nullableLocation(include bool, value string) *string {
	if !include {
		return nil
	}
	return &value
}

func nullableText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
