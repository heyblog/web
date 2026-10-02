package dataimport

import (
	"fmt"
)

func validatePlanLocations(plan Plan, siteIDs map[string]SiteRow) error {
	feedKeys := make(map[string]struct{}, len(plan.Feeds))
	feedDefaults := make(map[string]int)
	feedCounts := make(map[string]int)
	for _, row := range plan.Feeds {
		if _, exists := siteIDs[row.SiteID]; !exists {
			return fmt.Errorf("feed references unknown site %q", row.SiteID)
		}
		key := row.SiteID + "\x00" + row.URLKey
		if row.URLKey == "" {
			return fmt.Errorf("feed for site %q has an empty URL key", row.SiteID)
		}
		if _, exists := feedKeys[key]; exists {
			return fmt.Errorf("site %q has duplicate feed location %q", row.SiteID, row.URLKey)
		}
		feedKeys[key] = struct{}{}
		feedCounts[row.SiteID]++
		if row.IsDefault {
			feedDefaults[row.SiteID]++
		}
	}
	for siteID, count := range feedCounts {
		if count > 0 && feedDefaults[siteID] != 1 {
			return fmt.Errorf("site %q must have exactly one default feed", siteID)
		}
	}

	resourceKinds := make(map[string]struct{}, len(plan.Resources))
	resourceKeys := make(map[string]struct{}, len(plan.Resources))
	for _, row := range plan.Resources {
		if _, exists := siteIDs[row.SiteID]; !exists {
			return fmt.Errorf("resource references unknown site %q", row.SiteID)
		}
		kindKey := row.SiteID + "\x00" + row.Kind
		urlKey := row.SiteID + "\x00" + row.URLKey
		if row.URLKey == "" {
			return fmt.Errorf("resource for site %q has an empty URL key", row.SiteID)
		}
		if _, exists := resourceKinds[kindKey]; exists {
			return fmt.Errorf("site %q has duplicate resource kind %q", row.SiteID, row.Kind)
		}
		if _, exists := resourceKeys[urlKey]; exists {
			return fmt.Errorf("site %q has duplicate resource location %q", row.SiteID, row.URLKey)
		}
		resourceKinds[kindKey] = struct{}{}
		resourceKeys[urlKey] = struct{}{}
	}
	return nil
}
