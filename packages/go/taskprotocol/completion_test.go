package taskprotocol

import "testing"

func TestBusinessComplete(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		progress   PathProgress
		complete   bool
	}{
		{"monitor needs both paths", "monitor", PathProgress{Done: true, Valid: true, Path: PathResult{Outcome: "OK"}}, false},
		{"valid feed", "rss", PathProgress{Done: true, Valid: true}, true},
		{"invalid feed needs fallback", "rss", PathProgress{Done: true}, false},
		{"matching proof", "verification", PathProgress{Done: true, Valid: true, Path: PathResult{Outcome: "OK"}}, true},
		{"mismatched proof", "verification", PathProgress{Done: true, Path: PathResult{Outcome: "OK", ErrorKind: "proof_mismatch"}}, true},
		{"unfinished proof", "verification", PathProgress{Path: PathResult{Outcome: "OK"}}, false},
		{"network failure", "verification", PathProgress{Done: true, Path: PathResult{Outcome: "FAIL"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := BusinessComplete(tc.kind, tc.progress); got != tc.complete {
				t.Fatalf("complete=%v, want %v", got, tc.complete)
			}
		})
	}
}
