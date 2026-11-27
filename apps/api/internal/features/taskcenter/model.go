package taskcenter

import (
	"encoding/json"
	"time"

	"heyblog.local/taskprotocol"
)

type Configuration = taskprotocol.Configuration
type RunState = taskprotocol.RunState

type ScheduleInput struct {
	Name          string        `json:"name" minLength:"1" maxLength:"120"`
	Kind          string        `json:"kind" enum:"monitor,rss"`
	Enabled       bool          `json:"enabled"`
	Archived      bool          `json:"archived"`
	Configuration Configuration `json:"configuration"`
	Revision      int           `json:"revision"`
}
type Schedule struct {
	ID string `json:"id"`
	ScheduleInput
	NextAt *time.Time `json:"next_at"`
}
type Run struct {
	ID             string                  `json:"id"`
	ScheduleID     *string                 `json:"schedule_id"`
	Kind           string                  `json:"kind"`
	State          RunState                `json:"state"`
	Snapshot       Configuration           `json:"snapshot"`
	ScheduledAt    time.Time               `json:"scheduled_at"`
	CreatedAt      time.Time               `json:"created_at"`
	StartedAt      *time.Time              `json:"started_at"`
	FinishedAt     *time.Time              `json:"finished_at"`
	PausedAt       *time.Time              `json:"paused_at"`
	PausedMS       int64                   `json:"paused_ms"`
	TimeoutSeconds int                     `json:"timeout_seconds"`
	ElapsedMS      int64                   `json:"elapsed_ms"`
	QueueMS        int64                   `json:"queue_ms"`
	Source         string                  `json:"source"`
	ActorID        *string                 `json:"actor_id"`
	RootRunID      *string                 `json:"root_run_id"`
	ParentRunID    *string                 `json:"parent_run_id"`
	RetryNumber    int                     `json:"retry_number"`
	Archived       bool                    `json:"archived"`
	CleanedAt      *time.Time              `json:"cleaned_at"`
	Reason         string                  `json:"reason"`
	Counts         taskprotocol.TaskCounts `json:"counts"`
	AllowedActions []string                `json:"allowed_actions"`
}
type Target struct {
	ID              string          `json:"id"`
	RunID           string          `json:"run_id"`
	SiteID          string          `json:"site_id"`
	FeedID          *string         `json:"feed_id"`
	ClaimID         *string         `json:"claim_id"`
	Kind            string          `json:"kind"`
	Address         string          `json:"address"`
	Generation      int64           `json:"generation"`
	AccessScope     *string         `json:"access_scope"`
	State           string          `json:"state"`
	LeaseToken      *string         `json:"-"`
	LeaseUntil      *time.Time      `json:"-"`
	WorkerID        *string         `json:"-"`
	RoundID         string          `json:"round_id"`
	SampleStartedAt *time.Time      `json:"sample_started_at"`
	Reason          string          `json:"reason"`
	Usable          bool            `json:"usable"`
	Recoveries      int             `json:"recoveries"`
	Result          json.RawMessage `json:"result"`
}
type Lease struct {
	Target        Target
	Configuration Configuration
	StartedAt     time.Time
	Progress      map[string]taskprotocol.PathProgress
	Route         []string
}
type Policy struct {
	Enabled    bool   `json:"enabled"`
	Carrier    string `json:"carrier" enum:"both,header"`
	TTLSeconds int    `json:"ttl_seconds" minimum:"60" maximum:"300"`
	Revision   int    `json:"revision"`
}
type Page[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}
type Query struct {
	Page     int    `query:"page" minimum:"1" default:"1"`
	PageSize int    `query:"page_size" minimum:"1" maximum:"100" default:"20"`
	Kind     string `query:"kind" enum:",monitor,rss,verification"`
	State    string `query:"state"`
	Source   string `query:"source"`
	Schedule string `query:"schedule"`
	From     string `query:"from"`
	To       string `query:"to"`
	Archived string `query:"archived" enum:",true,false"`
}
