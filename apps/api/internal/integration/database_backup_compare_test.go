//go:build integration

package integration_test

import (
	"encoding/json"
	"io"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type backupData struct {
	Tables map[string][]map[string]any `json:"tables"`
	Graph  map[string][]map[string]any `json:"graph"`
}

func readBackupData(t *testing.T, file *os.File) backupData {
	t.Helper()
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	var data backupData
	if err := decoder.Decode(&data); err != nil {
		t.Fatal(err)
	}
	return data
}

func compareDatabaseBackups(t *testing.T, source, target *os.File, oldAdmin, newAdmin string) {
	t.Helper()
	want, got := readBackupData(t, source), readBackupData(t, target)
	if len(want.Tables) != 53 || len(got.Tables) != 53 {
		t.Fatal("incomplete table inventory")
	}
	for table, rows := range want.Tables {
		if len(rows) == 0 && !strings.HasPrefix(table, "operations.") && table != "directory.site_monitoring_rounds" {
			t.Errorf("roundtrip fixture has no rows for %s", table)
		}
		for _, row := range rows {
			if table == "operations.signing_policy" {
				row["enabled"] = true
				revision, err := row["revision"].(json.Number).Int64()
				if err != nil {
					t.Fatal(err)
				}
				row["revision"] = json.Number(strconv.FormatInt(revision+1, 10))
			}
			if table == "operations.request_configs" {
				row["enabled"] = false
				parameters, ok := row["parameters"].(map[string]any)
				if !ok {
					t.Fatal("invalid request configuration")
				}
				parameters["signed"] = true
			}
			for _, field := range []string{"created_by", "updated_by", "granted_by", "revoked_by", "reviewed_by", "published_by", "archived_by", "changed_by", "owner_id", "user_id", "actor_id", "submitter_user_id", "identified_by"} {
				if row[field] == oldAdmin {
					row[field] = newAdmin
				}
			}
			if table == "directory.sites" {
				previous, err := strconv.ParseInt(row["monitoring_generation"].(string), 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				row["monitoring_generation"] = strconv.FormatInt(previous+1, 10)
				row["status"], row["access_scope"], row["last_checked_at"] = nil, nil, nil
			}
			if table == "directory.site_claim_attachments" {
				row["status"] = "DELETED"
				row["storage_key"] = ""
				row["storage_identity"] = ""
				for _, key := range []string{"cleanup_requested_at", "archive_at", "purge_at"} {
					row[key] = nil
				}
			}
			if table == "directory.tag_assignment_archive" {
				if snapshot, ok := row["snapshot"].(map[string]any); ok && snapshot["actor_id"] == oldAdmin {
					snapshot["actor_id"] = newAdmin
				}
			}
			if table == "directory.site_origins" {
				metadata, ok := row["metadata"].(map[string]any)
				if ok && (metadata["channel"] == "ACCOUNT_SUBMISSION" || metadata["channel"] == "OWNER_FRIEND_LINK") {
					if metadata["user_id"] == oldAdmin {
						metadata["user_id"] = newAdmin
					}
					if recommendations, ok := metadata["recommendations"].([]any); ok {
						for _, item := range recommendations {
							if recommendation, ok := item.(map[string]any); ok && recommendation["user_id"] == oldAdmin {
								recommendation["user_id"] = newAdmin
							}
						}
					}
				}
			}
			if table == "directory.site_audits" {
				for _, field := range []string{"base_snapshot", "proposed_snapshot", "final_snapshot"} {
					if snapshot, ok := row[field].(map[string]any); ok {
						if snapshot["confirmed_by"] == oldAdmin {
							snapshot["confirmed_by"] = newAdmin
						}
					}
				}
			}
			if table == "directory.site_annotation_history" {
				if snapshot, ok := row["after_snapshot"].(map[string]any); ok && snapshot["confirmed_by"] == oldAdmin {
					snapshot["confirmed_by"] = newAdmin
				}
			}
		}
		if !reflect.DeepEqual(sortedBackupRows(t, rows), sortedBackupRows(t, got.Tables[table])) {
			t.Errorf("restored content differs for %s", table)
		}
	}
	if !reflect.DeepEqual(want.Graph, got.Graph) {
		t.Error("restored graph differs")
	}
}

func sortedBackupRows(t *testing.T, rows []map[string]any) []string {
	t.Helper()
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		encoded, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, string(encoded))
	}
	slices.Sort(result)
	return result
}
