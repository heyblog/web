package migrations

import "testing"

func TestColumnCommentRevisionsRetainIndependentInlineDocumentation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		sql     string
		inline  string
		catalog string
	}{
		{"catalog only preserves inline", "COMMENT ON COLUMN directory.site_audits.site_id IS 'New description';", "Original description", "New description"},
		{"explicit revision", "COMMENT ON COLUMN directory.site_audits.site_id IS 'New description'; -- New description", "New description", "New description"},
		{"mismatch remains detectable", "COMMENT ON COLUMN directory.site_audits.site_id IS 'New description'; -- Different description", "Different description", "New description"},
	} {
		t.Run(test.name, func(t *testing.T) {
			const column = "directory.site_audits.site_id"
			inline := map[string]string{column: "Original description"}
			catalog := map[string]string{column: "Original description"}
			collectColumnComments(test.sql, inline, catalog)
			if inline[column] != test.inline || catalog[column] != test.catalog {
				t.Fatalf("column documentation = (%q, %q), want (%q, %q)", inline[column], catalog[column], test.inline, test.catalog)
			}
		})
	}
}
