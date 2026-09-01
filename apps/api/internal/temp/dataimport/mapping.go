package dataimport

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"heyblog-api/internal/domain/site"
)

var (
	uuidV7Pattern  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	slugPattern    = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	shortIDPattern = regexp.MustCompile(`^[0-9A-Za-z]{9}$`)
)

type shortIDGenerator func() (string, error)

type componentCandidate struct {
	ID             string
	Name           string
	NormalizedName string
	Description    string
	HomepageURL    string
	RepositoryURL  string
	IsOpenSource   bool
	IsEnabled      bool
	Priority       int
}

func BuildPlan(bundles Bundles, generateShortID shortIDGenerator) (Plan, error) {
	if generateShortID == nil {
		return Plan{}, fmt.Errorf("short ID generator is required")
	}
	plan := Plan{
		Sources: []SourceRow{
			{Key: "HEYBLOG_OLD", Name: "Legacy HeyBlog classification data"},
			{Key: "WEB_SUBMIT", Name: "Web submission"},
			{Key: "ZHBLOGS_OLD", Name: "Legacy ZHBlogs directory data"},
		},
	}
	usedShortIDs := make(map[string]struct{}, len(bundles.Blogs.Blogs))
	siteIDByHost := make(map[string]string, len(bundles.Blogs.Blogs))
	tags := make(map[string]TagRow)
	componentCandidates := make(map[string][]componentCandidate)

	for index, blog := range bundles.Blogs.Blogs {
		if err := validateUUIDv7(blog.ID, fmt.Sprintf("blogs[%d].id", index)); err != nil {
			return Plan{}, err
		}
		address, err := site.NormalizeAddress(blog.URL)
		if err != nil {
			return Plan{}, fmt.Errorf("normalize blogs[%d].url: %w", index, err)
		}
		if _, exists := siteIDByHost[address.NormalizedHost]; exists {
			return Plan{}, fmt.Errorf("blogs[%d] duplicates normalized host %q", index, address.NormalizedHost)
		}
		shortID, err := uniqueShortID(generateShortID, usedShortIDs)
		if err != nil {
			return Plan{}, fmt.Errorf("generate blogs[%d] short ID: %w", index, err)
		}
		joinedAt, err := parseImportTime(blog.JoinedAt, fmt.Sprintf("blogs[%d].joined_at", index))
		if err != nil {
			return Plan{}, err
		}
		updatedAt, err := parseImportTime(blog.UpdatedAt, fmt.Sprintf("blogs[%d].updated_at", index))
		if err != nil {
			return Plan{}, err
		}
		if updatedAt.Before(joinedAt) {
			return Plan{}, fmt.Errorf("blogs[%d] timestamps violate joined_at <= updated_at", index)
		}
		plan.Sites = append(plan.Sites, SiteRow{
			ID: blog.ID, ShortID: shortID, Name: strings.TrimSpace(blog.Name),
			Scheme: address.Scheme, NormalizedHost: address.NormalizedHost, BasePath: address.BasePath,
			Summary: strings.TrimSpace(blog.Summary), AccessScope: blog.AccessScope,
			Visibility: blog.Visibility, VisibilityReason: valueOrEmpty(blog.VisibilityReason),
			JoinedAt: joinedAt, UpdatedAt: updatedAt,
		})
		siteIDByHost[address.NormalizedHost] = blog.ID
		for feedIndex, feed := range blog.Feeds {
			location, normalizeErr := site.NormalizeLocation(feed.URL, address, false)
			if normalizeErr != nil {
				return Plan{}, fmt.Errorf("normalize blogs[%d].feed[%d]: %w", index, feedIndex, normalizeErr)
			}
			format, formatErr := feedFormat(feed.Format)
			if formatErr != nil {
				return Plan{}, fmt.Errorf("blogs[%d].feed[%d]: %w", index, feedIndex, formatErr)
			}
			plan.Feeds = append(plan.Feeds, FeedRow{
				SiteID: blog.ID, Name: strings.TrimSpace(feed.Name), LocationType: location.Type,
				URLRef: location.URLRef, ExternalURL: location.ExternalURL, URLKey: location.URLKey,
				Format: format, IsEnabled: true, IsDefault: feed.IsDefault,
			})
		}
		resourceKeys := make(map[string]string, 2)
		for _, resource := range []struct {
			kind string
			raw  *string
		}{{kind: "LINK_PAGE", raw: blog.LinkPage}, {kind: "SITEMAP", raw: blog.Sitemap}} {
			if resource.raw == nil {
				continue
			}
			location, normalizeErr := site.NormalizeLocation(*resource.raw, address, false)
			if normalizeErr != nil {
				return Plan{}, fmt.Errorf("normalize blogs[%d].%s: %w", index, strings.ToLower(resource.kind), normalizeErr)
			}
			if existingKind, exists := resourceKeys[location.URLKey]; exists {
				return Plan{}, fmt.Errorf("blogs[%d] resources %s and %s normalize to the same location", index, existingKind, resource.kind)
			}
			resourceKeys[location.URLKey] = resource.kind
			plan.Resources = append(plan.Resources, ResourceRow{
				SiteID: blog.ID, Kind: resource.kind, LocationType: location.Type,
				URLRef: location.URLRef, ExternalURL: location.ExternalURL, URLKey: location.URLKey,
			})
		}
		if blog.MainTag != nil {
			if err := addTag(&plan, tags, blog.ID, *blog.MainTag, "PRIMARY"); err != nil {
				return Plan{}, fmt.Errorf("map blogs[%d].main_tag: %w", index, err)
			}
		}
		for tagIndex, tag := range blog.SubTags {
			if err := addTag(&plan, tags, blog.ID, tag, "SECONDARY"); err != nil {
				return Plan{}, fmt.Errorf("map blogs[%d].sub_tags[%d]: %w", index, tagIndex, err)
			}
		}
		if blog.Architecture != nil {
			architecture := blog.Architecture
			if err := validateUUIDv7(architecture.Program.ID, fmt.Sprintf("blogs[%d].architecture.program.id", index)); err != nil {
				return Plan{}, err
			}
			programKey := normalizedComponentName(architecture.Program.NormalizedName)
			if programKey == "" {
				return Plan{}, fmt.Errorf("blogs[%d].architecture.program normalized name is empty", index)
			}
			componentCandidates[programKey] = append(componentCandidates[programKey], componentCandidate{
				ID: architecture.Program.ID, Name: architecture.Program.Name,
				NormalizedName: programKey, HomepageURL: valueOrEmpty(architecture.Program.WebsiteURL),
				RepositoryURL: valueOrEmpty(architecture.Program.RepositoryURL),
				IsOpenSource:  architecture.Program.IsOpenSource, IsEnabled: architecture.Program.IsEnabled,
				Priority: 3,
			})
			plan.SiteComponents = append(plan.SiteComponents, SiteComponentRow{
				SiteID: blog.ID, ComponentID: programKey, Role: "SITE_PROGRAM", IdentifiedAt: joinedAt,
			})
			for stackIndex, stack := range architecture.TechnologyStacks {
				if err := validateUUIDv7(stack.ID, fmt.Sprintf("blogs[%d].architecture.technology_stacks[%d].id", index, stackIndex)); err != nil {
					return Plan{}, err
				}
				stackKey := normalizedComponentName(stack.NormalizedName)
				role, roleErr := dependencyRole(stack.Category)
				if roleErr != nil {
					return Plan{}, fmt.Errorf("blogs[%d].architecture.technology_stacks[%d]: %w", index, stackIndex, roleErr)
				}
				componentCandidates[stackKey] = append(componentCandidates[stackKey], componentCandidate{
					ID: stack.ID, Name: stack.Name, NormalizedName: stackKey, IsEnabled: true, Priority: 1,
				})
				if stack.Catalog != nil {
					if err := validateUUIDv7(stack.Catalog.ID, fmt.Sprintf("blogs[%d].architecture.technology_stacks[%d].catalog.id", index, stackIndex)); err != nil {
						return Plan{}, err
					}
					catalogKey := normalizedComponentName(stack.Catalog.NormalizedName)
					if catalogKey != stackKey {
						return Plan{}, fmt.Errorf("blogs[%d] stack and catalog normalized names differ", index)
					}
					componentCandidates[stackKey] = append(componentCandidates[stackKey], componentCandidate{
						ID: stack.Catalog.ID, Name: stack.Catalog.Name, NormalizedName: stackKey,
						Description: valueOrEmpty(stack.Catalog.Description), HomepageURL: valueOrEmpty(stack.Catalog.OfficialURL),
						IsEnabled: stack.Catalog.IsEnabled, Priority: 2,
					})
				}
				if stackKey == programKey {
					return Plan{}, fmt.Errorf("blogs[%d] contains a self software dependency", index)
				}
				plan.Dependencies = append(plan.Dependencies, DependencyRow{
					ComponentID: programKey, DependencyComponentID: stackKey, Role: role,
				})
			}
		}
		for originIndex, origin := range blog.Origins {
			firstDiscoveredAt, parseErr := parseImportTime(origin.FirstDiscoveredAt, fmt.Sprintf("blogs[%d].origins[%d].first_discovered_at", index, originIndex))
			if parseErr != nil {
				return Plan{}, parseErr
			}
			metadata, marshalErr := json.Marshal(origin.Metadata)
			if marshalErr != nil {
				return Plan{}, fmt.Errorf("marshal blogs[%d].origins[%d].metadata: %w", index, originIndex, marshalErr)
			}
			plan.Origins = append(plan.Origins, OriginRow{
				SiteID: blog.ID, SourceKey: origin.SourceKey, ExternalReference: origin.ExternalReference,
				FirstDiscoveredAt: firstDiscoveredAt, Metadata: metadata,
			})
		}
	}

	componentIDByName := make(map[string]string, len(componentCandidates))
	tagIDs := make([]string, 0, len(tags))
	for id := range tags {
		tagIDs = append(tagIDs, id)
	}
	sort.Strings(tagIDs)
	for _, id := range tagIDs {
		plan.Tags = append(plan.Tags, tags[id])
	}
	componentKeys := make([]string, 0, len(componentCandidates))
	for key := range componentCandidates {
		componentKeys = append(componentKeys, key)
	}
	sort.Strings(componentKeys)
	for _, key := range componentKeys {
		component := mergeComponentCandidates(componentCandidates[key])
		plan.Components = append(plan.Components, component)
		componentIDByName[key] = component.ID
	}
	for index := range plan.Dependencies {
		plan.Dependencies[index].ComponentID = componentIDByName[plan.Dependencies[index].ComponentID]
		plan.Dependencies[index].DependencyComponentID = componentIDByName[plan.Dependencies[index].DependencyComponentID]
	}
	plan.Dependencies = uniqueDependencies(plan.Dependencies)
	for index := range plan.SiteComponents {
		plan.SiteComponents[index].ComponentID = componentIDByName[plan.SiteComponents[index].ComponentID]
	}

	graphHosts := make(map[string]struct{})
	for sourceIndex, source := range bundles.Graph.Links {
		siteID, exists := siteIDByHost[source.Source]
		if !exists {
			return Plan{}, fmt.Errorf("graph.links[%d] references unknown source host", sourceIndex)
		}
		graphHosts[source.Source] = struct{}{}
		for destinationIndex, raw := range source.Destinations {
			target, err := site.NormalizeAddress(raw)
			if err != nil || target.CanonicalURL() != raw {
				return Plan{}, fmt.Errorf("graph.links[%d].destinations[%d] is not normalized", sourceIndex, destinationIndex)
			}
			if _, targetExists := siteIDByHost[target.NormalizedHost]; !targetExists {
				return Plan{}, fmt.Errorf("graph.links[%d].destinations[%d] references unknown target host", sourceIndex, destinationIndex)
			}
			graphHosts[target.NormalizedHost] = struct{}{}
			plan.FriendLinks = append(plan.FriendLinks, FriendLinkRow{
				SourceSiteID: siteID, TargetURL: target.CanonicalURL(), TargetHost: target.NormalizedHost,
			})
		}
	}
	if len(graphHosts) != bundles.Graph.NodeCount || len(plan.FriendLinks) != bundles.Graph.EdgeCount {
		return Plan{}, fmt.Errorf("graph node or edge counts are inconsistent after normalization")
	}

	sortPlan(&plan)
	if err := validatePlan(plan); err != nil {
		return Plan{}, fmt.Errorf("validate import plan: %w", err)
	}
	return plan, nil
}

