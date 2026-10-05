package sluggeneration

import (
	"context"
	"time"
)

type JobSelection struct {
	Kind   string     `json:"kind" enum:"invalid,ids,filter,all"`
	IDs    []string   `json:"ids,omitempty" maxItems:"500"`
	Filter *JobFilter `json:"filter,omitempty"`
}
type JobFilter struct {
	Query   string `json:"query,omitempty" maxLength:"128"`
	Enabled *bool  `json:"is_enabled,omitempty"`
}
type CreateJobInput struct {
	Selection        JobSelection `json:"selection"`
	ExpectedRevision string       `json:"expected_revision,omitempty"`
}
type JobItem struct {
	TagID        string `json:"tag_id"`
	Name         string `json:"name"`
	OriginalSlug string `json:"original_slug"`
	Slug         string `json:"slug"`
	State        string `json:"state" enum:"pending,running,ready,failed,stale,applied"`
	ErrorCode    string `json:"error_code"`
	Source       string `json:"source"`
}
type JobCounts struct {
	Total   int `json:"total"`
	Ready   int `json:"ready"`
	Failed  int `json:"failed"`
	Applied int `json:"applied"`
}
type Job struct {
	ID          string    `json:"id"`
	Status      string    `json:"status" enum:"queued,running,paused,ready,cancelled,completed"`
	Revision    string    `json:"revision"`
	ModelID     string    `json:"model_id"`
	PauseCode   string    `json:"pause_code"`
	ResumeAfter string    `json:"resume_after"`
	Counts      JobCounts `json:"counts"`
	Items       []JobItem `json:"items"`
}
type JobControlInput struct {
	Action           string `json:"action" enum:"pause,resume,cancel,retry"`
	ExpectedRevision string `json:"expected_revision,omitempty"`
}
type JobEdit struct {
	TagID string `json:"tag_id" minLength:"36" maxLength:"36"`
	Slug  string `json:"slug" minLength:"1" maxLength:"128" pattern:"^[a-z0-9]+(-[a-z0-9]+)*$"`
}
type JobEditInput struct {
	Items            []JobEdit `json:"items" minItems:"1" maxItems:"500"`
	ExpectedRevision string    `json:"expected_revision" minLength:"1"`
}
type JobApplyInput struct {
	TagIDs           []string `json:"tag_ids" minItems:"1" maxItems:"500"`
	ExpectedRevision string   `json:"expected_revision" minLength:"1"`
}

// Internal snapshots stay inside the backend; HTTP exposes only JobItem.
type jobItemRecord struct {
	JobItem
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type jobRecord struct {
	ID          string
	OwnerID     string
	IPHash      string
	ModelID     string
	Status      string
	Revision    int64
	Items       []jobItemRecord
	PauseCode   string
	ResumeAfter time.Time
	LeaseToken  string
}

type JobStore interface {
	Create(context.Context, Identity, JobSelection, string, int) (jobRecord, error)
	Get(context.Context, string, Identity) (jobRecord, error)
	List(context.Context, Identity) ([]jobRecord, error)
	Mutate(context.Context, string, Identity, string, func(*jobRecord) error) (jobRecord, error)
	Apply(context.Context, string, Identity, JobApplyInput) (jobRecord, error)
	Claim(context.Context, string) (jobRecord, bool, error)
	SaveClaim(context.Context, jobRecord) (bool, error)
	Authorized(context.Context, string) (bool, error)
}
