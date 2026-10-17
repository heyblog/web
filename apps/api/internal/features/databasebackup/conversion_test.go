package databasebackup

import (
	"encoding/json"
	"testing"
)

func TestAccountBackupConversionUsesOAuthBindings(t *testing.T) {
	doc := document{header: Header{SchemaVersion: 3}, legacyOAuth: map[string]bool{"oauth-user": true}}
	for _, test := range []struct{ id, state string }{{"oauth-user", "PENDING"}, {"local-user", "NOT_REQUIRED"}} {
		raw, err := doc.convertRow("identity.users", json.RawMessage(`{"id":"`+test.id+`","username":"retained"}`))
		if err != nil {
			t.Fatal(err)
		}
		var row struct {
			Username string `json:"username"`
			State    string `json:"profile_confirmation_status"`
			Used     int    `json:"username_changes_used"`
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			t.Fatal(err)
		}
		if row.State != test.state || row.Used != 0 || row.Username != "retained" {
			t.Fatalf("converted account: %+v", row)
		}
	}
	current := document{header: Header{SchemaVersion: 4}}
	original := json.RawMessage(`{"id":"oauth-user","profile_confirmation_status":"CONFIRMED","username_changes_used":1}`)
	raw, err := current.convertRow("identity.users", original)
	if err != nil || string(raw) != string(original) {
		t.Fatalf("current state changed: %s / %v", raw, err)
	}
}

func TestLegacyClaimConversionPreservesOriginalShortEvidence(t *testing.T) {
	for _, version := range []int{3, 4, 5} {
		doc := document{header: Header{SchemaVersion: version}}
		raw, err := doc.convertRow("directory.site_claims", json.RawMessage(`{"evidence":"Historical proof"}`))
		if err != nil {
			t.Fatal(err)
		}
		var row map[string]json.RawMessage
		if err := json.Unmarshal(raw, &row); err != nil {
			t.Fatal(err)
		}
		if string(row["evidence"]) != `"Historical proof"` || string(row["reviewed_at"]) != "null" {
			t.Fatalf("legacy claim changed: %s", raw)
		}
		if _, err := doc.convertRow("directory.site_claims", json.RawMessage(`{"reviewed_at":null}`)); err == nil {
			t.Fatal("legacy row accepted new field")
		}
	}
}

func TestRestoredEvidenceNeverRetainsSourceObjectAuthority(t *testing.T) {
	doc := document{header: Header{SchemaVersion: 6}}
	for _, status := range []string{"UPLOADING", "READY", "BOUND", "CLEANUP_PENDING", "ARCHIVED"} {
		raw, err := doc.convertRow("directory.site_claim_attachments", json.RawMessage(`{"filename":"proof.pdf","status":"`+status+`","storage_key":"backup/foreign","storage_identity":"source","archive_at":"2024-01-01","purge_at":"2024-02-01"}`))
		if err != nil {
			t.Fatal(err)
		}
		var row map[string]json.RawMessage
		_ = json.Unmarshal(raw, &row)
		if string(row["status"]) != `"DELETED"` || string(row["storage_key"]) != `""` || string(row["storage_identity"]) != `""` || string(row["filename"]) != `"proof.pdf"` {
			t.Fatalf("source authority retained: %s", raw)
		}
	}
}
