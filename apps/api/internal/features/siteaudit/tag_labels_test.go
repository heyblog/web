package siteaudit

import (
	"encoding/json"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestDisabledTagRequiresExistingAssignment(t *testing.T) {
	row := dbgen.DirectoryTag{ID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}, Name: "旧名称"}
	_, err := canonicalTag(TagSnapshot{Role: "TERTIARY"}, row, false)
	require.Error(t, err)
	tag, err := canonicalTag(TagSnapshot{Role: "TERTIARY"}, row, true)
	require.NoError(t, err)
	require.Equal(t, "旧名称", tag.Name)
}

func TestDifferentNamesRemainIndependentTags(t *testing.T) {
	tags := []TagSnapshot{{ID: "javascript", Name: "JavaScript", Role: "TERTIARY"}, {ID: "js", Name: "JS", Role: "TERTIARY"}}
	require.NoError(t, validateTagConcepts(tags))
	tags = append(tags, tags[0])
	require.Error(t, validateTagConcepts(tags))
}

func TestHistoricalTagLabelIsInternalOnly(t *testing.T) {
	var tag TagSnapshot
	require.NoError(t, json.Unmarshal([]byte(`{"id":"original","label_id":"selected","name":"算法","slug":"algorithm","role":"TERTIARY"}`), &tag))
	require.Equal(t, "selected", tag.historicalLabelID)
	require.Equal(t, "original", tag.ID)
	require.Equal(t, "算法", tag.Name)
	encoded, err := json.Marshal(tag)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "label_id")
	require.NotContains(t, string(encoded), "slug")
}
