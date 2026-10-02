package dataimport

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"heyblog-api/internal/domain/site"
)

type shortIDGenerator func() (string, error)

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
			if err := addTag(&plan, tags, blog.ID, *blog.MainTag, "PRIMARY", 0); err != nil {
				return Plan{}, fmt.Errorf("map blogs[%d].main_tag: %w", index, err)
			}
		}
		for tagIndex, tag := range blog.SubTags {
			position := int16(tagIndex + 2)
			if position > 20 {
				break
			}
			if err := addTag(&plan, tags, blog.ID, tag, "SECONDARY", 0); err != nil {
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
