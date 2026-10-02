package dataimport

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"heyblog-api/internal/domain/site"
)

var (
	uuidV7Pattern  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	slugPattern    = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	shortIDPattern = regexp.MustCompile(`^[0-9A-Za-z]{9}$`)
)

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

func addTag(plan *Plan, tags map[string]TagRow, siteID string, legacy LegacyTag, role string, position int16) error {
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
	plan.SiteTags = append(plan.SiteTags, SiteTagRow{
		SiteID: siteID, TagID: legacy.ID, Role: role, Position: position,
	})
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

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
