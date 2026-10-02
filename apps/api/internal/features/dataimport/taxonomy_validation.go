package dataimport

import (
	"errors"
	"strings"
	"time"
)

func validateTagTaxonomyBundle(bundle TagTaxonomyBundle) error {
	if bundle.Format != taxonomyFormat || bundle.Version != 1 {
		return errors.New("unsupported tag taxonomy format or version")
	}
	if bundle.TagCount != len(bundle.Tags) || bundle.CascadeCount != len(bundle.Cascades) || bundle.SiteCount != len(bundle.Sites) {
		return errors.New("tag taxonomy counts are inconsistent")
	}
	if _, err := time.Parse(time.RFC3339Nano, bundle.GeneratedAt); err != nil || len(bundle.Inputs) != 2 {
		return errors.New("tag taxonomy generation metadata is invalid")
	}
	tagSources := make(map[string]string, len(bundle.Tags))
	for _, tag := range bundle.Tags {
		if tag.TagID == "" || strings.TrimSpace(tag.Name) == "" || (tag.Source != "SQLITE" && tag.Source != "LEGACY") {
			return errors.New("tag taxonomy tag definition is invalid")
		}
		if _, exists := tagSources[tag.TagID]; exists {
			return errors.New("tag taxonomy tag identifiers must be unique")
		}
		tagSources[tag.TagID] = tag.Source
	}
	cascadeKeys := make(map[string]TagTaxonomyCascade, len(bundle.Cascades))
	for _, cascade := range bundle.Cascades {
		if cascade.Scope != "SITE" && cascade.Scope != "ARTICLE" || cascade.CascadeKey == "" || cascade.Level1TagID == "" || cascade.Level2TagID == "" {
			return errors.New("tag taxonomy cascade is invalid")
		}
		if tagSources[cascade.Level1TagID] != "SQLITE" || tagSources[cascade.Level2TagID] != "SQLITE" {
			return errors.New("tag taxonomy cascade references unknown fixed tags")
		}
		cascadeKeys[cascade.Scope+"\x00"+cascade.CascadeKey] = cascade
	}
	tertiaryCount := 0
	seenSites := make(map[string]struct{}, len(bundle.Sites))
	for _, site := range bundle.Sites {
		if site.SiteID == "" || site.CascadeKey == "" {
			return errors.New("tag taxonomy site mapping is invalid")
		}
		if len(site.TertiaryTags) > 20 {
			return errors.New("tag taxonomy tertiary tags exceed twenty")
		}
		if _, exists := seenSites[site.SiteID]; exists {
			return errors.New("tag taxonomy site identifiers must be unique")
		}
		seenSites[site.SiteID] = struct{}{}
		cascade, exists := cascadeKeys["SITE\x00"+site.CascadeKey]
		if !exists {
			return errors.New("tag taxonomy site references an unknown SITE cascade")
		}
		seenTags := make(map[string]struct{}, len(site.TertiaryTags))
		for index, tag := range site.TertiaryTags {
			if tag.Position != int16(index+1) || tagSources[tag.TagID] != tag.Source || tag.TagID == cascade.Level1TagID || tag.TagID == cascade.Level2TagID {
				return errors.New("tag taxonomy tertiary assignment is invalid")
			}
			if _, duplicate := seenTags[tag.TagID]; duplicate {
				return errors.New("tag taxonomy tertiary assignments must be unique")
			}
			seenTags[tag.TagID] = struct{}{}
		}
		tertiaryCount += len(site.TertiaryTags)
	}
	if tertiaryCount != bundle.TertiaryCount {
		return errors.New("tag taxonomy tertiary count is inconsistent")
	}
	return nil
}
