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
	columnCommentPattern    = regexp.MustCompile(`^COMMENT ON COLUMN ([a-z][a-z0-9_]*\.[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*) IS '([^']*)';(?: -- (.+))?$`)
	createTablePattern      = regexp.MustCompile(`^CREATE TABLE ([a-z][a-z0-9_]*\.[a-z][a-z0-9_]*) \($`)
	alterTablePattern       = regexp.MustCompile(`^ALTER TABLE ([a-z][a-z0-9_]*\.[a-z][a-z0-9_]*)$`)
	alterColumnPattern      = regexp.MustCompile(`^\s+(?:ADD COLUMN(?: IF NOT EXISTS)?|ALTER COLUMN) ([a-z][a-z0-9_]*) .+[,;]\s+--\s+(.+)$`)
)

func TestTagTaxonomyMigrationDefinesSharedCascadesAndArticlePreparation(t *testing.T) {
	t.Parallel()

	migrationFS, err := Filesystem()
	if err != nil {
		t.Fatalf("Filesystem() error = %v", err)
	}
	content, err := fs.ReadFile(migrationFS, "00011_tag_taxonomy.sql")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	schema := string(content)

	for _, required := range []string{
		"CREATE TABLE directory.tag_cascades (",
		"scope text NOT NULL",
		"level1_tag_id uuid NOT NULL",
		"level2_tag_id uuid NOT NULL",
		"tag_cascade_id uuid NOT NULL",
		"CREATE TABLE content.articles (",
		"CREATE TABLE content.article_tags (",
		"CHECK (role IN ('TERTIARY', 'WARNING'))",
		"position BETWEEN 1 AND 20",
		"ADD COLUMN IF NOT EXISTS review_draft_snapshot jsonb",
	} {
		if !strings.Contains(schema, required) {
			t.Errorf("tag taxonomy schema is missing %q", required)
		}
	}
}

func TestSiteAuditsPreserveImmutableSnapshotsAndAnonymousLookup(t *testing.T) {
	t.Parallel()

	migrationFS, err := Filesystem()
	if err != nil {
		t.Fatalf("Filesystem() error = %v", err)
	}
	content, err := fs.ReadFile(migrationFS, "00010_site_audits.sql")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	schema := string(content)

	for _, required := range []string{
		"CREATE TABLE directory.site_audits (",
		"lookup_secret_hash bytea NOT NULL",
		"base_snapshot jsonb",
		"proposed_snapshot jsonb NOT NULL",
		"review_draft_snapshot jsonb",
		"review_draft_revision bigint NOT NULL DEFAULT 0",
		"final_snapshot jsonb",
		"INSERT INTO directory.software_components (",
		"'其他'",
		"CREATE UNIQUE INDEX site_audits_pending_site_unique_idx",
		"CREATE TRIGGER site_audits_preserve_submission",
		"GRANT SELECT, INSERT, UPDATE ON directory.site_audits TO api_runtime",
	} {
		if !strings.Contains(schema, required) {
			t.Errorf("site audit schema is missing %q", required)
		}
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

func TestDirectoryTagsAndIconsUseReviewedConstraints(t *testing.T) {
	t.Parallel()

	migrationFS, err := Filesystem()
	if err != nil {
		t.Fatalf("Filesystem() error = %v", err)
	}
	content, err := fs.ReadFile(migrationFS, "00004_directory.sql")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	schema := string(content)

	for _, required := range []string{
		"CONSTRAINT tags_normalized_name_unique UNIQUE (normalized_name)",
		"CONSTRAINT tags_slug_unique UNIQUE (slug)",
		"role text NOT NULL, -- Assignment role: PRIMARY, SECONDARY, or WARNING.",
		"CHECK (role IN ('PRIMARY', 'SECONDARY', 'WARNING'))",
		"octet_length(content) BETWEEN 1 AND 1048576",
	} {
		if !strings.Contains(schema, required) {
			t.Errorf("directory schema is missing %q", required)
		}
	}
	for _, removed := range []string{"tag_kind", "topic_role", "assigned_by", "tags_kind_check"} {
		if strings.Contains(schema, removed) {
			t.Errorf("directory schema still contains removed tag concept %q", removed)
		}
	}
}

func TestDirectorySitesUsesJoinedAtAsItsOnlyCreationTimestamp(t *testing.T) {
	t.Parallel()

	// Given
	migrationFS, err := Filesystem()
	if err != nil {
		t.Fatalf("Filesystem() error = %v", err)
	}
	content, err := fs.ReadFile(migrationFS, "00004_directory.sql")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	sitesDefinition := strings.Split(strings.Split(string(content), "CREATE TABLE directory.sites (")[1], ");")[0]

	// When
	hasJoinedAt := strings.Contains(sitesDefinition, "joined_at timestamptz NOT NULL DEFAULT now()")
	hasCreatedAt := strings.Contains(sitesDefinition, "created_at")

	// Then
	if !hasJoinedAt {
		t.Error("directory.sites is missing its joined_at creation timestamp")
	}
	if hasCreatedAt {
		t.Error("directory.sites still contains the redundant created_at timestamp")
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
	alteredTable := ""
	for scanner.Scan() {
		line := scanner.Text()
		if match := createTablePattern.FindStringSubmatch(line); len(match) == 2 {
			currentTable = match[1]
			continue
		}
		if match := alterTablePattern.FindStringSubmatch(line); len(match) == 2 {
			alteredTable = match[1]
			continue
		}
		if alteredTable != "" {
			if match := alterColumnPattern.FindStringSubmatch(line); len(match) == 3 {
				inlineComments[alteredTable+"."+match[1]] = strings.TrimSpace(match[2])
			}
			if strings.HasSuffix(strings.TrimSpace(line), ";") {
				alteredTable = ""
			}
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
		if match := columnCommentPattern.FindStringSubmatch(line); len(match) == 4 {
			catalogComments[match[1]] = match[2]
			if match[3] != "" {
				inlineComments[match[1]] = match[3]
			}
		}
	}
}
