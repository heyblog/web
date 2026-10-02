package dataimport

import (
	"fmt"
	"slices"
	"strings"
)

func validatePlanTags(plan Plan, siteIDs map[string]SiteRow) error {
	tagIDs := make(map[string]struct{}, len(plan.Tags))
	tagNames := make(map[string]struct{}, len(plan.Tags))
	tagSlugs := make(map[string]struct{}, len(plan.Tags))
	for _, row := range plan.Tags {
		if _, exists := tagIDs[row.ID]; exists {
			return fmt.Errorf("duplicate tag id %q", row.ID)
		}
		if _, exists := tagNames[row.NormalizedName]; exists {
			return fmt.Errorf("duplicate normalized tag name %q", row.NormalizedName)
		}
		if _, exists := tagSlugs[row.Slug]; exists {
			return fmt.Errorf("duplicate tag slug %q", row.Slug)
		}
		if strings.TrimSpace(row.Name) == "" || row.NormalizedName == "" || !slugPattern.MatchString(row.Slug) {
			return fmt.Errorf("tag %q contains invalid identity data", row.ID)
		}
		tagIDs[row.ID] = struct{}{}
		tagNames[row.NormalizedName] = struct{}{}
		tagSlugs[row.Slug] = struct{}{}
	}
	siteTagKeys := make(map[string]struct{}, len(plan.SiteTags))
	positions := make(map[string]map[int16]struct{})
	primarySites := make(map[string]struct{})
	for _, row := range plan.SiteTags {
		if _, exists := siteIDs[row.SiteID]; !exists {
			return fmt.Errorf("site tag references unknown site %q", row.SiteID)
		}
		if _, exists := tagIDs[row.TagID]; !exists {
			return fmt.Errorf("site tag references unknown tag %q", row.TagID)
		}
		key := row.SiteID + "\x00" + row.TagID
		if _, exists := siteTagKeys[key]; exists {
			return fmt.Errorf("duplicate site tag assignment %q", key)
		}
		if !slices.Contains([]string{"PRIMARY", "SECONDARY", "TERTIARY", "WARNING"}, row.Role) {
			return fmt.Errorf("site tag has invalid role %q", row.Role)
		}
		if row.Role == "TERTIARY" && (row.Position < 1 || row.Position > 20) || row.Role != "TERTIARY" && row.Position != 0 {
			return fmt.Errorf("site tag %q has invalid position %d", row.Role, row.Position)
		}
		if row.Role == "PRIMARY" {
			if _, exists := primarySites[row.SiteID]; exists {
				return fmt.Errorf("site %q has multiple primary tags", row.SiteID)
			}
			primarySites[row.SiteID] = struct{}{}
		}
		if row.Note != "" && strings.TrimSpace(row.Note) == "" {
			return fmt.Errorf("site tag note must not contain only whitespace")
		}
		if row.Role != "TERTIARY" {
			siteTagKeys[key] = struct{}{}
			continue
		}
		if positions[row.SiteID] == nil {
			positions[row.SiteID] = make(map[int16]struct{})
		}
		if _, exists := positions[row.SiteID][row.Position]; exists {
			return fmt.Errorf("site %q has duplicate tertiary tag position %d", row.SiteID, row.Position)
		}
		positions[row.SiteID][row.Position] = struct{}{}
		siteTagKeys[key] = struct{}{}
	}
	return nil
}
