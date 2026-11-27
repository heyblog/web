package monitoring

import "testing"

func TestHealthBoundaries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		failures int
		want     string
	}{{0, "OK"}, {1, "OK"}, {2, "WARN"}, {9, "WARN"}, {10, "DOWN"}} {
		rounds := make([]Round, 10)
		for i := range rounds {
			rounds[i] = Round{CN: OK, GL: Unknown}
			if i < test.failures {
				rounds[i] = Round{CN: Fail, GL: Fail}
			}
		}
		status, _ := Summarize(rounds, nil)
		if status == nil || *status != test.want {
			t.Fatalf("failures %d: status=%v", test.failures, status)
		}
		status, _ = Summarize(rounds[:9], nil)
		if status != nil {
			t.Fatal("insufficient history changed health")
		}
	}
}

func TestScopeRequiresFiveMatchingNonemptyCandidates(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		round Round
		want  string
	}{{Round{CN: OK, GL: OK}, "ALL"}, {Round{CN: OK, GL: Fail}, "CN"}, {Round{CN: Fail, GL: OK}, "GL"}} {
		rounds := []Round{test.round, test.round, test.round, test.round, test.round}
		_, scope := Summarize(nil, rounds)
		if scope == nil || *scope != test.want {
			t.Fatalf("scope=%v, want %s", scope, test.want)
		}
		_, scope = Summarize(nil, rounds[:4])
		if scope != nil {
			t.Fatal("short scope window updated")
		}
		rounds[2] = Round{CN: Fail, GL: Fail}
		_, scope = Summarize(nil, rounds)
		if scope != nil {
			t.Fatal("empty candidate did not block update")
		}
		rounds[2] = Round{CN: Unknown, GL: OK}
		_, scope = Summarize(nil, rounds)
		if scope != nil {
			t.Fatal("incomplete candidate updated scope")
		}
	}
}
