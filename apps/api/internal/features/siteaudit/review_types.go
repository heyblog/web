package siteaudit

import "time"

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

type Audit struct {
	SourceChannel        string     `json:"source_channel"`
	SubmitterUserID      string     `json:"submitter_user_id,omitempty"`
	SourceSiteID         string     `json:"source_site_id,omitempty"`
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
