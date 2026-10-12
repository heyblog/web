package databasebackup

import (
	"context"
	"encoding/json"
	"io"
	"os"
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
