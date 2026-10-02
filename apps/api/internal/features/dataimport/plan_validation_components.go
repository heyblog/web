package dataimport

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

func validatePlanComponents(plan Plan, siteIDs map[string]SiteRow) error {
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
