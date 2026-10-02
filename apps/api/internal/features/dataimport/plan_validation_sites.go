package dataimport

import (
	"fmt"
	"slices"
	"strings"
)

type planSiteIndex struct {
	byID   map[string]SiteRow
	byHost map[string]string
}

func validatePlanSites(plan Plan) (planSiteIndex, error) {
	siteIDs := make(map[string]SiteRow, len(plan.Sites))
	hosts := make(map[string]string, len(plan.Sites))
	shortIDs := make(map[string]struct{}, len(plan.Sites))
	for _, row := range plan.Sites {
		if _, exists := siteIDs[row.ID]; exists {
			return planSiteIndex{}, fmt.Errorf("duplicate site id %q", row.ID)
		}
		if _, exists := hosts[row.NormalizedHost]; exists {
			return planSiteIndex{}, fmt.Errorf("duplicate site host %q", row.NormalizedHost)
		}
		if _, exists := shortIDs[row.ShortID]; exists {
			return planSiteIndex{}, fmt.Errorf("duplicate site short id %q", row.ShortID)
		}
		if strings.TrimSpace(row.Name) == "" ||
			!slices.Contains([]string{"ALL", "CN_ONLY", "GLOBAL_ONLY"}, row.AccessScope) ||
			!slices.Contains([]string{"VISIBLE", "HIDDEN"}, row.Visibility) ||
			row.Visibility == "VISIBLE" && row.VisibilityReason != "" ||
			row.Visibility == "HIDDEN" && strings.TrimSpace(row.VisibilityReason) == "" {
			return planSiteIndex{}, fmt.Errorf("site %q contains invalid profile data", row.ID)
		}
		if row.UpdatedAt.Before(row.JoinedAt) {
			return planSiteIndex{}, fmt.Errorf("site %q contains invalid timestamps", row.ID)
		}
		siteIDs[row.ID] = row
		hosts[row.NormalizedHost] = row.ID
		shortIDs[row.ShortID] = struct{}{}
	}
	return planSiteIndex{byID: siteIDs, byHost: hosts}, nil
}
