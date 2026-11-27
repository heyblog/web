package taskcenter

import (
	"testing"
	"time"
)

func scheduleFixture() ScheduleInput {
	return ScheduleInput{Name: "Check", Kind: "monitor", Configuration: Configuration{Mode: "interval", IntervalSeconds: 3600, Timezone: "Asia/Shanghai", Scope: "all", SiteIDs: []string{}, UserAgent: "HeyBlog-Test", Headers: map[string]string{}, TimeoutSeconds: 8, IntervalMS: 1000, Concurrency: 2}}
}
func TestScheduleNextHandlesCronTimezoneAndRejectsInvalidInputs(t *testing.T) {
	now := time.Date(2026, 10, 19, 0, 1, 0, 0, time.UTC)
	cases := []struct {
		name  string
		edit  func(*ScheduleInput)
		valid bool
		next  time.Time
	}{
		{"interval", func(*ScheduleInput) {}, true, now.Add(time.Hour)},
		{"timezone cron", func(s *ScheduleInput) { s.Configuration.Mode = "cron"; s.Configuration.Cron = "0 9 * * *" }, true, time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)},
		{"six fields", func(s *ScheduleInput) { s.Configuration.Mode = "cron"; s.Configuration.Cron = "0 0 9 * * *" }, false, time.Time{}},
		{"descriptor", func(s *ScheduleInput) { s.Configuration.Mode = "cron"; s.Configuration.Cron = "@hourly" }, false, time.Time{}},
		{"unknown zone", func(s *ScheduleInput) { s.Configuration.Timezone = "Asia/Invalid" }, false, time.Time{}},
		{"excess retries", func(s *ScheduleInput) { s.Configuration.Retries = 3 }, false, time.Time{}},
		{"empty selected", func(s *ScheduleInput) { s.Configuration.Scope = "selected" }, false, time.Time{}},
		{"header injection", func(s *ScheduleInput) { s.Configuration.Headers["Accept"] = "x\r\nAuthorization: secret" }, false, time.Time{}},
		{"proof override", func(s *ScheduleInput) { s.Configuration.Headers["x-heyblog-proof"] = "fake" }, false, time.Time{}},
		{"worker secret", func(s *ScheduleInput) { s.Configuration.Headers["authorization"] = "Bearer secret" }, false, time.Time{}},
		{"forwarded header", func(s *ScheduleInput) { s.Configuration.Headers["X-Forwarded-For"] = "1.1.1.1" }, false, time.Time{}},
		{"unsupported content header", func(s *ScheduleInput) { s.Configuration.Headers["Content-Type"] = "text/plain" }, false, time.Time{}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			input := scheduleFixture()
			tt.edit(&input)
			next, err := input.Validate(now)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
			if tt.valid && !next.Equal(tt.next) {
				t.Fatalf("next=%v want=%v", next, tt.next)
			}
		})
	}
}
func TestFeedParsingRejectsMalformedInputAndDeduplicates(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		count      int
		valid      bool
	}{
		{"rss", `<rss><channel><item><guid>one</guid><title>A</title><link>https://example.com/a</link></item><item><guid>one</guid><title>A</title><link>https://example.com/a</link></item></channel></rss>`, 1, true},
		{"atom", `<feed xmlns="http://www.w3.org/2005/Atom"><entry><id>one</id><title>A</title><link href="https://example.com/a"/></entry></feed>`, 1, true},
		{"json", `{"version":"https://jsonfeed.org/version/1.1","items":[]}`, 0, true},
		{"html", `<html><head></head></html>`, 0, false},
		{"unsafe item", `<rss><channel><item><link>javascript:alert(1)</link></item></channel></rss>`, 0, false},
		{"invalid json", `{"items":[]}`, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			items, err := parseFeed([]byte(tt.body))
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v err=%v", tt.valid, err)
			}
			if tt.valid && len(items) != tt.count {
				t.Fatalf("items=%d want %d", len(items), tt.count)
			}
		})
	}
}
