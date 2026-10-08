package siteaudit

import (
	"context"

	"heyblog-api/internal/features/auth"
)

type Store interface {
	SubmissionSite(context.Context, string) (Snapshot, error)
	ResolveSite(context.Context, string) (Snapshot, error)
	CreateSubmission(context.Context, submissionRecord) (string, error)
	LookupAudit(context.Context, []byte) (Audit, error)
	PrepareSubmission(context.Context, Snapshot, Snapshot) (Snapshot, error)
	Options(context.Context) (SubmissionOptions, error)
	SearchSites(context.Context, string) ([]SiteSearchResult, error)
	ExistingSiteForHost(context.Context, string) (*SiteSearchResult, error)
	ListAudits(context.Context, *Status, *Action, int32, int32) (AuditPage, error)
	AuditDetail(context.Context, string) (Audit, error)
	DecisionRecipients(context.Context, string) ([]string, error)
	InTransaction(context.Context, func(AuditTransaction) error) error
}

// AuditTransaction exposes business operations bound to the same database transaction.
type AuditTransaction interface {
	LockAudit(context.Context, string) (Audit, error)
	ReadLockedAudit(context.Context) (Audit, error)
	LockSiteSnapshot(context.Context, string) (Snapshot, error)
	PrepareSubmission(context.Context, Snapshot, Snapshot) (Snapshot, error)
	ResolveTaxonomy(context.Context, auth.User, Snapshot, Snapshot) (Snapshot, error)
	ApplySite(context.Context, reviewedSite, func() (string, error)) (Snapshot, error)
	Approve(context.Context, decisionRecord) (Audit, error)
	Reject(context.Context, decisionRecord) (Audit, error)
	SaveDraft(context.Context, draftRecord) (Audit, error)
	DiscardDraft(context.Context, draftRecord) (Audit, error)
}

type submissionRecord struct {
	Action     Action
	Base       Snapshot
	Proposed   Snapshot
	Input      SubmissionInput
	LookupHash []byte
	Provenance submissionProvenance
}

type submissionProvenance struct {
	UserID       string
	Channel      string
	SourceSiteID string
	OwnershipID  string
}

type reviewedSite struct {
	Action                     Action
	Current                    Snapshot
	Final                      Snapshot
	SiteID                     string
	ReviewerID                 string
	CreatesProgramDependencies bool
}

type decisionRecord struct {
	AuditID         string
	ReviewerID      string
	ReviewerComment string
	Final           Snapshot
}

type draftVersion struct {
	AuditID                     string
	ExpectedSiteRevision        int64
	ExpectedReviewDraftRevision int64
}

type draftRecord struct {
	AuditID          string
	ReviewerID       string
	ExpectedRevision int64
	Snapshot         Snapshot
}
