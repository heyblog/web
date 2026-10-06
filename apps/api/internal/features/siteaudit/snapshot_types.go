package siteaudit

type FeedSnapshot struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Format    string `json:"format"`
	IsDefault bool   `json:"is_default"`
}

type ResourceSnapshot struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

type TagSnapshot struct {
	LabelID       string `json:"label_id,omitempty"`
	ID            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	SuggestedName string `json:"suggested_name,omitempty"`
	Slug          string `json:"slug,omitempty"`
	Description   string `json:"description,omitempty"`
	Role          string `json:"role"`
	Level         int    `json:"level,omitempty"`
	ParentID      string `json:"parent_id,omitempty"`
}

type CascadeSnapshot struct {
	ID          string      `json:"id"`
	TaxonomyKey string      `json:"taxonomy_key"`
	Level1      TagSnapshot `json:"level1"`
	Level2      TagSnapshot `json:"level2"`
}

type ComponentSnapshot struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	SuggestedName string `json:"suggested_name,omitempty"`
	Role          string `json:"role"`
	HomepageURL   string `json:"homepage_url,omitempty"`
	RepositoryURL string `json:"repository_url,omitempty"`
	IsOpenSource  *bool  `json:"is_open_source"`
}

type Snapshot struct {
	SiteID              string              `json:"site_id,omitempty"`
	Revision            int64               `json:"revision,omitempty"`
	ShortID             string              `json:"short_id,omitempty"`
	CustomID            string              `json:"custom_id,omitempty"`
	Name                string              `json:"name"`
	Scheme              string              `json:"scheme"`
	NormalizedHost      string              `json:"normalized_host"`
	BasePath            string              `json:"base_path"`
	Summary             string              `json:"summary"`
	AccessScope         string              `json:"access_scope"`
	TagCascadeID        string              `json:"tag_cascade_id,omitempty"`
	Classification      *CascadeSnapshot    `json:"classification,omitempty"`
	Visibility          string              `json:"visibility"`
	VisibilityReason    string              `json:"visibility_reason,omitempty"`
	Feeds               []FeedSnapshot      `json:"feeds"`
	Resources           []ResourceSnapshot  `json:"resources"`
	Tags                []TagSnapshot       `json:"tags"`
	Components          []ComponentSnapshot `json:"components"`
	ProgramDependencies []ComponentSnapshot `json:"program_dependencies"`
}
