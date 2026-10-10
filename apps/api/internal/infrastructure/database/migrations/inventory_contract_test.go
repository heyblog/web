package migrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestMigrationFilesDescribeGreenfieldSchemas(t *testing.T) {
	t.Parallel()

	migrationFS, err := Filesystem()
	if err != nil {
		t.Fatalf("Filesystem() error = %v", err)
	}
	entries, err := fs.ReadDir(migrationFS, ".")
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	gotFiles := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			gotFiles = append(gotFiles, entry.Name())
		}
	}
	wantFiles := []string{
		"00001_extensions.sql",
		"00002_age_runtime.sql",
		"00003_identity.sql",
		"00004_directory.sql",
		"00005_directory_graph.sql",
		"00006_permissions.sql",
		"00007_content_announcements.sql",
		"00008_directory_registered_friend_links.sql",
		"00009_authentication.sql",
		"00010_site_audits.sql",
		"00011_tag_taxonomy.sql",
		"00012_fix_site_audit_rejection.sql",
		"00013_api_clients.sql",
		"00014_example_api_scope.sql",
		"00015_public_friend_graph.sql",
		"00016_incremental_data_import.sql",
		"00017_tag_management.sql",
		"00018_system_ai_settings.sql",
		"00019_tag_dictionary.sql",
		"00020_slug_generation_jobs.sql",
		"00021_site_metrics.sql",
		"00022_tag_labels.sql",
		"00023_site_claims.sql",
		"00024_owner_site_audits.sql",
		"00025_owner_management_fixes.sql",
		"00026_database_backup.sql",
		"00027_database_backup_graph.sql",
		"00028_tag_name_only.sql",
	}
	if strings.Join(gotFiles, "\n") != strings.Join(wantFiles, "\n") {
		t.Fatalf("migration files = %v, want %v", gotFiles, wantFiles)
	}

	wantTables := []string{
		"content.announcement_revisions",
		"content.announcements",
		"content.article_tags",
		"content.articles",
		"content.system_ai_settings",
		"directory.owner_friend_link_requests",
		"directory.site_audits",
		"directory.site_claims",
		"directory.site_feeds",
		"directory.site_icons",
		"directory.site_metric_events",
		"directory.site_metrics",
		"directory.site_origins",
		"directory.site_ownership_events",
		"directory.site_ownerships",
		"directory.site_resources",
		"directory.site_software_components",
		"directory.site_sources",
		"directory.site_tags",
		"directory.sites",
		"directory.slug_generation_cache",
		"directory.slug_generation_jobs",
		"directory.software_component_dependencies",
		"directory.software_components",
		"directory.tag_assignment_archive",
		"directory.tag_cascades",
		"directory.tag_identity_aliases",
		"directory.tag_labels",
		"directory.tag_slug_aliases",
		"directory.tags",
		"identity.api_client_scopes",
		"identity.api_clients",
		"identity.api_keys",
		"identity.email_verification_codes",
		"identity.oauth_identities",
		"identity.password_reset_tokens",
		"identity.user_management_permissions",
		"identity.users",
	}
	gotTables := collectCreatedTables(t, migrationFS)
	if strings.Join(gotTables, "\n") != strings.Join(wantTables, "\n") {
		t.Fatalf("created tables = %v, want %v", gotTables, wantTables)
	}
}

func TestAPIClientMigrationProtectsServiceCredentials(t *testing.T) {
	t.Parallel()

	// Given
	migrationFS, err := Filesystem()
	if err != nil {
		t.Fatalf("Filesystem() error = %v", err)
	}
	content, err := fs.ReadFile(migrationFS, "00013_api_clients.sql")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	schema := string(content)

	// When
	required := []string{
		"CREATE TABLE identity.api_clients (",
		"CREATE TABLE identity.api_client_scopes (",
		"CREATE TABLE identity.api_keys (",
		"octet_length(secret_hash) = 32",
		"CREATE FUNCTION identity.enforce_api_key_expiration()",
		"interval '90 days'",
		"GRANT SELECT, INSERT, UPDATE ON identity.api_clients TO api_runtime",
		"GRANT SELECT, INSERT, UPDATE ON identity.api_keys TO api_runtime",
	}

	// Then
	for _, fragment := range required {
		if !strings.Contains(schema, fragment) {
			t.Errorf("API client schema is missing %q", fragment)
		}
	}
}

func TestExampleAPIScopeMigrationReplacesRetiredScopeAndRestoresConstraint(t *testing.T) {
	t.Parallel()

	// Given
	migrationFS, err := Filesystem()
	if err != nil {
		t.Fatalf("Filesystem() error = %v", err)
	}
	content, err := fs.ReadFile(migrationFS, "00014_example_api_scope.sql")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	up, down, found := strings.Cut(string(content), "-- +goose Down")
	if !found {
		t.Fatal("example API scope migration is missing the down section")
	}

	// When / Then: applying the migration installs only the current scopes.
	if !strings.Contains(up, "scope IN ('data_import.write', 'example.call')") {
		t.Error("example API scope migration does not install the current scope constraint")
	}
	if strings.Contains(up, "'sites.read'") {
		t.Error("example API scope migration retains the retired sites.read scope")
	}

	// When / Then: rolling back removes example grants and restores migration 00013.
	for _, required := range []string{
		"DELETE FROM identity.api_client_scopes",
		"scope = 'example.call'",
		"UPDATE identity.api_clients AS client",
		"scope IN ('data_import.write', 'sites.read')",
	} {
		if !strings.Contains(down, required) {
			t.Errorf("example API scope migration is missing %q", required)
		}
	}
}
