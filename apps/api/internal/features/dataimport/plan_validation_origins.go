package dataimport

import (
	"fmt"
	"strings"
)

func validatePlanOrigins(plan Plan, siteIDs map[string]SiteRow) error {
	sourceKeys := make(map[string]struct{}, len(plan.Sources))
	for _, row := range plan.Sources {
		if strings.TrimSpace(row.Key) == "" || strings.TrimSpace(row.Name) == "" {
			return fmt.Errorf("source contains empty identity data")
		}
		if _, exists := sourceKeys[row.Key]; exists {
			return fmt.Errorf("duplicate source key %q", row.Key)
		}
		sourceKeys[row.Key] = struct{}{}
	}
	originKeys := make(map[string]struct{}, len(plan.Origins))
	originsPerSite := make(map[string]int)
	for _, row := range plan.Origins {
		if _, exists := siteIDs[row.SiteID]; !exists {
			return fmt.Errorf("origin references unknown site %q", row.SiteID)
		}
		if _, exists := sourceKeys[row.SourceKey]; !exists {
			return fmt.Errorf("origin references unknown source %q", row.SourceKey)
		}
		key := row.SiteID + "\x00" + row.SourceKey
		if _, exists := originKeys[key]; exists {
			return fmt.Errorf("duplicate site origin")
		}
		originKeys[key] = struct{}{}
		originsPerSite[row.SiteID]++
	}
	for siteID := range siteIDs {
		if originsPerSite[siteID] == 0 {
			return fmt.Errorf("site %q must have at least one origin", siteID)
		}
	}
	return nil
}
