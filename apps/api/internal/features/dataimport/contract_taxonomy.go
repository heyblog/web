package dataimport

type TagTaxonomyBundle struct {
	Format        string                     `json:"format"`
	Version       int                        `json:"version"`
	GeneratedAt   string                     `json:"generated_at"`
	Inputs        []TagTaxonomyInputMetadata `json:"inputs"`
	TagCount      int                        `json:"tag_count"`
	CascadeCount  int                        `json:"cascade_count"`
	SiteCount     int                        `json:"site_count"`
	TertiaryCount int                        `json:"tertiary_count"`
	TrimmedCount  int                        `json:"trimmed_count"`
	Tags          []TagTaxonomyDefinition    `json:"tags"`
	Cascades      []TagTaxonomyCascade       `json:"cascades"`
	Sites         []SiteTagTaxonomyMigration `json:"sites"`
}

type TagTaxonomyInputMetadata struct {
	Kind   string `json:"kind"`
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	Count  int    `json:"count"`
}

type TagTaxonomyDefinition struct {
	Source        string  `json:"source"`
	TagID         string  `json:"tag_id"`
	Name          string  `json:"name"`
	Description   *string `json:"description"`
	TaxonomyLevel *int    `json:"taxonomy_level"`
	ParentTagID   *string `json:"parent_tag_id"`
	SortOrder     *int    `json:"sort_order"`
}

type TagTaxonomyCascade struct {
	Scope       string `json:"scope"`
	CascadeKey  string `json:"cascade_key"`
	Level1TagID string `json:"level1_tag_id"`
	Level2TagID string `json:"level2_tag_id"`
	SortOrder   int    `json:"sort_order"`
}

type SiteTagTaxonomyMigration struct {
	SiteID       string                `json:"site_id"`
	SourceURL    string                `json:"source_url"`
	MatchMethod  string                `json:"match_method"`
	CascadeKey   string                `json:"cascade_key"`
	TertiaryTags []TagTaxonomyTertiary `json:"tertiary_tags"`
}

type TagTaxonomyTertiary struct {
	Source   string `json:"source"`
	TagID    string `json:"tag_id"`
	Name     string `json:"name"`
	Position int16  `json:"position"`
}
