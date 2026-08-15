package dataimport

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

func validatePlan(plan Plan) error {
	siteIDs := make(map[string]SiteRow, len(plan.Sites))
	hosts := make(map[string]string, len(plan.Sites))
	shortIDs := make(map[string]struct{}, len(plan.Sites))
	for _, row := range plan.Sites {
		if _, exists := siteIDs[row.ID]; exists {
			return fmt.Errorf("duplicate site id %q", row.ID)
		}
		if _, exists := hosts[row.NormalizedHost]; exists {
			return fmt.Errorf("duplicate site host %q", row.NormalizedHost)
		}
		if _, exists := shortIDs[row.ShortID]; exists {
			return fmt.Errorf("duplicate site short id %q", row.ShortID)
		}
		if strings.TrimSpace(row.Name) == "" || strings.TrimSpace(row.Summary) == "" ||
			!slices.Contains([]string{"ALL", "CN_ONLY", "GLOBAL_ONLY"}, row.AccessScope) ||
			!slices.Contains([]string{"VISIBLE", "HIDDEN"}, row.Visibility) ||
			row.Visibility == "VISIBLE" && row.VisibilityReason != "" ||
			row.Visibility == "HIDDEN" && strings.TrimSpace(row.VisibilityReason) == "" {
			return fmt.Errorf("site %q contains invalid profile data", row.ID)
		}
		if row.JoinedAt.Before(row.CreatedAt) || row.UpdatedAt.Before(row.JoinedAt) || row.UpdatedAt.Before(row.CreatedAt) {
			return fmt.Errorf("site %q contains invalid timestamps", row.ID)
		}
		siteIDs[row.ID] = row
		hosts[row.NormalizedHost] = row.ID
		shortIDs[row.ShortID] = struct{}{}
	}

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
	primaryTags := make(map[string]int)
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
		if !slices.Contains([]string{"PRIMARY", "SECONDARY"}, row.TopicRole) {
			return fmt.Errorf("site tag has invalid role %q", row.TopicRole)
		}
		if row.TopicRole == "PRIMARY" {
			primaryTags[row.SiteID]++
			if primaryTags[row.SiteID] > 1 {
				return fmt.Errorf("site %q has multiple primary tags", row.SiteID)
			}
		}
		siteTagKeys[key] = struct{}{}
	}

	componentIDs := make(map[string]struct{}, len(plan.Components))
	componentNames := make(map[string]struct{}, len(plan.Components))
	for _, row := range plan.Components {
		if _, exists := componentIDs[row.ID]; exists {
			return fmt.Errorf("duplicate software component id %q", row.ID)
		}
		if _, exists := componentNames[row.NormalizedName]; exists {
			return fmt.Errorf("duplicate normalized component name %q", row.NormalizedName)
		}
		if strings.TrimSpace(row.Name) == "" || row.NormalizedName == "" {
			return fmt.Errorf("software component %q contains invalid identity data", row.ID)
		}
		if err := validateComponentURL(row.HomepageURL, "homepage"); err != nil {
			return fmt.Errorf("software component %q: %w", row.ID, err)
		}
		if err := validateComponentURL(row.RepositoryURL, "repository"); err != nil {
			return fmt.Errorf("software component %q: %w", row.ID, err)
		}
		componentIDs[row.ID] = struct{}{}
		componentNames[row.NormalizedName] = struct{}{}
	}
	dependencyKeys := make(map[DependencyRow]struct{}, len(plan.Dependencies))
	dependencyGraph := make(map[string][]string)
	for _, row := range plan.Dependencies {
		if _, exists := componentIDs[row.ComponentID]; !exists {
			return fmt.Errorf("dependency references unknown component %q", row.ComponentID)
		}
		if _, exists := componentIDs[row.DependencyComponentID]; !exists {
			return fmt.Errorf("dependency references unknown component %q", row.DependencyComponentID)
		}
		if row.ComponentID == row.DependencyComponentID {
			return fmt.Errorf("component %q depends on itself", row.ComponentID)
		}
		if !slices.Contains([]string{"FRAMEWORK", "LANGUAGE"}, row.Role) {
			return fmt.Errorf("dependency has invalid role %q", row.Role)
		}
		if _, exists := dependencyKeys[row]; exists {
			return fmt.Errorf("duplicate software dependency")
		}
		dependencyKeys[row] = struct{}{}
		dependencyGraph[row.ComponentID] = append(dependencyGraph[row.ComponentID], row.DependencyComponentID)
	}
	if hasDependencyCycle(dependencyGraph) {
		return fmt.Errorf("software dependency graph contains a cycle")
	}

	siteComponentKeys := make(map[string]struct{}, len(plan.SiteComponents))
	sitePrograms := make(map[string]int)
	for _, row := range plan.SiteComponents {
		if _, exists := siteIDs[row.SiteID]; !exists {
			return fmt.Errorf("site component references unknown site %q", row.SiteID)
		}
		if _, exists := componentIDs[row.ComponentID]; !exists {
			return fmt.Errorf("site component references unknown component %q", row.ComponentID)
		}
		if row.Role != "SITE_PROGRAM" {
			return fmt.Errorf("site component has invalid role %q", row.Role)
		}
		key := row.SiteID + "\x00" + row.ComponentID + "\x00" + row.Role
		if _, exists := siteComponentKeys[key]; exists {
			return fmt.Errorf("duplicate site component assignment")
		}
		sitePrograms[row.SiteID]++
		if sitePrograms[row.SiteID] > 1 {
			return fmt.Errorf("site %q has multiple site programs", row.SiteID)
		}
		siteComponentKeys[key] = struct{}{}
	}

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

func validateComponentURL(value, field string) error {
	if value == "" {
		return nil
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") ||
		!parsed.IsAbs() || parsed.Host == "" || parsed.User != nil ||
		parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s URL must be an absolute HTTP or HTTPS URL without credentials", field)
	}
	return nil
}

func hasDependencyCycle(graph map[string][]string) bool {
	states := make(map[string]uint8, len(graph))
	var visit func(string) bool
	visit = func(node string) bool {
		switch states[node] {
		case 1:
			return true
		case 2:
			return false
		}
		states[node] = 1
		for _, dependency := range graph[node] {
			if visit(dependency) {
				return true
			}
		}
		states[node] = 2
		return false
	}
	for node := range graph {
		if visit(node) {
			return true
		}
	}
	return false
}
