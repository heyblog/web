package siteaudit

import (
	"context"
	"fmt"
	"net/http"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func snapshotHasTag(snapshot Snapshot, id string) bool {
	for _, tag := range snapshot.Tags {
		if tag.ID == id {
			return true
		}
	}
	return false
}

// Normalize only the in-memory review view; original submissions and completed JSON remain immutable.
func normalizeAuditTaxonomy(ctx context.Context, q *dbgen.Queries, audit Audit) (Audit, error) {
	var err error
	audit.BaseSnapshot, err = normalizeSnapshotTaxonomy(ctx, q, audit.BaseSnapshot)
	if err != nil {
		return Audit{}, err
	}
	audit.ProposedSnapshot, err = normalizeSnapshotTaxonomy(ctx, q, audit.ProposedSnapshot)
	if err != nil {
		return Audit{}, err
	}
	if audit.ReviewDraftSnapshot != nil {
		draft, err := normalizeSnapshotTaxonomy(ctx, q, *audit.ReviewDraftSnapshot)
		if err != nil {
			return Audit{}, err
		}
		audit.ReviewDraftSnapshot = &draft
	}
	audit.FinalSnapshot, err = normalizeSnapshotTaxonomy(ctx, q, audit.FinalSnapshot)
	if err != nil {
		return Audit{}, err
	}
	return audit, nil
}
func normalizeSnapshotTaxonomy(ctx context.Context, q *dbgen.Queries, snapshot Snapshot) (Snapshot, error) {
	tags := make([]TagSnapshot, 0, len(snapshot.Tags))
	primaryID, secondaryID := "", ""
	if snapshot.TagCascadeID != "" {
		id, err := parseUUID(snapshot.TagCascadeID)
		if err != nil {
			return Snapshot{}, err
		}
		c, err := q.GetCanonicalCascade(ctx, id)
		if err != nil {
			return Snapshot{}, fmt.Errorf("resolve historical classification: %w", err)
		}
		first, err := q.GetCanonicalTag(ctx, c.Level1TagID)
		if err != nil {
			return Snapshot{}, err
		}
		second, err := q.GetCanonicalTag(ctx, c.Level2TagID)
		if err != nil {
			return Snapshot{}, err
		}
		primaryID, _ = uuidString(first.ID)
		secondaryID, _ = uuidString(second.ID)
		primary, labelErr := canonicalTagLabel(ctx, q, tagByRole(snapshot.Tags, "PRIMARY"), first, true)
		if labelErr != nil {
			return Snapshot{}, labelErr
		}
		primary.Role, primary.Level = "PRIMARY", 1
		secondary, labelErr := canonicalTagLabel(ctx, q, tagByRole(snapshot.Tags, "SECONDARY"), second, true)
		if labelErr != nil {
			return Snapshot{}, labelErr
		}
		secondary.Role, secondary.Level, secondary.ParentID = "SECONDARY", 2, primaryID

		snapshot.TagCascadeID, _ = uuidString(c.ID)
		snapshot.Classification = &CascadeSnapshot{ID: snapshot.TagCascadeID, TaxonomyKey: c.TaxonomyKey, Level1: primary, Level2: secondary}
		tags = append(tags, primary, secondary)
	}
	seen := map[string]string{}
	for _, tag := range snapshot.Tags {
		if tag.Level == 1 || tag.Level == 2 {
			continue
		}
		if tag.ID == "" {
			tags = append(tags, tag)
			continue
		}
		id, err := parseUUID(tag.ID)
		if err != nil {
			return Snapshot{}, err
		}
		row, err := q.GetCanonicalTag(ctx, id)
		if err != nil {
			return Snapshot{}, fmt.Errorf("resolve historical tag: %w", err)
		}
		tag, err = canonicalTagLabel(ctx, q, tag, row, true)
		if err != nil {
			return Snapshot{}, err
		}
		tag.Level = 3
		tag.ParentID = ""
		if tag.Role == "TERTIARY" && (tag.ID == primaryID || tag.ID == secondaryID) {
			continue
		}
		if role, exists := seen[tag.ID]; exists {
			if role != tag.Role {
				return Snapshot{}, newServiceError("taxonomy_assignment_conflict", http.StatusConflict, "merged tags have conflicting roles; correct the review draft")
			}
			continue
		}
		seen[tag.ID] = tag.Role
		tags = append(tags, tag)
	}
	snapshot.Tags = tags
	return snapshot, nil
}
