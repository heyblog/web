package mail

import (
	"testing"
	"time"
)

func TestFormatValidityUsesLargestExactUnit(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		duration time.Duration
		want     string
	}{
		"seconds": {duration: 45 * time.Second, want: "45 秒"},
		"minutes": {duration: 30 * time.Minute, want: "30 分钟"},
		"hours":   {duration: 2 * time.Hour, want: "2 小时"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := FormatValidity(test.duration); got != test.want {
				t.Fatalf("FormatValidity(%s) = %q, want %q", test.duration, got, test.want)
			}
		})
	}
}
