package siteaudit

import (
	"context"
	"errors"
	"fmt"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"net/http"

	"github.com/jackc/pgx/v5"
)

func snapshotHasTag(snapshot Snapshot, id string) bool {
	for _, tag := range snapshot.Tags {
		if tag.ID == id {
			return true
		}
	}
	return false
}

// Projection never changes the immutable persisted submission or history.
func normalizeAuditTaxonomy(ctx context.Context, q *dbgen.Queries, audit Audit) (Audit, error) {
	return projectAuditTaxonomy(ctx, q, audit, false)
}

func projectAuditTaxonomy(ctx context.Context, q *dbgen.Queries, audit Audit, preserveNames bool) (Audit, error) {
	snapshots := []*Snapshot{&audit.BaseSnapshot, &audit.ProposedSnapshot, &audit.FinalSnapshot, &audit.CurrentSnapshot}
	if audit.ReviewDraftSnapshot != nil {
		snapshots = append(snapshots, audit.ReviewDraftSnapshot)
	}
	for _, snapshot := range snapshots {
		projected, err := projectSnapshotTaxonomy(ctx, q, *snapshot, preserveNames)
		if err != nil {
			return Audit{}, err
		}
		*snapshot = projected
	}
	return audit, nil
}

func normalizeSnapshotTaxonomy(ctx context.Context, q *dbgen.Queries, snapshot Snapshot) (Snapshot, error) {
	projected, err := projectSnapshotTaxonomy(ctx, q, snapshot, false)
	if err != nil {
		return Snapshot{}, err
	}
	for _, tag := range projected.Tags {
		if tag.unresolved {
			return Snapshot{}, newServiceError("taxonomy_assignment_conflict", http.StatusConflict, "a historical tag is unavailable; correct the review draft")
		}
	}
	return projected, nil
}

func projectSnapshotTaxonomy(ctx context.Context, q *dbgen.Queries, snapshot Snapshot, preserveNames bool) (Snapshot, error) {
	tags := make([]TagSnapshot, 0, len(snapshot.Tags))
	for _, tag := range snapshot.Tags {
		if tag.ID == "" && tag.historicalLabelID == "" {
			tags = append(tags, tag)
			continue
		}
		row, err := historicalTag(ctx, q, tag)
		if errors.Is(err, pgx.ErrNoRows) {
			tag.unresolved = true
			tags = append(tags, tag)
			continue
		}
		if err != nil {
			return Snapshot{}, fmt.Errorf("resolve historical tag: %w", err)
		}
		name := tag.Name
		tag, _ = canonicalTag(tag, row, true)
		if preserveNames && name != "" {
			tag.Name = name
		}
		tags = append(tags, tag)
	}
	snapshot.Tags = tags
	if snapshot.TagCascadeID != "" {
		id, err := parseUUID(snapshot.TagCascadeID)
		if err != nil {
			return Snapshot{}, err
		}
		cascade, err := q.GetCanonicalCascade(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return snapshot, nil
		}
		if err != nil {
			return Snapshot{}, err
		}
		primary, secondary := tagByRole(tags, "PRIMARY"), tagByRole(tags, "SECONDARY")
		if primary.ID == "" || secondary.ID == "" {
			return snapshot, nil
		}
		primaryID, err := parseUUID(primary.ID)
		if err != nil {
			return snapshot, nil
		}
		secondaryID, err := parseUUID(secondary.ID)
		if err != nil {
			return snapshot, nil
		}
		paths, err := q.ListManagedCascades(ctx)
		if err != nil {
			return Snapshot{}, err
		}
		found := false
		for _, path := range paths {
			if path.Scope == cascade.Scope && path.Level1TagID == primaryID && path.Level2TagID == secondaryID && !path.MergedIntoID.Valid {
				cascade.ID = path.ID
				cascade.TaxonomyKey = path.TaxonomyKey
				found = true
				break
			}
		}
		if !found {
			for i := range snapshot.Tags {
				if snapshot.Tags[i].Role == "PRIMARY" || snapshot.Tags[i].Role == "SECONDARY" {
					snapshot.Tags[i].unresolved = true
				}
			}
			return snapshot, nil
		}
		snapshot.TagCascadeID, _ = uuidString(cascade.ID)
		secondary.ParentID = primary.ID
		for i := range snapshot.Tags {
			if snapshot.Tags[i].Role == "SECONDARY" {
				snapshot.Tags[i].ParentID = primary.ID
			}
		}
		snapshot.Classification = &CascadeSnapshot{ID: snapshot.TagCascadeID, TaxonomyKey: cascade.TaxonomyKey, Level1: primary, Level2: secondary}
	}
	seen := map[string]string{}
	primary, secondary := tagByRole(tags, "PRIMARY").ID, tagByRole(tags, "SECONDARY").ID
	snapshot.Tags = make([]TagSnapshot, 0, len(tags))
	for _, tag := range tags {
		if tag.Role == "TERTIARY" && (tag.ID == primary || tag.ID == secondary) && tag.ID != "" {
			continue
		}
		if tag.Level < 3 && (tag.Role == "PRIMARY" || tag.Role == "SECONDARY") {
			snapshot.Tags = append(snapshot.Tags, tag)
			continue
		}
		if tag.ID != "" {
			if role, exists := seen[tag.ID]; exists {
				if role != tag.Role {
					tag.unresolved = true
				} else {
					continue
				}
			}
			seen[tag.ID] = tag.Role
		}
		snapshot.Tags = append(snapshot.Tags, tag)
	}
	return snapshot, nil
}

func historicalTag(ctx context.Context, q *dbgen.Queries, tag TagSnapshot) (dbgen.DirectoryTag, error) {
	if tag.historicalLabelID != "" {
		id, err := parseUUID(tag.historicalLabelID)
		if err != nil {
			return dbgen.DirectoryTag{}, err
		}
		return q.GetHistoricalTagLabel(ctx, id)
	}
	id, err := parseUUID(tag.ID)
	if err != nil {
		return dbgen.DirectoryTag{}, err
	}
	return q.GetCanonicalTag(ctx, id)
}
