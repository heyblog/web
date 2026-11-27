package databasebackup

import (
	"encoding/json"
	"maps"
	"strconv"
	"time"
)

// Validate the original version's shape before adding or resetting fields. The
// source's complete integrity manifest is checked before this inventory is used.
func sourceInventory(current columnInventory, version int) columnInventory {
	result := make(columnInventory, len(current))
	for name, fields := range current {
		result[name] = maps.Clone(fields)
	}
	if version < emailChangeSchemaVersion {
		delete(result["directory.site_claims"], "email_change_id")
		delete(result["directory.site_claims"], "email_change_ownership_id")
	}
	if version < annotationSchemaVersion {
		delete(result["directory.sites"], "blog_status")
		delete(result, "directory.site_annotation_history")
	}
	if version < 6 {
		delete(result, "directory.site_claim_attachments")
		delete(result["directory.site_claims"], "reviewed_at")
	}
	if version == 3 {
		delete(result["identity.users"], "profile_confirmation_status")
		delete(result["identity.users"], "username_changes_used")
	}
	return result
}

func convertMonitoringRow(name string, raw json.RawMessage, version int, convertedAt time.Time) (json.RawMessage, error) {
	var row map[string]json.RawMessage
	if err := json.Unmarshal(raw, &row); err != nil {
		return nil, ErrInvalid
	}
	switch name {
	case "directory.sites":
		if version < annotationSchemaVersion {
			if _, exists := row["blog_status"]; exists {
				return nil, ErrInvalid
			}
			// The database derives the final classification from restored audits.
			row["blog_status"] = json.RawMessage(`"UNREVIEWED"`)
		}
		generation := int64(1)
		if value, exists := row["monitoring_generation"]; exists {
			var previous string
			if json.Unmarshal(value, &previous) != nil {
				return nil, ErrInvalid
			}
			parsed, err := strconv.ParseInt(previous, 10, 64)
			if err != nil || parsed < 1 || parsed == 1<<63-1 {
				return nil, ErrInvalid
			}
			generation = parsed + 1
		}
		row["status"], row["access_scope"], row["last_checked_at"] = json.RawMessage(`null`), json.RawMessage(`null`), json.RawMessage(`null`)
		row["monitoring_generation"] = json.RawMessage(strconv.Quote(strconv.FormatInt(generation, 10)))
	case "operations.schedules":
		row["enabled"], row["next_at"] = json.RawMessage(`false`), json.RawMessage(`null`)
	case "operations.runs", "operations.targets":
		var state string
		if json.Unmarshal(row["state"], &state) != nil {
			return nil, ErrInvalid
		}
		if state == "pending" || state == "queued" || state == "running" || state == "pausing" || state == "paused" || state == "cancelling" || state == "retry_wait" {
			row["state"] = json.RawMessage(`"cancelled"`)
			row["finished_at"], _ = json.Marshal(convertedAt.UTC().Format("2006-01-02T15:04:05.999999+00:00"))
			row["reason"] = json.RawMessage(`"database_restored"`)
		}
		if name == "operations.runs" {
			row["paused_at"], row["next_dispatch_at"] = json.RawMessage(`null`), json.RawMessage(`null`)
		}
		if name == "operations.targets" {
			row["lease_token"], row["lease_until"] = json.RawMessage(`null`), json.RawMessage(`null`)
			row["worker_id"] = json.RawMessage(`null`)
		}
	case "operations.request_configs":
		row["enabled"] = json.RawMessage(`false`)
		var parameters map[string]json.RawMessage
		if json.Unmarshal(row["parameters"], &parameters) != nil {
			return nil, ErrInvalid
		}
		parameters["signed"] = json.RawMessage(`true`)
		var err error
		row["parameters"], err = json.Marshal(parameters)
		if err != nil {
			return nil, err
		}
	case "operations.workers":
		row["draining"] = json.RawMessage(`true`)
		row["heartbeat_at"] = json.RawMessage(`"1970-01-01T00:00:00+00:00"`)
	case "operations.platform_backoff":
		row["retry_at"], row["probe_id"], row["probe_until"] = json.RawMessage(`null`), json.RawMessage(`null`), json.RawMessage(`null`)
		row["reason"], row["stage"] = json.RawMessage(`""`), json.RawMessage(`0`)
	case "operations.signing_policy":
		row["enabled"] = json.RawMessage(`true`)
	case "operations.signing_keys":
		row["state"] = json.RawMessage(`"revoked"`)
		row["revoked_at"], _ = json.Marshal(convertedAt.UTC().Format("2006-01-02T15:04:05.999999+00:00"))
	}
	return json.Marshal(row)
}
