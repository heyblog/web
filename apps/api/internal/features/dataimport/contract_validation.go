package dataimport

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func validateBundles(bundles Bundles) error {
	blogs := bundles.Blogs
	if blogs.Format != blogsFormat || blogs.Version != contractVersion {
		return errors.New("unsupported cleaned blogs format or version")
	}
	if _, err := time.Parse(time.RFC3339Nano, blogs.GeneratedAt); err != nil {
		return errors.New("cleaned blogs generated_at is not RFC3339")
	}
	if err := validateInputMetadata(blogs.Inputs); err != nil {
		return fmt.Errorf("invalid cleaned blogs input metadata: %w", err)
	}
	if blogs.Count == 0 || blogs.Count != len(blogs.Blogs) {
		return errors.New("cleaned blogs counts are inconsistent")
	}
	for index, blog := range blogs.Blogs {
		if strings.TrimSpace(blog.ID) == "" || strings.TrimSpace(blog.Name) == "" ||
			strings.TrimSpace(blog.URL) == "" {
			return fmt.Errorf("blogs[%d] is missing required migration data", index)
		}
		if !slices.Contains([]string{"ALL", "CN_ONLY", "GLOBAL_ONLY"}, blog.AccessScope) {
			return fmt.Errorf("blogs[%d].access_scope is unsupported", index)
		}
		if !slices.Contains([]string{"VISIBLE", "HIDDEN"}, blog.Visibility) ||
			blog.Visibility == "VISIBLE" && blog.VisibilityReason != nil ||
			blog.Visibility == "HIDDEN" && (blog.VisibilityReason == nil || strings.TrimSpace(*blog.VisibilityReason) == "") {
			return fmt.Errorf("blogs[%d] has invalid visibility state", index)
		}
		joinedAt, joinedErr := time.Parse(time.RFC3339Nano, blog.JoinedAt)
		updatedAt, updatedErr := time.Parse(time.RFC3339Nano, blog.UpdatedAt)
		if joinedErr != nil || updatedErr != nil || updatedAt.Before(joinedAt) {
			return fmt.Errorf("blogs[%d] has invalid timestamps", index)
		}
		defaults := 0
		for feedIndex, feed := range blog.Feeds {
			if strings.TrimSpace(feed.URL) == "" || strings.TrimSpace(feed.Name) == "" ||
				!slices.Contains([]string{"UNKNOWN", "RSS", "ATOM", "JSON"}, feed.Format) {
				return fmt.Errorf("blogs[%d].feed[%d] is incomplete", index, feedIndex)
			}
			if feed.IsDefault {
				defaults++
			}
		}
		if len(blog.Feeds) > 0 && defaults != 1 {
			return fmt.Errorf("blogs[%d] must have exactly one default feed", index)
		}
		if len(blog.Origins) == 0 {
			return fmt.Errorf("blogs[%d] has no origins", index)
		}
		seenOrigins := make(map[string]struct{}, len(blog.Origins))
		for originIndex, origin := range blog.Origins {
			if !slices.Contains([]string{"HEYBLOG_OLD", "ZHBLOGS_OLD", "WEB_SUBMIT"}, origin.SourceKey) ||
				strings.TrimSpace(origin.ExternalReference) == "" || len(origin.Metadata.InputKinds) == 0 ||
				len(origin.Metadata.ExternalReferences) == 0 {
				return fmt.Errorf("blogs[%d].origins[%d] is incomplete", index, originIndex)
			}
			if _, err := time.Parse(time.RFC3339Nano, origin.FirstDiscoveredAt); err != nil {
				return fmt.Errorf("blogs[%d].origins[%d] has invalid first_discovered_at", index, originIndex)
			}
			if _, exists := seenOrigins[origin.SourceKey]; exists {
				return fmt.Errorf("blogs[%d] has duplicate origin source %q", index, origin.SourceKey)
			}
			seenOrigins[origin.SourceKey] = struct{}{}
		}
	}

	graph := bundles.Graph
	if graph.Format != graphFormat || graph.Version != contractVersion {
		return errors.New("unsupported cleaned graph format or version")
	}
	if graph.GeneratedAt != blogs.GeneratedAt || !reflect.DeepEqual(graph.Inputs, blogs.Inputs) {
		return errors.New("cleaned bundle generation metadata differs")
	}
	if graph.Count != len(graph.Links) || graph.NodeCount < 0 || graph.EdgeCount < 0 {
		return errors.New("cleaned graph counts are inconsistent")
	}
	edges := 0
	for index, link := range graph.Links {
		if strings.TrimSpace(link.Source) == "" || len(link.Destinations) == 0 {
			return fmt.Errorf("graph.links[%d] is incomplete", index)
		}
		edges += len(link.Destinations)
	}
	if edges != graph.EdgeCount {
		return errors.New("cleaned graph edge_count does not match destinations")
	}
	return nil
}

func validateInputMetadata(inputs []InputMetadata) error {
	expected := []string{"zhblogs", "classification"}
	if len(inputs) != len(expected) {
		return errors.New("exactly two input records are required")
	}
	for index, input := range inputs {
		if input.Kind != expected[index] || strings.TrimSpace(input.File) == "" ||
			!sha256Pattern.MatchString(input.SHA256) || input.Count < 0 {
			return fmt.Errorf("inputs[%d] is invalid", index)
		}
	}
	return nil
}
