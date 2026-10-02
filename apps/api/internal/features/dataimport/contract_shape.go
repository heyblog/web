package dataimport

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func validateCleanedJSONShape(blogData, graphData []byte) error {
	blogs, err := requiredObject(blogData, "$", []string{"format", "version", "generated_at", "inputs", "count", "blogs"}, nil)
	if err != nil {
		return fmt.Errorf("validate blogs bundle shape: %w", err)
	}
	if err := validateInputsShape(blogs["inputs"], "$.inputs"); err != nil {
		return fmt.Errorf("validate blogs bundle shape: %w", err)
	}
	blogRows, err := requiredArray(blogs["blogs"], "$.blogs")
	if err != nil {
		return fmt.Errorf("validate blogs bundle shape: %w", err)
	}
	for index, raw := range blogRows {
		path := fmt.Sprintf("$.blogs[%d]", index)
		blog, objectErr := requiredObject(raw, path, []string{
			"id", "name", "url", "summary", "feeds", "sitemap", "link_page",
			"joined_at", "updated_at", "access_scope", "visibility",
			"visibility_reason", "origins", "main_tag", "sub_tags", "architecture",
		}, map[string]bool{
			"sitemap": true, "link_page": true, "visibility_reason": true,
			"main_tag": true, "architecture": true,
		})
		if objectErr != nil {
			return fmt.Errorf("validate blogs bundle shape: %w", objectErr)
		}
		feeds, arrayErr := requiredArray(blog["feeds"], path+".feeds")
		if arrayErr != nil {
			return fmt.Errorf("validate blogs bundle shape: %w", arrayErr)
		}
		for feedIndex, feedRaw := range feeds {
			if _, objectErr = requiredObject(feedRaw, fmt.Sprintf("%s.feeds[%d]", path, feedIndex), []string{"url", "name", "is_default", "format"}, nil); objectErr != nil {
				return fmt.Errorf("validate blogs bundle shape: %w", objectErr)
			}
		}
		origins, arrayErr := requiredArray(blog["origins"], path+".origins")
		if arrayErr != nil {
			return fmt.Errorf("validate blogs bundle shape: %w", arrayErr)
		}
		for originIndex, rawOrigin := range origins {
			originPath := fmt.Sprintf("%s.origins[%d]", path, originIndex)
			origin, originErr := requiredObject(rawOrigin, originPath, []string{"source_key", "external_reference", "first_discovered_at", "metadata"}, nil)
			if originErr != nil {
				return fmt.Errorf("validate blogs bundle shape: %w", originErr)
			}
			metadata, metadataErr := requiredObject(origin["metadata"], originPath+".metadata", []string{"input_kinds", "external_references"}, nil)
			if metadataErr != nil {
				return fmt.Errorf("validate blogs bundle shape: %w", metadataErr)
			}
			if _, arrayErr = requiredArray(metadata["input_kinds"], originPath+".metadata.input_kinds"); arrayErr != nil {
				return fmt.Errorf("validate blogs bundle shape: %w", arrayErr)
			}
			if _, arrayErr = requiredArray(metadata["external_references"], originPath+".metadata.external_references"); arrayErr != nil {
				return fmt.Errorf("validate blogs bundle shape: %w", arrayErr)
			}
		}
		if err := validateOptionalTag(blog["main_tag"], path+".main_tag"); err != nil {
			return fmt.Errorf("validate blogs bundle shape: %w", err)
		}
		subTags, arrayErr := requiredArray(blog["sub_tags"], path+".sub_tags")
		if arrayErr != nil {
			return fmt.Errorf("validate blogs bundle shape: %w", arrayErr)
		}
		for tagIndex, tagRaw := range subTags {
			if err := validateTag(tagRaw, fmt.Sprintf("%s.sub_tags[%d]", path, tagIndex)); err != nil {
				return fmt.Errorf("validate blogs bundle shape: %w", err)
			}
		}
		if err := validateOptionalArchitecture(blog["architecture"], path+".architecture"); err != nil {
			return fmt.Errorf("validate blogs bundle shape: %w", err)
		}
	}

	graph, err := requiredObject(graphData, "$", []string{"format", "version", "generated_at", "inputs", "node_count", "edge_count", "count", "links"}, nil)
	if err != nil {
		return fmt.Errorf("validate graph bundle shape: %w", err)
	}
	if err := validateInputsShape(graph["inputs"], "$.inputs"); err != nil {
		return fmt.Errorf("validate graph bundle shape: %w", err)
	}
	links, err := requiredArray(graph["links"], "$.links")
	if err != nil {
		return fmt.Errorf("validate graph bundle shape: %w", err)
	}
	for index, raw := range links {
		path := fmt.Sprintf("$.links[%d]", index)
		link, objectErr := requiredObject(raw, path, []string{"source", "destinations"}, nil)
		if objectErr != nil {
			return fmt.Errorf("validate graph bundle shape: %w", objectErr)
		}
		if _, arrayErr := requiredArray(link["destinations"], path+".destinations"); arrayErr != nil {
			return fmt.Errorf("validate graph bundle shape: %w", arrayErr)
		}
	}
	return nil
}