func uniqueShortID(generate shortIDGenerator, used map[string]struct{}) (string, error) {
	for range site.ShortIDCollisionRetries {
		value, err := generate()
		if err != nil {
			return "", err
		}
		if !shortIDPattern.MatchString(value) {
			return "", fmt.Errorf("generator returned an invalid short ID")
		}
		if _, exists := used[value]; exists {
			continue
		}
		used[value] = struct{}{}
		return value, nil
	}
	return "", fmt.Errorf("short ID collision retry limit reached")
}

func validateUUIDv7(value, field string) error {
	if !uuidV7Pattern.MatchString(strings.ToLower(value)) {
		return fmt.Errorf("%s must be a UUIDv7", field)
	}
	return nil
}

func parseImportTime(value, field string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be an RFC3339 timestamp", field)
	}
	return parsed, nil
}

func feedFormat(value string) (string, error) {
	format := strings.ToUpper(strings.TrimSpace(value))
	if !slices.Contains([]string{"UNKNOWN", "RSS", "ATOM", "JSON"}, format) {
		return "", fmt.Errorf("unsupported feed format %q", format)
	}
	return format, nil
}

func addTag(plan *Plan, tags map[string]TagRow, siteID string, legacy LegacyTag, role string) error {
	if err := validateUUIDv7(legacy.ID, "tag id"); err != nil {
		return err
	}
	name := strings.TrimSpace(legacy.Name)
	if name == "" {
		return fmt.Errorf("tag name is empty")
	}
	normalizedName := strings.ToLower(name)
	slug := "legacy-" + strings.ToLower(legacy.ID)
	if legacy.MachineKey != nil {
		candidate := strings.ToLower(strings.TrimSpace(*legacy.MachineKey))
		if slugPattern.MatchString(candidate) {
			slug = candidate
		}
	}
	description := valueOrEmpty(legacy.Description)
	row := TagRow{ID: legacy.ID, Name: name, NormalizedName: normalizedName, Slug: slug, Description: description, IsEnabled: legacy.IsEnabled}
	if existing, exists := tags[legacy.ID]; exists && existing != row {
		return fmt.Errorf("tag %s has inconsistent definitions", legacy.ID)
	}
	tags[legacy.ID] = row
	plan.SiteTags = append(plan.SiteTags, SiteTagRow{SiteID: siteID, TagID: legacy.ID, Role: role})
	return nil
}

