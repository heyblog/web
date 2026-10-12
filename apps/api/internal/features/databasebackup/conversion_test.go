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
