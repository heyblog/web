package siteaudit

import (
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"net/http"
)

func canonicalTag(tag TagSnapshot, canonical dbgen.DirectoryTag, allowDisabled bool) (TagSnapshot, error) {
	if !allowDisabled && !canonical.IsEnabled {
		return TagSnapshot{}, newServiceError("invalid_tag", http.StatusUnprocessableEntity, "the selected tag is no longer available")
	}
	tag.ID, _ = uuidString(canonical.ID)
	tag.Name, tag.Description = canonical.Name, canonical.Description
	tag.SuggestedName = ""
	tag.historicalLabelID = ""
	tag.unresolved = false
	return tag, nil
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
			return newServiceError("invalid_tag", http.StatusUnprocessableEntity, "tertiary tags must select distinct tags outside the classification")
		}
		seen[tag.ID] = true
	}
	return nil
}
