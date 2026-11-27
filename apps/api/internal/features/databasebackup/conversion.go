package databasebackup

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
)

// Conversion runs only after the complete original manifest has been verified.
// The source file and its checksum remain unchanged for inspect/restore matching.
func (doc *document) prepareConversion(ctx context.Context, file *os.File) error {
	if doc.header.SchemaVersion != 3 {
		return nil
	}
	doc.legacyOAuth = make(map[string]bool)
	if _, err := file.Seek(doc.offsets["identity.oauth_identities"], io.SeekStart); err != nil {
		return err
	}
	_, err := readRows(json.NewDecoder(contextReader{ctx, file}), "identity.oauth_identities", func(_ string, raw json.RawMessage) error {
		var identity struct {
			UserID string `json:"user_id"`
		}
		if err := json.Unmarshal(raw, &identity); err != nil || identity.UserID == "" {
			return ErrInvalid
		}
		doc.legacyOAuth[identity.UserID] = true
		return nil
	})
	return err
}

func (doc document) convertRow(name string, raw json.RawMessage) (json.RawMessage, error) {
	if name == "directory.sites" || strings.HasPrefix(name, "operations.") {
		if err := validateMonitoringSource(name, raw, doc.header.SchemaVersion); err != nil {
			return nil, err
		}
		return convertMonitoringRow(name, raw, doc.header.SchemaVersion, doc.convertedAt)
	}
	if name == "directory.site_claims" {
		return convertSiteClaim(doc.header.SchemaVersion, raw)
	}
	if name == "directory.site_claim_attachments" {
		var row map[string]json.RawMessage
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, ErrInvalid
		}
		// Database backups carry metadata only. Restoring must never grant access
		// to, or schedule deletion of, the source instance's private objects.
		if err := validateSourceFields(row, []sourceField{
			{"status", "enum", false, []string{"UPLOADING", "READY", "BOUND", "CLEANUP_PENDING", "ARCHIVED", "DELETED"}},
			{"storage_key", "text", false, nil}, {"storage_identity", "text", false, nil},
			{"cleanup_requested_at", "time", true, nil}, {"archive_at", "time", true, nil}, {"purge_at", "time", true, nil},
		}); err != nil {
			return nil, err
		}
		row["status"] = json.RawMessage(`"DELETED"`)
		row["storage_key"] = json.RawMessage(`""`)
		row["storage_identity"] = json.RawMessage(`""`)
		for _, key := range []string{"cleanup_requested_at", "archive_at", "purge_at"} {
			row[key] = json.RawMessage(`null`)
		}
		return json.Marshal(row)
	}
	if doc.header.SchemaVersion != 3 || name != "identity.users" {
		return raw, nil
	}
	var row map[string]json.RawMessage
	if err := json.Unmarshal(raw, &row); err != nil {
		return nil, ErrInvalid
	}
	if _, exists := row["profile_confirmation_status"]; exists {
		return nil, ErrInvalid
	}
	if _, exists := row["username_changes_used"]; exists {
		return nil, ErrInvalid
	}
	var id string
	if err := json.Unmarshal(row["id"], &id); err != nil {
		return nil, ErrInvalid
	}
	row["profile_confirmation_status"] = json.RawMessage(`"NOT_REQUIRED"`)
	if doc.legacyOAuth[id] {
		row["profile_confirmation_status"] = json.RawMessage(`"PENDING"`)
	}
	row["username_changes_used"] = json.RawMessage(`0`)
	return json.Marshal(row)
}

func convertSiteClaim(version int, raw json.RawMessage) (json.RawMessage, error) {
	var row map[string]json.RawMessage
	if err := json.Unmarshal(raw, &row); err != nil || row == nil {
		return nil, ErrInvalid
	}
	if version < 6 {
		if _, exists := row["reviewed_at"]; exists {
			return nil, ErrInvalid
		}
		row["reviewed_at"] = json.RawMessage(`null`)
	}
	if version < emailChangeSchemaVersion {
		for _, key := range []string{"email_change_id", "email_change_ownership_id"} {
			if _, exists := row[key]; exists {
				return nil, ErrInvalid
			}
			row[key] = json.RawMessage(`null`)
		}
	} else if string(row["email_change_id"]) != "null" && string(row["status"]) == `"PENDING"` {
		// In-progress identity requests are instance-local and are never restored.
		row["status"] = json.RawMessage(`"CANCELLED"`)
		row["reviewed_at"] = row["created_at"]
		return json.Marshal(row)
	} else {
		return raw, nil
	}
	return json.Marshal(row)
}