func dependencyRole(category string) (string, error) {
	role := strings.ToUpper(strings.TrimSpace(category))
	if role != "FRAMEWORK" && role != "LANGUAGE" {
		return "", fmt.Errorf("unsupported dependency category %q", category)
	}
	return role, nil
}

func normalizedComponentName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func mergeComponentCandidates(candidates []componentCandidate) ComponentRow {
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].Priority != candidates[right].Priority {
			return candidates[left].Priority > candidates[right].Priority
		}
		return candidates[left].ID < candidates[right].ID
	})
	canonical := candidates[0]
	row := ComponentRow{
		ID: canonical.ID, Name: strings.TrimSpace(canonical.Name), NormalizedName: canonical.NormalizedName,
		Description: canonical.Description, HomepageURL: canonical.HomepageURL,
		RepositoryURL: canonical.RepositoryURL, IsOpenSource: canonical.IsOpenSource,
		IsEnabled: canonical.IsEnabled,
	}
	for _, candidate := range candidates {
		if row.Description == "" && candidate.Description != "" {
			row.Description = candidate.Description
		}
		if row.HomepageURL == "" && candidate.HomepageURL != "" {
			row.HomepageURL = candidate.HomepageURL
		}
		if row.RepositoryURL == "" && candidate.RepositoryURL != "" {
			row.RepositoryURL = candidate.RepositoryURL
		}
	}
	return row
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

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
		return plan.SiteTags[i].SiteID+plan.SiteTags[i].Role+plan.SiteTags[i].TagID < plan.SiteTags[j].SiteID+plan.SiteTags[j].Role+plan.SiteTags[j].TagID
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