func validateInputsShape(data json.RawMessage, path string) error {
	inputs, err := requiredArray(data, path)
	if err != nil {
		return err
	}
	for index, raw := range inputs {
		if _, err := requiredObject(raw, fmt.Sprintf("%s[%d]", path, index), []string{"kind", "file", "sha256", "count"}, nil); err != nil {
			return err
		}
	}
	return nil
}

func requiredObject(data json.RawMessage, path string, required []string, nullable map[string]bool) (map[string]json.RawMessage, error) {
	if isJSONNull(data) {
		return nil, fmt.Errorf("%s must be an object", path)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return nil, fmt.Errorf("%s must be an object", path)
	}
	for _, key := range required {
		value, exists := object[key]
		if !exists {
			return nil, fmt.Errorf("%s.%s is required", path, key)
		}
		if isJSONNull(value) && !nullable[key] {
			return nil, fmt.Errorf("%s.%s must not be null", path, key)
		}
	}
	return object, nil
}

func requiredArray(data json.RawMessage, path string) ([]json.RawMessage, error) {
	if isJSONNull(data) {
		return nil, fmt.Errorf("%s must be an array", path)
	}
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil || values == nil {
		return nil, fmt.Errorf("%s must be an array", path)
	}
	return values, nil
}

func validateOptionalTag(data json.RawMessage, path string) error {
	if isJSONNull(data) {
		return nil
	}
	return validateTag(data, path)
}

func validateTag(data json.RawMessage, path string) error {
	_, err := requiredObject(data, path, []string{"id", "name", "machine_key", "description", "is_enabled"}, map[string]bool{"machine_key": true, "description": true})
	return err
}

func validateOptionalArchitecture(data json.RawMessage, path string) error {
	if isJSONNull(data) {
		return nil
	}
	architecture, err := requiredObject(data, path, []string{"program", "technology_stacks"}, nil)
	if err != nil {
		return err
	}
	if _, err := requiredObject(architecture["program"], path+".program", []string{
		"id", "name", "name_normalized", "is_open_source", "website_url", "repo_url", "is_enabled",
	}, map[string]bool{"website_url": true, "repo_url": true}); err != nil {
		return err
	}
	stacks, err := requiredArray(architecture["technology_stacks"], path+".technology_stacks")
	if err != nil {
		return err
	}
	for index, raw := range stacks {
		stackPath := fmt.Sprintf("%s.technology_stacks[%d]", path, index)
		stack, objectErr := requiredObject(raw, stackPath, []string{"id", "category", "name", "name_normalized", "catalog"}, map[string]bool{"catalog": true})
		if objectErr != nil {
			return objectErr
		}
		if isJSONNull(stack["catalog"]) {
			continue
		}
		if _, objectErr = requiredObject(stack["catalog"], stackPath+".catalog", []string{
			"id", "name", "name_normalized", "technology_type", "description", "official_url", "is_enabled",
		}, map[string]bool{"description": true, "official_url": true}); objectErr != nil {
			return objectErr
		}
	}
	return nil
}

func isJSONNull(data json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(data), []byte("null"))
}
