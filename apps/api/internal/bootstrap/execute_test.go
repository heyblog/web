package bootstrap

import "testing"

func TestParseInvocationAcceptsOnlyServiceOrHealthcheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            []string
		wantHealthcheck bool
		wantError       bool
	}{
		{name: "service"},
		{name: "healthcheck", args: []string{"--healthcheck"}, wantHealthcheck: true},
		{name: "old mode flag", args: []string{"--mode", "production"}, wantError: true},
		{name: "old config flag", args: []string{"--config", "config/conf.yaml"}, wantError: true},
		{name: "subcommand", args: []string{"healthcheck"}, wantError: true},
		{name: "duplicate", args: []string{"--healthcheck", "--healthcheck"}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseInvocation(test.args)
			if (err != nil) != test.wantError {
				t.Fatalf("parseInvocation() error = %v, wantError %t", err, test.wantError)
			}
			if got != test.wantHealthcheck {
				t.Fatalf("healthcheck = %t, want %t", got, test.wantHealthcheck)
			}
		})
	}
}
