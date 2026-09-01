package dataimport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	blogsFormat     = "heyblog.data-import.blogs"
	graphFormat     = "heyblog.data-import.graph"
	contractVersion = 3
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Bundles struct {
	Blogs BlogBundle
	Graph GraphBundle
}

type BlogBundle struct {
	Format      string          `json:"format"`
	Version     int             `json:"version"`
	GeneratedAt string          `json:"generated_at"`
	Inputs      []InputMetadata `json:"inputs"`
	Count       int             `json:"count"`
	Blogs       []LegacyBlog    `json:"blogs"`
}

type InputMetadata struct {
	Kind   string `json:"kind"`
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	Count  int    `json:"count"`
}

type LegacyBlog struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	URL              string              `json:"url"`
	Summary          string              `json:"summary"`
	Feeds            []LegacyFeed        `json:"feeds"`
	Sitemap          *string             `json:"sitemap"`
	LinkPage         *string             `json:"link_page"`
	JoinedAt         string              `json:"joined_at"`
	UpdatedAt        string              `json:"updated_at"`
	AccessScope      string              `json:"access_scope"`
	Visibility       string              `json:"visibility"`
	VisibilityReason *string             `json:"visibility_reason"`
	Origins          []LegacyOrigin      `json:"origins"`
	MainTag          *LegacyTag          `json:"main_tag"`
	SubTags          []LegacyTag         `json:"sub_tags"`
	Architecture     *LegacyArchitecture `json:"architecture"`
}

type LegacyFeed struct {
	URL       string `json:"url"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	Format    string `json:"format"`
}

type LegacyOrigin struct {
	SourceKey         string         `json:"source_key"`
	ExternalReference string         `json:"external_reference"`
	FirstDiscoveredAt string         `json:"first_discovered_at"`
	Metadata          OriginMetadata `json:"metadata"`
}

type OriginMetadata struct {
	InputKinds         []string `json:"input_kinds"`
	ExternalReferences []string `json:"external_references"`
}

type LegacyTag struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	MachineKey  *string `json:"machine_key"`
	Description *string `json:"description"`
	IsEnabled   bool    `json:"is_enabled"`
}

type LegacyArchitecture struct {
	Program          LegacyProgram `json:"program"`
	TechnologyStacks []LegacyStack `json:"technology_stacks"`
}

type LegacyProgram struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	NormalizedName string  `json:"name_normalized"`
	IsOpenSource   bool    `json:"is_open_source"`
	WebsiteURL     *string `json:"website_url"`
	RepositoryURL  *string `json:"repo_url"`
	IsEnabled      bool    `json:"is_enabled"`
}

type LegacyStack struct {
	ID             string         `json:"id"`
	Category       string         `json:"category"`
	Name           string         `json:"name"`
	NormalizedName string         `json:"name_normalized"`
	Catalog        *LegacyCatalog `json:"catalog"`
}

type LegacyCatalog struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	NormalizedName string  `json:"name_normalized"`
	TechnologyType string  `json:"technology_type"`
	Description    *string `json:"description"`
	OfficialURL    *string `json:"official_url"`
	IsEnabled      bool    `json:"is_enabled"`
}

type GraphBundle struct {
	Format      string             `json:"format"`
	Version     int                `json:"version"`
	GeneratedAt string             `json:"generated_at"`
	Inputs      []InputMetadata    `json:"inputs"`
	NodeCount   int                `json:"node_count"`
	EdgeCount   int                `json:"edge_count"`
	Count       int                `json:"count"`
	Links       []FriendLinkSource `json:"links"`
}

type FriendLinkSource struct {
	Source       string   `json:"source"`
	Destinations []string `json:"destinations"`
}

func DecodeBundles(blogData, graphData []byte) (Bundles, error) {
	var bundles Bundles
	if err := decodeStrictJSON(blogData, &bundles.Blogs); err != nil {
		return Bundles{}, fmt.Errorf("decode blogs bundle: %w", err)
	}
	if err := decodeStrictJSON(graphData, &bundles.Graph); err != nil {
		return Bundles{}, fmt.Errorf("decode graph bundle: %w", err)
	}
	if err := validateCleanedJSONShape(blogData, graphData); err != nil {
		return Bundles{}, err
	}
	if err := validateBundles(bundles); err != nil {
		return Bundles{}, err
	}
	return bundles, nil
}

func decodeStrictJSON(data []byte, destination any) error {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON documents are not allowed")
		}
		return err
	}
	return nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := scanJSONValue(decoder, "$", false); err != nil {
		return err
	}
	if token, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("unexpected token after JSON document: %v", token)
		}
		return err
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder, path string, tokenAlreadyRead bool) error {
	var token json.Token
	var err error
	if tokenAlreadyRead {
		return errors.New("internal JSON scanner misuse")
	}
	token, err = decoder.Token()
	if err != nil {
		return err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, keyErr := decoder.Token()
			if keyErr != nil {
				return keyErr
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("%s object key must be a string", path)
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("%s contains duplicate key %q", path, key)
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder, path+"."+key, false); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		index := 0
		for decoder.More() {
			if err := scanJSONValue(decoder, fmt.Sprintf("%s[%d]", path, index), false); err != nil {
				return err
			}
			index++
		}
		_, err = decoder.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
}

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
