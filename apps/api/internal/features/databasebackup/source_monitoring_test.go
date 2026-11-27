package databasebackup

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMonitoringConversionRejectsInvalidOriginalValues(t *testing.T) {
	for _, test := range []struct{ name, row string }{
		{"operations.runs", `{"state":"completed","reason":"","finished_at":null,"paused_at":null,"next_dispatch_at":null}`},
		{"operations.signing_keys", `{"state":{},"revoked_at":null}`},
		{"operations.signing_keys", `{"state":"active","revoked_at":"invalid"}`},
		{"operations.schedules", `{"enabled":"invalid","next_at":null}`},
		{"operations.schedules", `{"enabled":null,"next_at":null}`},
		{"operations.targets", `{"state":"running","reason":"","finished_at":null,"lease_token":12,"lease_until":null,"worker_id":null}`},
		{"directory.sites", `{"access_scope":"ALL","status":"fake","last_checked_at":null}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := document{header: Header{SchemaVersion: schemaVersion}}
			if _, err := doc.convertRow(test.name, json.RawMessage(test.row)); err == nil {
				t.Fatal("invalid source was normalized by conversion")
			}
		})
	}
}

func TestTaskRecoveryRejectsInvalidFieldsBeforeReset(t *testing.T) {
	for _, test := range []struct{ name, row string }{
		{"operations.request_configs", `{"enabled":true,"parameters":{"signed":"false"}}`},
		{"operations.request_configs", `{"enabled":true,"parameters":{"signed":null}}`},
		{"operations.request_configs", `{"enabled":true,"parameters":null}`},
		{"operations.platform_backoff", `{"retry_at":null,"probe_id":null,"probe_until":null,"reason":"","stage":7}`},
		{"operations.platform_backoff", `{"retry_at":null,"probe_id":null,"probe_until":null,"reason":12,"stage":0}`},
		{"operations.targets", `{"state":"running","reason":"","finished_at":null,"lease_token":null,"lease_until":null,"worker_id":"invalid"}`},
		{"operations.runs", `{"state":"paused","reason":"","finished_at":null,"paused_at":"invalid","next_dispatch_at":null}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given source fields that recovery would otherwise overwrite.
			doc := document{header: Header{SchemaVersion: schemaVersion}}
			// When converting the untrusted row, malformed values must be rejected.
			if _, err := doc.convertRow(test.name, json.RawMessage(test.row)); err == nil {
				t.Fatal("invalid recovery source was accepted")
			}
		})
	}
}

func TestTaskRecoveryCancelsEveryActiveRunAndClearsDispatchControls(t *testing.T) {
	for _, state := range []string{"queued", "running", "pausing", "paused", "cancelling"} {
		t.Run(state, func(t *testing.T) {
			// Given a schema-nine active run with retained execution controls.
			doc := document{header: Header{SchemaVersion: schemaVersion}, convertedAt: time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)}
			row := json.RawMessage(`{"state":"` + state + `","finished_at":null,"paused_at":"2026-10-21T00:00:00Z","next_dispatch_at":"2026-10-23T00:00:00Z","reason":""}`)
			// When preparing recovery.
			raw, err := doc.convertRow("operations.runs", row)
			if err != nil {
				t.Fatal(err)
			}
			var recovered struct {
				State          string     `json:"state"`
				Reason         string     `json:"reason"`
				PausedAt       *time.Time `json:"paused_at"`
				NextDispatchAt *time.Time `json:"next_dispatch_at"`
			}
			if err := json.Unmarshal(raw, &recovered); err != nil {
				t.Fatal(err)
			}
			// Then no restored run remains resumable or retains a dispatch reservation.
			if recovered.State != "cancelled" || recovered.Reason != "database_restored" || recovered.PausedAt != nil || recovered.NextDispatchAt != nil {
				t.Fatalf("unsafe restored run: %+v", recovered)
			}
		})
	}
}

func TestTaskRecoveryRejectsMalformedReasonBeforeReset(t *testing.T) {
	for _, name := range []string{"operations.runs", "operations.targets"} {
		for _, reason := range []string{`null`, `{}`, `42`} {
			t.Run(name+reason, func(t *testing.T) {
				row := json.RawMessage(`{"state":"running","finished_at":null,"reason":` + reason + `,
					"paused_at":null,"next_dispatch_at":null,"worker_id":null,"lease_token":null,"lease_until":null}`)
				doc := document{header: Header{SchemaVersion: schemaVersion}}
				if _, err := doc.convertRow(name, row); err == nil {
					t.Fatal("malformed task reason was reset without validation")
				}
			})
		}
	}
}
