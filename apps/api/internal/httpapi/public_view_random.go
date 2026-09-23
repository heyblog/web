package httpapi

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/application/publicview"
)

func parseRandomSiteSearch(search string) (publicview.RandomSiteQuery, error) {
	values, err := url.ParseQuery(search)
	if err != nil {
		return publicview.RandomSiteQuery{}, invalidRandomQuery("random_invalid_parameter", "query", "must use valid query encoding")
	}
	return parseRandomSiteQuery(values)
}

func parseRandomSiteQuery(values url.Values) (publicview.RandomSiteQuery, error) {
	for _, name := range []string{"level1", "level2"} {
		if len(values[name]) > 1 {
			return publicview.RandomSiteQuery{}, invalidRandomQuery("random_duplicate_parameter", name, "must be provided once")
		}
	}
	for name := range values {
		if name != "level1" && name != "level2" {
			return publicview.RandomSiteQuery{}, invalidRandomQuery("random_unknown_parameter", name, "is not supported")
		}
	}
	level1, err := readRandomSiteName(values, "level1")
	if err != nil {
		return publicview.RandomSiteQuery{}, err
	}
	level2, err := readRandomSiteName(values, "level2")
	if err != nil {
		return publicview.RandomSiteQuery{}, err
	}
	if level2 != "" && level1 == "" {
		return publicview.RandomSiteQuery{}, invalidRandomQuery("random_missing_level1", "level2", "requires level1")
	}
	return publicview.RandomSiteQuery{Level1: level1, Level2: level2}, nil
}

func readRandomSiteName(values url.Values, name string) (string, error) {
	value, exists, err := readDirectorySingle(values, name)
	if err != nil || !exists {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > directoryMaximumFilterValue ||
		strings.ContainsFunc(value, unicode.IsControl) {
		return "", invalidRandomQuery("random_invalid_parameter", name, "must contain a classification name of 1 to 100 characters")
	}
	return value, nil
}

func invalidRandomQuery(code, name, reason string) error {
	return apperror.New(apperror.KindBadRequest, code, "random site query is invalid").
		WithInvalidParams([]apperror.InvalidParam{{Name: name, Reason: reason}})
}
