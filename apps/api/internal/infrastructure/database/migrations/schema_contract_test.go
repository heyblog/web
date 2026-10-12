package migrations

import (
	"bufio"
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

var (
	columnDefinitionPattern = regexp.MustCompile(`^\s{4}([a-z][a-z0-9_]*)\s+.+,?\s+--\s+(.+)$`)
	columnCommentPattern    = regexp.MustCompile(`^COMMENT ON COLUMN ([a-z][a-z0-9_]*\.[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*) IS '([^']*)';(?: -- (.+))?$`)
	createTablePattern      = regexp.MustCompile(`^CREATE TABLE ([a-z][a-z0-9_]*\.[a-z][a-z0-9_]*) \($`)
	alterTablePattern       = regexp.MustCompile(`^ALTER TABLE ([a-z][a-z0-9_]*\.[a-z][a-z0-9_]*)$`)
	alterColumnPattern      = regexp.MustCompile(`^\s+(?:ADD COLUMN(?: IF NOT EXISTS)?|ALTER COLUMN) ([a-z][a-z0-9_]*) .+[,;]\s+--\s+(.+)$`)
)

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
