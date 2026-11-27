package monitoring

import "time"

type Outcome string

const (
	OK      Outcome = "OK"
	Fail    Outcome = "FAIL"
	Unknown Outcome = "UNKNOWN"
)

// Round identifies a logical dual-region check, shared by retries and recovery.
type Round struct {
	SiteID     string
	Generation int64
	ID         string
	StartedAt  time.Time
	CN         Outcome
	GL         Outcome
}

func (outcome Outcome) valid() bool {
	switch outcome {
	case OK, Fail, Unknown:
		return true
	default:
		return false
	}
}

// Summarize consumes newest-first windows already filtered for validity.
// A nil conclusion preserves the previous stable value.
func Summarize(health, scope []Round) (status, access *string) {
	if len(health) >= 10 {
		failures := 0
		for _, round := range health[:10] {
			if round.CN == Fail && round.GL == Fail {
				failures++
			}
		}
		value := "OK"
		if failures == 10 {
			value = "DOWN"
		} else if failures >= 2 {
			value = "WARN"
		}
		status = &value
	}
	if len(scope) >= 5 {
		candidate := scopeCandidate(scope[0])
		for _, round := range scope[1:5] {
			if scopeCandidate(round) != candidate {
				return status, nil
			}
		}
		if candidate != "" {
			access = &candidate
		}
	}
	return status, access
}

func scopeCandidate(round Round) string {
	switch {
	case round.CN == OK && round.GL == OK:
		return "ALL"
	case round.CN == OK && round.GL == Fail:
		return "CN"
	case round.CN == Fail && round.GL == OK:
		return "GL"
	default:
		return ""
	}
}
