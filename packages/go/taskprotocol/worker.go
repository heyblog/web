package taskprotocol

import (
	"time"
)

type Instance struct {
	InstanceID string `json:"instance_id" format:"uuid"`
	Protocol   int    `json:"protocol"`
	Version    string `json:"version" maxLength:"120"`
}

type Worker struct {
	Instance
	RegisteredAt time.Time `json:"registered_at"`
	HeartbeatAt  time.Time `json:"heartbeat_at"`
	Draining     bool      `json:"draining"`
	Online       bool      `json:"online"`
	Active       int       `json:"active"`
}

type WorkerInput struct {
	InstanceID string `json:"instance_id" format:"uuid"`
	Protocol   int    `json:"protocol"`
}

type Fence struct {
	WorkerInput
	Token string `json:"token" format:"uuid"`
}

type PathResult struct {
	Platform   string            `json:"platform"`
	Outcome    string            `json:"outcome"`
	HTTPStatus int               `json:"http_status"`
	ErrorKind  string            `json:"error_kind,omitempty"`
	Requests   int               `json:"requests"`
	DurationMS int64             `json:"duration_ms"`
	Redirects  []string          `json:"redirects"`
	Node       map[string]string `json:"node"`
	ObservedAt *time.Time        `json:"observed_at"`
}

type ExecutionResult struct {
	Paths         []PathResult `json:"paths"`
	Fallback      bool         `json:"fallback"`
	BusinessValid bool         `json:"business_valid"`
}

type PathProgress struct {
	Attempts int         `json:"attempts"`
	Done     bool        `json:"done"`
	Valid    bool        `json:"valid"`
	Path     PathResult  `json:"path"`
	Pending  *PendingHop `json:"pending,omitempty"`
}

type PendingHop struct {
	URL      string `json:"url"`
	Retry    int    `json:"retry"`
	Hop      int    `json:"hop"`
	ParentID string `json:"parent_id"`
}

type Work struct {
	ID              string                  `json:"id"`
	RunID           string                  `json:"run_id"`
	Kind            string                  `json:"kind"`
	Address         string                  `json:"address"`
	Token           string                  `json:"token"`
	LeaseUntil      time.Time               `json:"lease_until"`
	SampleStartedAt *time.Time              `json:"sample_started_at"`
	Configuration   Configuration           `json:"configuration"`
	Route           []string                `json:"route"`
	Progress        map[string]PathProgress `json:"progress"`
	Headers         map[string]string       `json:"headers"`
	MaxBytes        int64                   `json:"max_bytes"`
	SameOrigin      bool                    `json:"same_origin"`
}

type DueSchedule struct {
	ID            string        `json:"id"`
	Revision      int           `json:"revision"`
	At            time.Time     `json:"at"`
	Configuration Configuration `json:"configuration"`
}

type Due struct {
	ServerTime time.Time     `json:"server_time"`
	Schedules  []DueSchedule `json:"schedules"`
}

type Dispatch struct {
	WorkerInput
	ID       string    `json:"id" format:"uuid"`
	Revision int       `json:"revision"`
	At       time.Time `json:"at"`
	NextAt   time.Time `json:"next_at"`
}

type Authorization struct {
	Fence
	RequestKey string `json:"request_key" minLength:"1" maxLength:"120"`
	Platform   string `json:"platform" enum:"CF,EO"`
	URL        string `json:"url" maxLength:"16384"`
	Method     string `json:"method" enum:"GET,HEAD"`
	Retry      int    `json:"retry" minimum:"0" maximum:"2"`
	Hop        int    `json:"hop" minimum:"0" maximum:"3"`
	ParentID   string `json:"parent_id"`
}

type Permit struct {
	ID        string            `json:"id"`
	Headers   map[string][]byte `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
	RetryAt   *time.Time        `json:"retry_at"`
	Reason    string            `json:"reason"`
}

type Report struct {
	Fence
	AttemptID  string            `json:"attempt_id" format:"uuid"`
	Status     int               `json:"status" minimum:"0" maximum:"599"`
	Headers    map[string][]byte `json:"headers"`
	Body       []byte            `json:"body"`
	ErrorKind  string            `json:"error_kind" enum:",target,platform,security,signature,cancelled,resolver,backoff"`
	DurationMS int64             `json:"duration_ms" minimum:"0"`
	Node       map[string]string `json:"node"`
}

type Evaluation struct {
	Valid    bool          `json:"valid"`
	Continue bool          `json:"continue"`
	Reason   string        `json:"reason"`
	Progress *PathProgress `json:"progress"`
}

type Finish struct {
	Fence
	Reason string `json:"reason" enum:",cancelled,interrupted,round_expired"`
}
