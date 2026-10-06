package siteaudit

import (
	"context"
	"errors"
	"net/http"
	"strings"

	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type tagLabelQueries interface {
	GetTagLabel(context.Context, pgtype.UUID) (dbgen.DirectoryTagLabel, error)
	GetTagLabelByNormalizedName(context.Context, string) (dbgen.DirectoryTagLabel, error)
}

// A chosen name is authoritative only after checking its concept ownership.
func canonicalTagLabel(ctx context.Context, q tagLabelQueries, tag TagSnapshot, canonical dbgen.DirectoryTagDictionary, allowDisabled bool) (TagSnapshot, error) {
	labelID := canonical.DefaultLabelID
	if tag.LabelID != "" {
		parsed, err := parseUUID(tag.LabelID)
		if err != nil {
			return TagSnapshot{}, newServiceError("invalid_tag_label", http.StatusUnprocessableEntity, "a selected tag name is invalid")
		}
		labelID = parsed
	} else if tag.Name != "" || tag.SuggestedName != "" {
		name := tag.Name
		if name == "" {
			name = tag.SuggestedName
		}
		normalized := strings.ToLower(strings.TrimSpace(name))
		label, err := q.GetTagLabelByNormalizedName(ctx, normalized)
		if err == nil && label.TagID == canonical.ID {
			labelID = label.ID
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return TagSnapshot{}, err
		}
	}
	label, err := q.GetTagLabel(ctx, labelID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return TagSnapshot{}, err
	}
	if err != nil || label.TagID != canonical.ID {
		return TagSnapshot{}, newServiceError("invalid_tag_label", http.StatusUnprocessableEntity, "the selected name does not belong to this tag")
	}
	if !allowDisabled && (!label.IsEnabled || !canonical.IsEnabled) {
		return TagSnapshot{}, newServiceError("invalid_tag_label", http.StatusUnprocessableEntity, "the selected tag name is no longer available")
	}
	tag.ID, _ = uuidString(canonical.ID)
	tag.LabelID, _ = uuidString(label.ID)
	tag.Name, tag.Slug, tag.Description = label.Name, canonical.Slug, canonical.Description
	tag.SuggestedName = ""
	return tag, nil
}

func snapshotHasTagLabel(snapshot Snapshot, tag TagSnapshot) bool {
	for _, existing := range snapshot.Tags {
		if existing.ID == tag.ID && existing.Role == tag.Role && (tag.LabelID == "" || existing.LabelID == tag.LabelID) {
			return true
		}
	}
	return false
}

func classificationLabelIDs(snapshot Snapshot) (pgtype.UUID, pgtype.UUID, error) {
	var primary, secondary pgtype.UUID
	for _, tag := range snapshot.Tags {
		if tag.Role != "PRIMARY" && tag.Role != "SECONDARY" {
			continue
		}
		id, err := parseOptionalUUID(tag.LabelID)
		if err != nil {
			return pgtype.UUID{}, pgtype.UUID{}, err
		}
		if tag.Role == "PRIMARY" {
			primary = id
		} else {
			secondary = id
		}
	}
	return primary, secondary, nil
}

func optionalUUIDString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	value, _ := uuidString(id)
	return value
}

func tagByRole(tags []TagSnapshot, role string) TagSnapshot {
	for _, tag := range tags {
		if tag.Role == role {
			return tag
		}
	}
	return TagSnapshot{Role: role}
}

func validateTagConcepts(tags []TagSnapshot) error {
	primary, secondary := tagByRole(tags, "PRIMARY").ID, tagByRole(tags, "SECONDARY").ID
	seen := map[string]bool{}
	for _, tag := range tags {
		if tag.Role != "TERTIARY" || tag.ID == "" {
			continue
		}
		if tag.ID == primary || tag.ID == secondary || seen[tag.ID] {
			return newServiceError("invalid_tag", http.StatusUnprocessableEntity, "tertiary tags must select distinct concepts outside the classification")
		}
		seen[tag.ID] = true
	}
	return nil
}
