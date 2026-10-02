package dataimport

import (
	"fmt"
)

func validatePlanFriendLinks(plan Plan, sites planSiteIndex) error {
	siteIDs, hosts := sites.byID, sites.byHost
	friendKeys := make(map[string]struct{}, len(plan.FriendLinks))
	for _, row := range plan.FriendLinks {
		sourceSite, exists := siteIDs[row.SourceSiteID]
		if !exists {
			return fmt.Errorf("friend link references unknown source site %q", row.SourceSiteID)
		}
		if row.TargetURL == "" || row.TargetHost == "" || row.TargetHost == sourceSite.NormalizedHost {
			return fmt.Errorf("friend link for site %q has an invalid target", row.SourceSiteID)
		}
		if _, exists := hosts[row.TargetHost]; !exists {
			return fmt.Errorf("friend link references unknown target host %q", row.TargetHost)
		}
		key := row.SourceSiteID + "\x00" + row.TargetHost
		if _, exists := friendKeys[key]; exists {
			return fmt.Errorf("site %q has duplicate friend target host %q", row.SourceSiteID, row.TargetHost)
		}
		friendKeys[key] = struct{}{}
	}
	return nil
}
