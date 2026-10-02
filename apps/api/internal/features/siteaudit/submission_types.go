package siteaudit

type FeedInput struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Format    string `json:"format"`
	IsDefault bool   `json:"is_default"`
}

type ResourceInput struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

type TagInput struct {
	ID            string `json:"id"`
	SuggestedName string `json:"suggested_name"`
	Slug          string `json:"slug"`
	Description   string `json:"description"`
	Role          string `json:"role"`
	Level         int    `json:"level,omitempty"`
	ParentID      string `json:"parent_id,omitempty"`
}

type ComponentInput struct {
	ID            string `json:"id"`
	SuggestedName string `json:"suggested_name"`
	Role          string `json:"role"`
	HomepageURL   string `json:"homepage_url"`
	RepositoryURL string `json:"repository_url"`
	IsOpenSource  *bool  `json:"is_open_source"`
}

type SiteInput struct {
	Name                string           `json:"name"`
	URL                 string           `json:"url"`
	Summary             string           `json:"summary"`
	Feeds               []FeedInput      `json:"feeds"`
	Resources           []ResourceInput  `json:"resources"`
	Tags                []TagInput       `json:"tags"`
	TagCascadeID        string           `json:"tag_cascade_id,omitempty"`
	Components          []ComponentInput `json:"components"`
	ProgramDependencies []ComponentInput `json:"program_dependencies"`
}

type ContactInput struct {
	Name          string `json:"name"`
	Email         string `json:"email"`
	NotifyByEmail bool   `json:"notify_by_email"`
}

type SubmissionInput struct {
	Site    SiteInput    `json:"site"`
	Reason  string       `json:"reason,omitempty"`
	Contact ContactInput `json:"contact"`
}

type SubmissionResult struct {
	AuditID     string `json:"audit_id"`
	LookupToken string `json:"lookup_token"`
	Action      Action `json:"action"`
	Status      Status `json:"status"`
	ShortID     string `json:"short_id,omitempty"`
}
