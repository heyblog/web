package taskprotocol

import (
	"errors"
	"testing"
	"time"
)

func TestScheduleNext(t *testing.T) {
	now := time.Date(2026, 10, 22, 0, 30, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		config Configuration
		want   time.Time
	}{
		{"interval recovery skips missed periods", Configuration{Mode: "interval", Timezone: "Asia/Shanghai", IntervalSeconds: 3600}, now.Add(time.Hour)},
		{"cron uses configured zone", Configuration{Mode: "cron", Timezone: "Asia/Shanghai", Cron: "0 9 * * *"}, now.Add(30 * time.Minute)},
		{"cron advances strictly beyond current minute", Configuration{Mode: "cron", Timezone: "UTC", Cron: "30 * * * *"}, now.Add(time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given a validated schedule and the API's current time.
			// When computing the first future occurrence.
			got, err := tc.config.Next(now)
			// Then downtime never schedules a past occurrence.
			if err != nil || !got.Equal(tc.want) {
				t.Fatalf("Next = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestScheduleRejectsInvalidRules(t *testing.T) {
	for _, c := range []Configuration{
		{Mode: "interval", Timezone: "Local", IntervalSeconds: 60},
		{Mode: "interval", Timezone: "UTC", IntervalSeconds: 59},
		{Mode: "interval", Timezone: "UTC", IntervalSeconds: 2678401},
		{Mode: "cron", Timezone: "UTC", Cron: "* * * * * *"},
		{Mode: "cron", Timezone: "UTC", Cron: "0 0 31 2 *"},
		{Mode: "cron", Timezone: "Unknown/Zone", Cron: "* * * * *"},
	} {
		// Given an invalid timezone, interval or Cron expression.
		// When computing its next occurrence.
		_, err := c.Next(time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC))
		// Then API and worker both reject the rule.
		if !errors.Is(err, ErrSchedule) {
			t.Errorf("Next(%+v) error = %v", c, err)
		}
	}
}
