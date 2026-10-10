package databasebackup

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func fixtureDocument(t *testing.T) []byte {
	t.Helper()
	empty := sha256.Sum256(nil)
	tables := map[string][]any{}
	manifest := []Manifest{}
	for _, name := range tableNames {
		tables[name] = []any{}
		manifest = append(manifest, Manifest{Dataset: Dataset{Name: name}, SHA256: hex.EncodeToString(empty[:])})
	}
	for _, name := range []string{"graph.vertices", "graph.edges"} {
		manifest = append(manifest, Manifest{Dataset: Dataset{Name: name}, SHA256: hex.EncodeToString(empty[:])})
	}
	data, err := json.Marshal(map[string]any{"metadata": Header{Format: Format, Version: 1, SchemaVersion: 3, GeneratedAt: time.Now(), ExcludedSystemAdminID: "00000000-0000-0000-0000-000000000001"}, "tables": tables, "graph": map[string]any{"vertices": []any{}, "edges": []any{}}, "manifest": manifest})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDocumentRejectsMissingUnknownAndTamperedDatasets(t *testing.T) {
	t.Parallel()
	valid := fixtureDocument(t)
	if _, err := readDocument(bytes.NewReader(valid), nil); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		replace string
		with    string
	}{
		{"missing", ",\"identity.users\":[]", ""},
		{"unknown", "\"identity.users\":[]", "\"identity.unknown\":[]"},
		{"tampered", "\"identity.users\":[]", "\"identity.users\":[{\"id\":\"changed\"}]"},
		{"version", "\"schema_version\":3", "\"schema_version\":1"},
		{"prior schema", "\"schema_version\":3", "\"schema_version\":2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := strings.Replace(string(valid), test.replace, test.with, 1)
			if data == string(valid) {
				t.Fatal("test failed to mutate fixture")
			}
			if _, err := readDocument(strings.NewReader(data), nil); !errors.Is(err, ErrInvalid) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestDuplicateJSONKeysAreRejectedAtEveryDepth(t *testing.T) {
	t.Parallel()
	for _, data := range []string{`{"metadata":{},"metadata":{}}`, `{"tables":{"identity.users":[{"id":"a","id":"b"}]}}`, `{"graph":{"vertices":[],"vertices":[]}}`} {
		if err := scanJSON(json.NewDecoder(strings.NewReader(data)), 0); !errors.Is(err, ErrInvalid) {
			t.Fatalf("duplicate accepted: %v", err)
		}
	}
}

func TestCanonicalRowsPreserveLargeJSONNumbers(t *testing.T) {
	t.Parallel()
	row, err := canonicalRow([]byte(`{"profile":{"number":9007199254740993},"revision":"9223372036854775807"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(row, []byte("9007199254740993")) || !bytes.Contains(row, []byte(`"9223372036854775807"`)) {
		t.Fatalf("precision changed: %s", row)
	}
}

func TestRowContractRejectsUnknownFieldsAndUnsafeBigints(t *testing.T) {
	t.Parallel()
	inventory := columnInventory{"table": {"id": "uuid", "count": "bigint", "bytes": "bytea"}}
	for _, row := range []string{`{"id":"x","count":9007199254740993,"bytes":"AA=="}`, `{"id":"x","count":"9223372036854775808","bytes":"AA=="}`, `{"id":"x","count":"1","bytes":"invalid"}`, `{"id":"x","count":"1","bytes":"AA==","unknown":true}`} {
		if err := validateRow("table", []byte(row), inventory); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid row accepted: %v", err)
		}
	}
}
