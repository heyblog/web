package taskcenter

import "testing"

func TestFallbackRespectsRequestPolicyFailures(t *testing.T) {
	for _, test := range []struct {
		kind    string
		valid   bool
		allowed bool
	}{
		{"", false, true},
		{"", true, false},
		{"signature", false, false},
		{"cancelled", false, false},
		{"security", false, false},
		{"cancelled", false, false},
		{"platform", false, true},
		{"target", false, true},
		{"invalid_feed", false, true},
	} {
		t.Run(test.kind, func(t *testing.T) {
			result := ExecutionResult{BusinessValid: test.valid, Paths: []PathResult{{ErrorKind: test.kind}}}
			if got := allowFallback(result); got != test.allowed {
				t.Fatalf("fallback=%v, want %v", got, test.allowed)
			}
		})
	}
}
