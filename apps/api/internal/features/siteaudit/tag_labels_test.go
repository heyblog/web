package siteaudit

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type labelFixture struct {
	labels map[pgtype.UUID]dbgen.DirectoryTagLabel
}

func (fixture labelFixture) GetTagLabel(_ context.Context, id pgtype.UUID) (dbgen.DirectoryTagLabel, error) {
	label, ok := fixture.labels[id]
	if !ok {
		return dbgen.DirectoryTagLabel{}, pgx.ErrNoRows
	}
	return label, nil
}
func (fixture labelFixture) GetTagLabelByNormalizedName(_ context.Context, name string) (dbgen.DirectoryTagLabel, error) {
	for _, label := range fixture.labels {
		if label.NormalizedName == name {
			return label, nil
		}
	}
	return dbgen.DirectoryTagLabel{}, pgx.ErrNoRows
}

func TestSelectedLabelRetainsNameAndSharedConcept(t *testing.T) {
	t.Parallel()
	// Given two confirmed labels belonging to the same semantic tag.
	concept := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	chinese := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	english := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	canonical := dbgen.DirectoryTagDictionary{ID: concept, DefaultLabelID: english, Name: "algorithm", Slug: "algorithm", IsEnabled: true}
	fixture := labelFixture{labels: map[pgtype.UUID]dbgen.DirectoryTagLabel{
		chinese: {ID: chinese, TagID: concept, Name: "算法", NormalizedName: "算法", IsEnabled: true},
		english: {ID: english, TagID: concept, Name: "algorithm", NormalizedName: "algorithm", IsEnabled: true},
	}}
	// When the Chinese name is selected despite the English default.
	result, err := canonicalTagLabel(context.Background(), fixture, TagSnapshot{LabelID: optionalUUIDString(chinese), Role: "TERTIARY"}, canonical, false)
	// Then the chosen name remains and identity/slug are shared.
	require.NoError(t, err)
	require.Equal(t, "算法", result.Name)
	require.Equal(t, optionalUUIDString(chinese), result.LabelID)
	require.Equal(t, optionalUUIDString(concept), result.ID)
	require.Equal(t, "algorithm", result.Slug)
}

func TestSelectedLabelRejectsForgedOwnership(t *testing.T) {
	t.Parallel()
	// Given a valid label owned by another concept.
	concept := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	label := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	other := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	fixture := labelFixture{labels: map[pgtype.UUID]dbgen.DirectoryTagLabel{label: {ID: label, TagID: other, IsEnabled: true}}}
	// When a client supplies that label with the first concept.
	_, err := canonicalTagLabel(context.Background(), fixture, TagSnapshot{LabelID: optionalUUIDString(label)}, dbgen.DirectoryTagDictionary{ID: concept, IsEnabled: true}, false)
	// Then selection is rejected even though the label exists.
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid_tag_label")
}

func TestDisabledSelectedLabelRemainsReadableButCannotBeNewlyChosen(t *testing.T) {
	t.Parallel()
	concept := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	label := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	fixture := labelFixture{labels: map[pgtype.UUID]dbgen.DirectoryTagLabel{label: {ID: label, TagID: concept, Name: "旧名称"}}}
	canonical := dbgen.DirectoryTagDictionary{ID: concept, DefaultLabelID: label, IsEnabled: true}
	tag := TagSnapshot{LabelID: optionalUUIDString(label)}
	t.Run("existing association", func(t *testing.T) {
		// Given an existing association. When it is loaded. Then its name survives.
		result, err := canonicalTagLabel(context.Background(), fixture, tag, canonical, true)
		require.NoError(t, err)
		require.Equal(t, "旧名称", result.Name)
	})
	t.Run("new choice", func(t *testing.T) {
		// Given a new association. When it selects the disabled name. Then it is rejected.
		_, err := canonicalTagLabel(context.Background(), fixture, tag, canonical, false)
		require.Error(t, err)
	})
}

func TestDistinctLabelNamesCannotDuplicateTertiaryConcept(t *testing.T) {
	t.Parallel()
	// Given different selected names sharing one concept.
	tags := []TagSnapshot{{ID: "primary", Role: "PRIMARY"}, {ID: "secondary", Role: "SECONDARY"}, {ID: "algorithm", LabelID: "中文", Role: "TERTIARY"}, {ID: "algorithm", LabelID: "english", Role: "TERTIARY"}}
	// When semantic assignment validation runs. Then the duplicate concept is rejected.
	require.Error(t, validateTagConcepts(tags))
}
