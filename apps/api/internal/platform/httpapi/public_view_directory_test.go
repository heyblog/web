package httpapi

import (
	"errors"
	"net/url"
	"testing"
	"time"

	"heyblog-api/internal/features/publicview"
	"heyblog-api/internal/platform/apperror"
)

func TestParseDirectoryQueryAcceptsSupportedFilters(t *testing.T) {
	t.Parallel()

	values := url.Values{
		"page":       {"3"},
		"q":          {"  Astro  "},
		"level1":     {"technology"},
		"level2":     {"writing"},
		"tertiary":   {"design", "astro"},
		"warning":    {"slow-access"},
		"technology": {"astro"},
		"access":     {"ALL", "CN_ONLY"},
		"feed":       {"with"},
		"status":     {"abnormal"},
		"sort":       {"updated"},
		"order":      {"asc"},
		"seed":       {"site-directory:shared"},
	}

	query, err := parseDirectoryQuery(values, time.Date(2026, time.September, 2, 0, 0, 0, 0, time.UTC))

	if err != nil {
		t.Fatalf("parseDirectoryQuery() error = %v", err)
	}
	if query.Page != 3 || query.Query != "Astro" || query.Feed != publicview.DirectoryFeedWith ||
		query.Status != publicview.DirectoryStatusAbnormal {
		t.Fatalf("query = %#v", query)
	}
	if query.Level1 != "technology" || query.Level2 != "writing" || len(query.TertiaryTags) != 2 ||
		query.Sort != publicview.DirectorySortUpdated || query.Order != publicview.DirectoryOrderAscending {
		t.Fatalf("normalized query = %#v", query)
	}
}

func TestParseDirectoryQueryRejectsUnknownAndInvalidParameters(t *testing.T) {
	t.Parallel()

	tests := []url.Values{
		{"unknown": {"value"}},
		{"topic": {"legacy"}},
		{"page": {"0"}},
		{"access": {"LOCAL"}},
		{"status": {"removed"}},
		{"seed": {"contains spaces"}},
		{"sort": {"random", "joined"}},
		{"level2": {"writing"}},
	}
	for _, values := range tests {
		_, err := parseDirectoryQuery(values, time.Now())
		var applicationError *apperror.Error
		if !errors.As(err, &applicationError) || applicationError.Kind() != apperror.KindBadRequest {
			t.Fatalf("parseDirectoryQuery(%v) error = %v, want bad request", values, err)
		}
	}
}

func TestDirectoryTagFiltersUseDisplayNamesAndDeduplicateCase(t *testing.T) {
	query, err := parseDirectoryQuery(url.Values{"level1": {"技术"}, "level2": {"Personal Writing"}, "tertiary": {"ExamPle", "example", "Example", "JS", "JavaScript"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if query.Level1 != "技术" || query.Level2 != "Personal Writing" || len(query.TertiaryTags) != 3 {
		t.Fatalf("name filters: %#v", query)
	}
	_, err = parseDirectoryQuery(url.Values{"tertiary_label_id": {"old-label"}}, time.Now())
	if err == nil {
		t.Fatal("retired label selector accepted")
	}
}
