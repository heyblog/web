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

	"heyblog-api/internal/domain/site"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func (transaction *auditTransaction) applySnapshot(
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
		row, err = transaction.createSite(ctx, queries, final)
		siteID = row.ID
	} else {
		if action == ActionUpdate {
			if err := queries.UnassignSiteTertiaryTags(ctx, siteID); err != nil {
				return pgtype.UUID{}, 0, err
			}
		}
		cascadeID := rowCascadeID(current, final)
		row, err = queries.ApplySiteSnapshot(ctx, dbgen.ApplySiteSnapshotParams{
			ID: siteID, Name: final.Name, Scheme: final.Scheme, NormalizedHost: final.NormalizedHost,
			BasePath: final.BasePath, Summary: final.Summary, AccessScope: final.AccessScope,
			Visibility: final.Visibility, VisibilityReason: stringPointer(final.VisibilityReason), TagCascadeID: cascadeID, Revision: current.Revision,
		})
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, 0, newServiceError("site_revision_changed", http.StatusConflict, "the site changed while the review was being applied")
		}
		if isSiteAddressConflict(err) {
			return pgtype.UUID{}, 0, siteAddressConflictError()
		}
		return pgtype.UUID{}, 0, fmt.Errorf("apply reviewed site snapshot: %w", err)
	}
	if action == ActionCreate || action == ActionUpdate {
		primary, secondary, labelErr := classificationLabelIDs(final)
		if labelErr != nil {
			return pgtype.UUID{}, 0, labelErr
		}
		if labelErr = queries.SetSiteClassificationLabels(ctx, dbgen.SetSiteClassificationLabelsParams{ID: siteID, PrimaryLabelID: primary, SecondaryLabelID: secondary}); labelErr != nil {
			return pgtype.UUID{}, 0, labelErr
		}
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

func (transaction *auditTransaction) createSite(ctx context.Context, queries *dbgen.Queries, snapshot Snapshot) (dbgen.DirectorySite, error) {
	cascadeID, _ := parseOptionalUUID(snapshot.TagCascadeID)
	for range site.ShortIDCollisionRetries {
		shortID, err := transaction.newShortID()
		if err != nil {
			return dbgen.DirectorySite{}, fmt.Errorf("generate site short ID: %w", err)
		}
		row, err := queries.CreateSite(ctx, dbgen.CreateSiteParams{ShortID: shortID, Name: snapshot.Name, Scheme: snapshot.Scheme, NormalizedHost: snapshot.NormalizedHost, BasePath: snapshot.BasePath, Summary: snapshot.Summary, AccessScope: snapshot.AccessScope, TagCascadeID: cascadeID})
		if err == nil {
			return row, nil
		}
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) {
			return dbgen.DirectorySite{}, err
		}
		if databaseError.ConstraintName == "sites_normalized_host_unique_idx" {
			return dbgen.DirectorySite{}, siteAddressConflictError()
		}
		if databaseError.ConstraintName != "sites_short_id_unique_idx" {
			return dbgen.DirectorySite{}, err
		}
	}
	return dbgen.DirectorySite{}, errors.New("site short ID collision retry limit reached")
}

func isSiteAddressConflict(err error) bool {
	var databaseError *pgconn.PgError
	return errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == "sites_normalized_host_unique_idx"
}

func parseOptionalUUID(value string) (pgtype.UUID, error) {
	if value == "" {
		return pgtype.UUID{}, nil
	}
	return parseUUID(value)
}

func rowCascadeID(current, final Snapshot) pgtype.UUID {
	value := final.TagCascadeID
	if value == "" {
		value = current.TagCascadeID
	}
	parsed, _ := parseOptionalUUID(value)
	return parsed
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
