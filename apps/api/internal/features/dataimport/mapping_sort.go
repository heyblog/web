package dataimport

import "sort"

func sortPlan(plan *Plan) {
	sort.Slice(plan.Sites, func(i, j int) bool { return plan.Sites[i].ID < plan.Sites[j].ID })
	sort.Slice(plan.Feeds, func(i, j int) bool {
		return plan.Feeds[i].SiteID+plan.Feeds[i].URLKey < plan.Feeds[j].SiteID+plan.Feeds[j].URLKey
	})
	sort.Slice(plan.Resources, func(i, j int) bool {
		return plan.Resources[i].SiteID+plan.Resources[i].Kind < plan.Resources[j].SiteID+plan.Resources[j].Kind
	})
	sort.Slice(plan.Tags, func(i, j int) bool { return plan.Tags[i].ID < plan.Tags[j].ID })
	sort.Slice(plan.SiteTags, func(i, j int) bool {
		left, right := plan.SiteTags[i], plan.SiteTags[j]
		if left.SiteID != right.SiteID {
			return left.SiteID < right.SiteID
		}
		return left.Position < right.Position
	})
	sort.Slice(plan.Dependencies, func(i, j int) bool {
		left := plan.Dependencies[i]
		right := plan.Dependencies[j]
		return left.ComponentID+left.Role+left.DependencyComponentID < right.ComponentID+right.Role+right.DependencyComponentID
	})
	sort.Slice(plan.SiteComponents, func(i, j int) bool {
		return plan.SiteComponents[i].SiteID < plan.SiteComponents[j].SiteID
	})
	sort.Slice(plan.Origins, func(i, j int) bool {
		left := plan.Origins[i]
		right := plan.Origins[j]
		return left.SiteID+left.SourceKey < right.SiteID+right.SourceKey
	})
	sort.Slice(plan.FriendLinks, func(i, j int) bool {
		return plan.FriendLinks[i].SourceSiteID+plan.FriendLinks[i].TargetHost < plan.FriendLinks[j].SourceSiteID+plan.FriendLinks[j].TargetHost
	})
}

func uniqueDependencies(rows []DependencyRow) []DependencyRow {
	seen := make(map[DependencyRow]struct{}, len(rows))
	result := make([]DependencyRow, 0, len(rows))
	for _, row := range rows {
		if _, exists := seen[row]; exists {
			continue
		}
		seen[row] = struct{}{}
		result = append(result, row)
	}
	return result
}
