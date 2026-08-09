package migrations

import (
	"bufio"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var (
	columnDefinitionPattern = regexp.MustCompile(`^\s{4}([a-z][a-z0-9_]*)\s+.+,\s+--\s+(.+)$`)
	columnCommentPattern    = regexp.MustCompile(`^COMMENT ON COLUMN ([a-z][a-z0-9_]*\.[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*) IS '([^']*)';$`)
	createTablePattern      = regexp.MustCompile(`^CREATE TABLE ([a-z][a-z0-9_]*\.[a-z][a-z0-9_]*) \($`)
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
	}
	if strings.Join(gotFiles, "\n") != strings.Join(wantFiles, "\n") {
		t.Fatalf("migration files = %v, want %v", gotFiles, wantFiles)
	}

	wantTables := []string{
		"content.announcement_revisions",
		"content.announcements",
		"directory.site_feeds",
		"directory.site_icons",
		"directory.site_origins",
		"directory.site_resources",
		"directory.site_software_components",
		"directory.site_sources",
		"directory.site_tags",
		"directory.sites",
		"directory.software_component_dependencies",
		"directory.software_components",
		"directory.tags",
		"identity.oauth_identities",
		"identity.users",
	}
	gotTables := collectCreatedTables(t, migrationFS)
	if strings.Join(gotTables, "\n") != strings.Join(wantTables, "\n") {
		t.Fatalf("created tables = %v, want %v", gotTables, wantTables)
	}
}

func TestMigrationColumnsHaveMatchingInlineAndCatalogComments(t *testing.T) {
	t.Parallel()

	migrationFS, err := Filesystem()
	if err != nil {
		t.Fatalf("Filesystem() error = %v", err)
	}
	entries, err := fs.ReadDir(migrationFS, ".")
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	inlineComments := make(map[string]string)
	catalogComments := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, readErr := fs.ReadFile(migrationFS, entry.Name())
		if readErr != nil {
			t.Fatalf("ReadFile(%q) error = %v", entry.Name(), readErr)
		}
		collectColumnComments(string(content), inlineComments, catalogComments)
	}

	if len(inlineComments) == 0 {
		t.Fatal("no inline column comments found")
	}
	for column, inlineComment := range inlineComments {
		catalogComment, ok := catalogComments[column]
		if !ok {
			t.Errorf("%s has inline comment but no COMMENT ON COLUMN", column)
			continue
		}
		if catalogComment != inlineComment {
			t.Errorf("%s comments differ: inline=%q catalog=%q", column, inlineComment, catalogComment)
		}
	}
	for column := range catalogComments {
		if _, ok := inlineComments[column]; !ok {
			t.Errorf("%s has COMMENT ON COLUMN but no inline comment", column)
		}
	}
}

func collectCreatedTables(t *testing.T, migrationFS fs.FS) []string {
	t.Helper()

	entries, err := fs.ReadDir(migrationFS, ".")
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	var tables []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, readErr := fs.ReadFile(migrationFS, entry.Name())
		if readErr != nil {
			t.Fatalf("ReadFile(%q) error = %v", entry.Name(), readErr)
		}
		for _, line := range strings.Split(string(content), "\n") {
			match := createTablePattern.FindStringSubmatch(line)
			if len(match) == 2 {
				tables = append(tables, match[1])
			}
		}
	}
	sort.Strings(tables)
	return tables
}

func collectColumnComments(content string, inlineComments, catalogComments map[string]string) {
	scanner := bufio.NewScanner(strings.NewReader(strings.Split(content, "-- +goose Down")[0]))
	currentTable := ""
	for scanner.Scan() {
		line := scanner.Text()
		if match := createTablePattern.FindStringSubmatch(line); len(match) == 2 {
			currentTable = match[1]
			continue
		}
		if currentTable != "" {
			if line == ");" {
				currentTable = ""
				continue
			}
			if match := columnDefinitionPattern.FindStringSubmatch(line); len(match) == 3 {
				inlineComments[currentTable+"."+match[1]] = strings.TrimSpace(match[2])
			}
		}
		if match := columnCommentPattern.FindStringSubmatch(line); len(match) == 3 {
			catalogComments[match[1]] = match[2]
		}
	}
}
