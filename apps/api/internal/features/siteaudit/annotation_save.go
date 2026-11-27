package siteaudit

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"

	"heyblog-api/internal/domain/site"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type annotationChange struct {
	ShortID string
	ActorID string
	Patch   AnnotationPatch
}

type annotationWrite struct {
	q     *dbgen.Queries
	row   dbgen.DirectorySite
	patch AnnotationPatch
}

func (repo *Repository) saveAnnotation(ctx context.Context, change annotationChange) (AnnotationDetail, error) {
	actorID, err := parseUUID(change.ActorID)
	if err != nil {
		return AnnotationDetail{}, err
	}
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return AnnotationDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := repo.queries.WithTx(tx)
	if err := q.LockTaxonomy(ctx); err != nil {
		return AnnotationDetail{}, err
	}
	row, err := q.GetSiteByShortID(ctx, change.ShortID)
	if isNotFound(err) {
		return AnnotationDetail{}, annotationNotFound()
	}
	if err != nil {
		return AnnotationDetail{}, err
	}
	row, err = q.LockSiteByID(ctx, row.ID)
	if err != nil {
		return AnnotationDetail{}, err
	}
	if row.Revision != change.Patch.ExpectedRevision {
		return AnnotationDetail{}, annotationConflict()
	}
	before, err := loadAnnotation(ctx, q, row)
	if err != nil {
		return AnnotationDetail{}, err
	}
	if before.FriendVersion != change.Patch.FriendVersion {
		return AnnotationDetail{}, annotationConflict()
	}
	write := annotationWrite{q: q, row: row, patch: change.Patch}
	updated, err := write.apply(ctx, before)
	if err != nil {
		return AnnotationDetail{}, err
	}
	after, err := loadAnnotation(ctx, q, updated)
	if err != nil {
		return AnnotationDetail{}, err
	}
	previous, current, err := annotationDiff(before, after)
	if err != nil {
		return AnnotationDetail{}, err
	}
	if len(previous) == 0 {
		return before, nil // The deferred rollback avoids generating a revision for a no-op.
	}
	beforeJSON, err := json.Marshal(previous)
	if err != nil {
		return AnnotationDetail{}, err
	}
	afterJSON, err := json.Marshal(current)
	if err != nil {
		return AnnotationDetail{}, err
	}
	if err := q.InsertSiteAnnotationHistory(ctx, dbgen.InsertSiteAnnotationHistoryParams{
		SiteID: row.ID, ActorID: actorID, SiteRevision: updated.Revision, Source: "ANNOTATION",
		Reason: stringPointer(strings.TrimSpace(change.Patch.Reason)), BeforeSnapshot: beforeJSON, AfterSnapshot: afterJSON,
	}); err != nil {
		return AnnotationDetail{}, err
	}
	return after, tx.Commit(ctx)
}

func annotationConflict() error {
	return newServiceError("site_revision_changed", http.StatusConflict, "the site changed; reload it before saving")
}

func invalidAnnotation(message string) error {
	return newServiceError("invalid_annotation", http.StatusUnprocessableEntity, message)
}

func annotationDiff(before, after AnnotationDetail) (map[string]json.RawMessage, map[string]json.RawMessage, error) {
	beforeValues, err := annotationHistoryValues(before)
	if err != nil {
		return nil, nil, err
	}
	afterValues, err := annotationHistoryValues(after)
	if err != nil {
		return nil, nil, err
	}
	for field, value := range beforeValues {
		if reflect.DeepEqual(value, afterValues[field]) {
			delete(beforeValues, field)
			delete(afterValues, field)
		}
	}
	return beforeValues, afterValues, nil
}

func annotationHistoryValues(detail AnnotationDetail) (map[string]json.RawMessage, error) {
	data, err := json.Marshal(detail)
	if err != nil {
		return nil, err
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	for _, field := range []string{"short_id", "revision", "updated_at", "pending_audit_id", "friend_version", "incoming"} {
		delete(values, field)
	}
	return values, nil
}

func (write annotationWrite) apply(ctx context.Context, before AnnotationDetail) (dbgen.DirectorySite, error) {
	row, patch := write.row, write.patch
	address := site.Address{Scheme: row.Scheme, NormalizedHost: row.NormalizedHost, BasePath: row.BasePath}
	if patch.URL != nil {
		normalized, err := site.NormalizeAddress(*patch.URL)
		if err != nil {
			return dbgen.DirectorySite{}, invalidAnnotation("the site address is invalid")
		}
		address = normalized
	}
	if patch.Name != nil {
		row.Name = strings.TrimSpace(*patch.Name)
		if row.Name == "" {
			return dbgen.DirectorySite{}, invalidAnnotation("the site name is required")
		}
	}
	if patch.Summary != nil {
		row.Summary = strings.TrimSpace(*patch.Summary)
	}
	if patch.AccessScope != nil {
		row.AccessScope = *patch.AccessScope
	}
	if patch.BlogStatus != nil {
		if !patch.BlogStatus.IsValid() {
			return dbgen.DirectorySite{}, invalidAnnotation("the blog classification is invalid")
		}
		row.BlogStatus = string(*patch.BlogStatus)
	}
	cascadeID, tags, err := write.prepareTags(ctx, before)
	if err != nil {
		return dbgen.DirectorySite{}, err
	}
	if err := write.removeChangedTags(ctx, tags); err != nil {
		return dbgen.DirectorySite{}, err
	}
	// Change outgoing edges first while the old graph address still identifies the source.
	if patch.Outgoing != nil {
		if err := write.saveFriends(ctx, address, before.Outgoing); err != nil {
			return dbgen.DirectorySite{}, err
		}
	}
	updated, err := write.q.UpdateSiteAnnotation(ctx, dbgen.UpdateSiteAnnotationParams{
		SiteID: row.ID, Name: row.Name, Scheme: address.Scheme, NormalizedHost: address.NormalizedHost, BasePath: address.BasePath,
		Summary: row.Summary, AccessScope: row.AccessScope, BlogStatus: row.BlogStatus, TagCascadeID: cascadeID, Revision: row.Revision,
	})
	if isNotFound(err) {
		return dbgen.DirectorySite{}, annotationConflict()
	}
	if isSiteAddressConflict(err) {
		return dbgen.DirectorySite{}, siteAddressConflictError()
	}
	if err != nil {
		return dbgen.DirectorySite{}, err
	}
	if err := write.addChangedTags(ctx, tags); err != nil {
		return dbgen.DirectorySite{}, err
	}
	if patch.Components != nil {
		if err := write.saveComponents(ctx, before.Components); err != nil {
			return dbgen.DirectorySite{}, err
		}
	}
	if patch.Feeds != nil || patch.Resources != nil || annotationURL(updated) != before.URL {
		if err := write.saveLocations(ctx, before, address); err != nil {
			return dbgen.DirectorySite{}, err
		}
	}
	return updated, nil
}
