package httpapi

import (
	"errors"
	"net/url"
	"testing"
	"time"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/application/publicview"
)

func TestParseDirectoryQueryAcceptsSupportedFilters(t *testing.T) {
	t.Parallel()

	values := url.Values{
		"page":       {"3"},
		"q":          {"  Astro  "},
		"primary":    {"technology", "life", "technology"},
		"secondary":  {"design", "writing"},
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
	if len(query.PrimaryTags) != 2 || len(query.SecondaryTags) != 2 ||
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
	}
	for _, values := range tests {
		_, err := parseDirectoryQuery(values, time.Now())
		var applicationError *apperror.Error
		if !errors.As(err, &applicationError) || applicationError.Kind() != apperror.KindBadRequest {
			t.Fatalf("parseDirectoryQuery(%v) error = %v, want bad request", values, err)
		}
	}
}
