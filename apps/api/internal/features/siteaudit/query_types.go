package siteaudit

import "time"

type PublicAuditResult struct {
	Action          Action     `json:"action"`
	Status          Status     `json:"status"`
	ShortID         string     `json:"short_id,omitempty"`
	ReviewerComment string     `json:"reviewer_comment,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type AuditListItem struct {
	SourceChannel   string     `json:"source_channel"`
	SubmitterUserID string     `json:"submitter_user_id,omitempty"`
	SourceSiteID    string     `json:"source_site_id,omitempty"`
	ID              string     `json:"id"`
	Action          Action     `json:"action"`
	Status          Status     `json:"status"`
	SiteID          string     `json:"site_id,omitempty"`
	SiteName        string     `json:"site_name"`
	SiteAddress     string     `json:"site_address"`
	SubmitterName   string     `json:"submitter_name,omitempty"`
	SubmitterEmail  string     `json:"submitter_email,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type AuditPage struct {
	Items      []AuditListItem `json:"items"`
	Page       int32           `json:"page"`
	PageSize   int32           `json:"page_size"`
	TotalItems int64           `json:"total_items"`
	TotalPages int32           `json:"total_pages"`
}

type SubmissionOptions struct {
	Tags                []Option                  `json:"tags"`
	Cascades            []CascadeOption           `json:"cascades,omitempty"`
	Components          []ComponentOption         `json:"components"`
	ProgramDependencies []ProgramDependencyOption `json:"program_dependencies"`
	PrivateProgramID    string                    `json:"private_program_id"`
}

type Option struct {
	LabelID  string   `json:"label_id"`
	Slug     string   `json:"slug"`
	Synonyms []string `json:"synonyms"`
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Level    int      `json:"level,omitempty"`
	ParentID string   `json:"parent_id,omitempty"`
	IsCustom bool     `json:"is_custom,omitempty"`
}

type CascadeOption struct {
	ID          string `json:"id"`
	TaxonomyKey string `json:"taxonomy_key"`
	Level1      Option `json:"level1"`
	Level2      Option `json:"level2"`
}

type ComponentOption struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	HomepageURL   string `json:"homepage_url,omitempty"`
	RepositoryURL string `json:"repository_url,omitempty"`
	IsOpenSource  bool   `json:"is_open_source"`
}

type ProgramDependencyOption struct {
	ProgramID   string `json:"program_id"`
	ComponentID string `json:"component_id"`
	Role        string `json:"role"`
}

type SiteSearchResult struct {
	ShortID    string `json:"short_id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Visibility string `json:"visibility"`
}

type SiteAvailability struct {
	Available    bool              `json:"available"`
	ExistingSite *SiteSearchResult `json:"existing_site,omitempty"`
}
