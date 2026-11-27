// Package taskprotocol defines the API-owned internal worker contract.
package taskprotocol

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var ErrSchedule = errors.New("invalid schedule")

type Configuration struct {
	Mode              string            `json:"mode" enum:"interval,cron"`
	IntervalSeconds   int               `json:"interval_seconds"`
	Cron              string            `json:"cron"`
	Timezone          string            `json:"timezone"`
	Scope             string            `json:"scope" enum:"all,selected"`
	SiteIDs           []string          `json:"site_ids"`
	RequestConfigID   string            `json:"request_config_id"`
	FeedMode          string            `json:"feed_mode" enum:"default,all"`
	UserAgent         string            `json:"user_agent"`
	Headers           map[string]string `json:"headers"`
	TimeoutSeconds    int               `json:"timeout_seconds"`
	IntervalMS        int               `json:"interval_ms"`
	Concurrency       int               `json:"concurrency"`
	Retries           int               `json:"retries"`
	MaxRedirects      int               `json:"max_redirects"`
	RunTimeoutSeconds int               `json:"run_timeout_seconds"`
	Signed            bool              `json:"signed"`
	Carrier           string            `json:"carrier"`
	TTLSeconds        int               `json:"ttl_seconds"`
}

func (c Configuration) Next(now time.Time) (time.Time, error) {
	location, err := time.LoadLocation(c.Timezone)
	if err != nil || c.Timezone == "" || c.Timezone == "Local" {
		return time.Time{}, fmt.Errorf("%w: timezone must be an IANA zone", ErrSchedule)
	}
	switch c.Mode {
	case "interval":
		if c.IntervalSeconds < 60 || c.IntervalSeconds > 2678400 {
			return time.Time{}, fmt.Errorf("%w: interval must be 60-2678400 seconds", ErrSchedule)
		}
		return now.Add(time.Duration(c.IntervalSeconds) * time.Second), nil
	case "cron":
		if len(strings.Fields(c.Cron)) != 5 {
			return time.Time{}, fmt.Errorf("%w: cron must contain five fields", ErrSchedule)
		}
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := parser.Parse(c.Cron)
		if err != nil {
			return time.Time{}, fmt.Errorf("%w: %w", ErrSchedule, err)
		}
		next := schedule.Next(now.In(location))
		if next.IsZero() {
			return time.Time{}, fmt.Errorf("%w: no next occurrence", ErrSchedule)
		}
		return next, nil
	default:
		return time.Time{}, ErrSchedule
	}
}
