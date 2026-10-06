package publicview

import (
	"context"
	"slices"
	"testing"

	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5/pgtype"
)

type presentationLabels struct {
	t      *testing.T
	second pgtype.UUID
}

func (q presentationLabels) ResolveDirectoryLabel(_ context.Context, in dbgen.ResolveDirectoryLabelParams) (dbgen.ResolveDirectoryLabelRow, error) {
	q.t.Helper()
	if len(in.LabelIds) != 2 || in.LabelIds[0] != "unrelated-label" || in.LabelIds[1] != uuidText(q.second) {
		q.t.Fatalf("label membership lost: %+v", in)
	}
	row := dbgen.ResolveDirectoryLabelRow{Slug: in.Slug}
	if in.Slug == "second" && slices.Contains(in.LabelIds, uuidText(q.second)) {
		row.LabelID = q.second
	}
	return row, nil
}
func TestDirectoryLabelsResolveUnorderedMembership(t *testing.T) {
	var label pgtype.UUID
	if err := label.Scan("019ded7f-4b91-702b-9f01-acb5ea1c99d8"); err != nil {
		t.Fatal(err)
	}
	query := DirectoryQuery{TertiaryTags: []string{"first", "second"}, TertiaryLabelIDs: []string{"unrelated-label", uuidText(label)}}
	got, err := resolveDirectoryLabels(t.Context(), presentationLabels{t: t, second: label}, query)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.TertiaryLabelIDs, []string{"", uuidText(label)}) {
		t.Fatalf("presentation names changed: %+v", got.TertiaryLabelIDs)
	}
}
