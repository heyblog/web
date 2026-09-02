package siteaudit

import "time"

type Action string

const (
	ActionCreate  Action = "CREATE"
	ActionUpdate  Action = "UPDATE"
	ActionDelete  Action = "DELETE"
	ActionRestore Action = "RESTORE"
)

type Status string

const (
	StatusPending  Status = "PENDING"
	StatusApproved Status = "APPROVED"
	StatusRejected Status = "REJECTED"
)

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
	ID            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	SuggestedName string `json:"suggested_name,omitempty"`
	Slug          string `json:"slug,omitempty"`
	Description   string `json:"description,omitempty"`
	Role          string `json:"role"`
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
	Visibility          string              `json:"visibility"`
	VisibilityReason    string              `json:"visibility_reason,omitempty"`
	Feeds               []FeedSnapshot      `json:"feeds"`
	Resources           []ResourceSnapshot  `json:"resources"`
	Tags                []TagSnapshot       `json:"tags"`
	Components          []ComponentSnapshot `json:"components"`
	ProgramDependencies []ComponentSnapshot `json:"program_dependencies"`
}

type DiffChange struct {
	Key    string `json:"key"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type DiffItem struct {
	Field   string       `json:"field"`
	Before  string       `json:"before,omitempty"`
	After   string       `json:"after,omitempty"`
	Added   []string     `json:"added,omitempty"`
	Removed []string     `json:"removed,omitempty"`
	Changed []DiffChange `json:"changed,omitempty"`
}

type DiffViews struct {
	Requested          []DiffItem `json:"requested"`
	Drift              []DiffItem `json:"drift"`
	ReviewerCorrection []DiffItem `json:"reviewer_correction"`
	Conflicts          []DiffItem `json:"conflicts"`
}

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

type Audit struct {
	ID                   string     `json:"id"`
	Action               Action     `json:"action"`
	Status               Status     `json:"status"`
	SiteID               string     `json:"site_id,omitempty"`
	BaseRevision         int64      `json:"base_revision,omitempty"`
	BaseSnapshot         Snapshot   `json:"base_snapshot"`
	ProposedSnapshot     Snapshot   `json:"proposed_snapshot"`
	ReviewDraftSnapshot  *Snapshot  `json:"review_draft_snapshot,omitempty"`
	ReviewDraftRevision  int64      `json:"review_draft_revision"`
	ReviewDraftUpdatedBy string     `json:"review_draft_updated_by,omitempty"`
	ReviewDraftUpdatedAt *time.Time `json:"review_draft_updated_at,omitempty"`
	FinalSnapshot        Snapshot   `json:"final_snapshot"`
	RequestReason        string     `json:"request_reason"`
	SubmitterName        string     `json:"submitter_name,omitempty"`
	SubmitterEmail       string     `json:"submitter_email,omitempty"`
	NotifyByEmail        bool       `json:"notify_by_email"`
	ReviewerComment      string     `json:"reviewer_comment,omitempty"`
	ReviewedBy           string     `json:"reviewed_by,omitempty"`
	ReviewedAt           *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	CurrentSnapshot      Snapshot   `json:"current_snapshot"`
	EffectiveSnapshot    Snapshot   `json:"effective_snapshot"`
	Diff                 DiffViews  `json:"diff"`
	HasCurrentSnapshot   bool       `json:"has_current_snapshot"`
}

type PublicAuditResult struct {
	Action          Action     `json:"action"`
	Status          Status     `json:"status"`
	ShortID         string     `json:"short_id,omitempty"`
	ReviewerComment string     `json:"reviewer_comment,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type AuditListItem struct {
	ID             string     `json:"id"`
	Action         Action     `json:"action"`
	Status         Status     `json:"status"`
	SiteID         string     `json:"site_id,omitempty"`
	SiteName       string     `json:"site_name"`
	SiteAddress    string     `json:"site_address"`
	SubmitterName  string     `json:"submitter_name,omitempty"`
	SubmitterEmail string     `json:"submitter_email,omitempty"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
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
	Components          []ComponentOption         `json:"components"`
	ProgramDependencies []ProgramDependencyOption `json:"program_dependencies"`
	PrivateProgramID    string                    `json:"private_program_id"`
}

type Option struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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

type ReviewDecision string

const (
	DecisionApprove ReviewDecision = "APPROVED"
	DecisionReject  ReviewDecision = "REJECTED"
)

type ReviewInput struct {
	AuditID                     string         `json:"-"`
	Decision                    ReviewDecision `json:"decision"`
	ReviewerComment             string         `json:"reviewer_comment"`
	ExpectedSiteRevision        int64          `json:"expected_site_revision"`
	ExpectedReviewDraftRevision int64          `json:"expected_review_draft_revision"`
}

type ReviewDraftInput struct {
	AuditID                     string    `json:"-"`
	Site                        SiteInput `json:"site"`
	ExpectedSiteRevision        int64     `json:"expected_site_revision"`
	ExpectedReviewDraftRevision int64     `json:"expected_review_draft_revision"`
}

type DiscardReviewDraftInput struct {
	AuditID                     string `json:"-"`
	ExpectedSiteRevision        int64  `json:"expected_site_revision"`
	ExpectedReviewDraftRevision int64  `json:"expected_review_draft_revision"`
}
