package databasebackup

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"
)

type sourceField struct {
	name     string
	kind     string
	nullable bool
	values   []string
}

// Fields overwritten during recovery must be validated before conversion can
// erase invalid source values. Other columns retain database constraint checks.
func validateMonitoringSource(name string, raw json.RawMessage, version int) error {
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return ErrInvalid
	}
	var fields []sourceField
	switch name {
	case "directory.sites":
		values := []string{"CN", "GL", "ALL"}
		if version < schemaVersion {
			values = []string{"CN_ONLY", "GLOBAL_ONLY", "ALL"}
		}
		fields = append(fields, sourceField{"access_scope", "enum", version >= schemaVersion, values})
		if version >= schemaVersion {
			fields = append(fields, sourceField{"status", "enum", true, []string{"OK", "WARN", "DOWN"}}, sourceField{"last_checked_at", "time", true, nil})
		}
	case "operations.schedules":
		fields = []sourceField{{"enabled", "bool", false, nil}, {"next_at", "time", true, nil}}
	case "operations.runs", "operations.targets":
		states := []string{"queued", "running", "pausing", "paused", "cancelling", "succeeded", "partial_failed", "failed", "timed_out", "cancelled"}
		if name == "operations.targets" {
			states = []string{"pending", "running", "retry_wait", "succeeded", "partial_failed", "failed", "timed_out", "cancelled", "skipped"}
		}
		fields = []sourceField{{"state", "enum", false, states}, {"finished_at", "time", true, nil}, {"reason", "text", false, nil}}
		if name == "operations.runs" {
			fields = append(fields, sourceField{"paused_at", "time", true, nil}, sourceField{"next_dispatch_at", "time", true, nil})
		} else {
			fields = append(fields, sourceField{"lease_token", "uuid", true, nil}, sourceField{"lease_until", "time", true, nil}, sourceField{"worker_id", "uuid", true, nil})
		}
	case "operations.signing_policy":
		fields = []sourceField{{"enabled", "bool", false, nil}}
	case "operations.signing_keys":
		fields = []sourceField{{"state", "enum", false, []string{"generated", "published", "active", "retired", "revoked"}}, {"revoked_at", "time", true, nil}}
	case "operations.request_configs":
		fields = []sourceField{{"enabled", "bool", false, nil}}
		var parameters map[string]json.RawMessage
		if json.Unmarshal(row["parameters"], &parameters) != nil || parameters == nil {
			return ErrInvalid
		}
		if err := validateSourceFields(parameters, []sourceField{{"signed", "bool", false, nil}}); err != nil {
			return err
		}
	case "operations.workers":
		fields = []sourceField{{"draining", "bool", false, nil}, {"heartbeat_at", "time", false, nil}}
	case "operations.platform_backoff":
		fields = []sourceField{{"retry_at", "time", true, nil}, {"probe_id", "uuid", true, nil}, {"probe_until", "time", true, nil}, {"reason", "text", false, nil}}
		var stage int
		if string(row["stage"]) == "null" || json.Unmarshal(row["stage"], &stage) != nil || stage < 0 || stage > 6 {
			return ErrInvalid
		}
	}
	return validateSourceFields(row, fields)
}

func validateSourceFields(row map[string]json.RawMessage, fields []sourceField) error {
	for _, field := range fields {
		value, exists := row[field.name]
		if !exists {
			return ErrInvalid
		}
		if string(value) == "null" {
			if field.nullable {
				continue
			}
			return ErrInvalid
		}
		if field.kind == "bool" {
			var b bool
			if json.Unmarshal(value, &b) != nil {
				return ErrInvalid
			}
			continue
		}
		var text string
		if json.Unmarshal(value, &text) != nil {
			return ErrInvalid
		}
		switch field.kind {
		case "enum":
			if !slices.Contains(field.values, text) {
				return ErrInvalid
			}
		case "time":
			if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
				return ErrInvalid
			}
		case "uuid":
			if _, err := uuid.Parse(text); err != nil {
				return ErrInvalid
			}
		}
	}
	return nil
}
