package taskprotocol

import "time"

const ProtocolVersion = 2

type RunState string

const (
	Queued        RunState = "queued"
	Running       RunState = "running"
	Pausing       RunState = "pausing"
	Paused        RunState = "paused"
	Cancelling    RunState = "cancelling"
	Succeeded     RunState = "succeeded"
	PartialFailed RunState = "partial_failed"
	Failed        RunState = "failed"
	TimedOut      RunState = "timed_out"
	Cancelled     RunState = "cancelled"
)

func (s RunState) Terminal() bool {
	switch s {
	case Succeeded, PartialFailed, Failed, TimedOut, Cancelled:
		return true
	case Queued, Running, Pausing, Paused, Cancelling:
		return false
	default:
		return false
	}
}

func (s RunState) Actions(archived bool) []string {
	actions := []string{}
	switch s {
	case Queued, Running:
		actions = append(actions, "pause", "cancel")
	case Pausing:
		actions = append(actions, "cancel")
	case Paused:
		actions = append(actions, "resume", "cancel")
	case Cancelling:
	case Succeeded, PartialFailed, Failed, TimedOut, Cancelled:
		actions = append(actions, "retry", "rerun")
		if archived {
			actions = append(actions, "unarchive", "cleanup")
		} else {
			actions = append(actions, "archive")
		}
	}
	return actions
}

// Elapsed excludes queue time and intervals fully paused; pausing still counts.
func Elapsed(now time.Time, started, pausedAt *time.Time, paused time.Duration) time.Duration {
	if started == nil {
		return 0
	}
	end := now
	if pausedAt != nil {
		end = *pausedAt
	}
	return max(0, end.Sub(*started)-paused)
}

type TaskCounts struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped"`
	Cancelled int `json:"cancelled"`
	Pending   int `json:"pending"`
	Usable    int `json:"usable"`
}

func (c TaskCounts) State() RunState {
	if c.Pending > 0 {
		return Running
	}
	if c.Failed == 0 && c.Cancelled == 0 {
		return Succeeded
	}
	if c.Usable > 0 {
		return PartialFailed
	}
	return Failed
}
