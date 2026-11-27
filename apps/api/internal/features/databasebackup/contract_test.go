package databasebackup

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fixtureDocumentForSchema(t *testing.T, schema int, budgets ...json.RawMessage) []byte {
	t.Helper()
	empty := sha256.Sum256(nil)
	tables := map[string][]any{}
	manifest := []Manifest{}
	for _, name := range tablesForSchema(schema) {
		tables[name] = []any{}
		manifest = append(manifest, Manifest{Dataset: Dataset{Name: name}, SHA256: hex.EncodeToString(empty[:])})
	}
	if len(budgets) > 0 {
		tables["operations.platform_budgets"] = make([]any, len(budgets))
		hash := sha256.New()
		for i, row := range budgets {
			canonical, err := canonicalRow(row)
			if err != nil {
				t.Fatal(err)
			}
			tables["operations.platform_budgets"][i] = json.RawMessage(canonical)
			_, _ = hash.Write(append(canonical, '\n'))
		}
		manifest = append(manifest, Manifest{Dataset: Dataset{Name: "operations.platform_budgets", Count: int64(len(budgets))}, SHA256: hex.EncodeToString(hash.Sum(nil))})
	}
	for _, name := range []string{"graph.vertices", "graph.edges"} {
		manifest = append(manifest, Manifest{Dataset: Dataset{Name: name}, SHA256: hex.EncodeToString(empty[:])})
	}
	data, err := json.Marshal(map[string]any{"metadata": Header{Format: Format, Version: 1, SchemaVersion: schema, GeneratedAt: time.Now(), ExcludedSystemAdminID: "00000000-0000-0000-0000-000000000001"}, "tables": tables, "graph": map[string]any{"vertices": []any{}, "edges": []any{}}, "manifest": manifest})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDocumentRejectsIntermediateFormats(t *testing.T) {
	quota := json.RawMessage(`{"platform":"CF","limit_requests":"100","used_requests":"2","period_seconds":3600,"period_start":"2026-01-01T00:00:00Z"}`)
	for _, data := range [][]byte{fixtureDocumentForSchema(t, schemaVersion, quota), bytes.Replace(fixtureDocumentForSchema(t, schemaVersion), []byte(`"schema_version":9`), []byte(`"schema_version":7`), 1), fixtureDocumentForSchema(t, 10)} {
		if _, err := readDocument(bytes.NewReader(data), nil); !errors.Is(err, ErrInvalid) {
			t.Fatalf("intermediate format accepted: %v", err)
		}
	}
}

func TestDocumentRejectsMissingUnknownAndTamperedDatasets(t *testing.T) {
	t.Parallel()
	valid := fixtureDocumentForSchema(t, 3)
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

func TestDocumentAcceptsSupportedSchemaVersions(t *testing.T) {
	// Given a valid transfer with no schema-specific rows.
	for _, version := range []int{3, 4, 5, 6, 7, 8, 9} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			// When its source schema is a supported version.
			data := fixtureDocumentForSchema(t, version)
			_, err := readDocument(bytes.NewReader(data), nil)
			// Then integrity validation still accepts it.
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDocumentRequiresVersionSpecificAttachmentInventory(t *testing.T) {
	var data map[string]json.RawMessage
	if err := json.Unmarshal(fixtureDocumentForSchema(t, 3), &data); err != nil {
		t.Fatal(err)
	}
	var header Header
	if err := json.Unmarshal(data["metadata"], &header); err != nil {
		t.Fatal(err)
	}
	header.SchemaVersion = 6
	data["metadata"], _ = json.Marshal(header)
	missing, _ := json.Marshal(data)
	if _, err := readDocument(bytes.NewReader(missing), nil); !errors.Is(err, ErrInvalid) {
		t.Fatal("schema six accepted missing attachment dataset")
	}
	var tables map[string]json.RawMessage
	_ = json.Unmarshal(data["tables"], &tables)
	tables["directory.site_claim_attachments"] = json.RawMessage(`[]`)
	data["tables"], _ = json.Marshal(tables)
	var manifest []Manifest
	_ = json.Unmarshal(data["manifest"], &manifest)
	empty := sha256.Sum256(nil)
	manifest = append(manifest, Manifest{Dataset: Dataset{Name: "directory.site_claim_attachments"}, SHA256: hex.EncodeToString(empty[:])})
	data["manifest"], _ = json.Marshal(manifest)
	valid, _ := json.Marshal(data)
	if _, err := readDocument(bytes.NewReader(valid), nil); err != nil {
		t.Fatal(err)
	}
	header.SchemaVersion = 5
	data["metadata"], _ = json.Marshal(header)
	extra, _ := json.Marshal(data)
	if _, err := readDocument(bytes.NewReader(extra), nil); !errors.Is(err, ErrInvalid) {
		t.Fatal("legacy schema accepted unexpected attachment dataset")
	}
}

func TestDocumentRejectsMismatchedAnnotationHistoryInventory(t *testing.T) {
	for _, test := range []struct {
		name       string
		inventory  int
		version    string
		mislabeled string
	}{
		{"legacy with history", 7, `"schema_version":7`, `"schema_version":6`},
		{"current without history", 6, `"schema_version":6`, `"schema_version":7`},
	} {
		t.Run(test.name, func(t *testing.T) {
			archive := fixtureDocumentForSchema(t, test.inventory)
			archive = bytes.Replace(archive, []byte(test.version), []byte(test.mislabeled), 1)
			if _, err := readDocument(bytes.NewReader(archive), nil); !errors.Is(err, ErrInvalid) {
				t.Fatalf("mismatched inventory accepted: %v", err)
			}
		})
	}
}
