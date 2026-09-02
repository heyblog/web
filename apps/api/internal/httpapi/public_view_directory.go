package httpapi

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/application/publicview"
)

const (
	directoryMaximumPage        = 100_000
	directoryMaximumQueryLength = 100
	directoryMaximumFilters     = 20
	directoryMaximumFilterValue = 100
	directoryMaximumSeedLength  = 96
)

var (
	directoryAllowedParameters = map[string]struct{}{
		"page": {}, "q": {}, "primary": {}, "secondary": {}, "warning": {},
		"technology": {}, "access": {}, "feed": {}, "status": {}, "sort": {},
		"order": {}, "seed": {},
	}
	directorySlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	directorySeedPattern = regexp.MustCompile(`^[A-Za-z0-9:_-]+$`)
)

func parseDirectoryQuery(values url.Values, now time.Time) (publicview.DirectoryQuery, error) {
	query := publicview.DefaultDirectoryQuery(now)
	for name := range values {
		if _, allowed := directoryAllowedParameters[name]; !allowed {
			return publicview.DirectoryQuery{}, invalidDirectoryQuery(name, "is not supported")
		}
	}

	page, exists, err := readDirectorySingle(values, "page")
	if err != nil {
		return publicview.DirectoryQuery{}, err
	}
	if exists {
		parsedPage, parseErr := strconv.ParseInt(page, 10, 32)
		if parseErr != nil || parsedPage < 1 || parsedPage > directoryMaximumPage {
			return publicview.DirectoryQuery{}, invalidDirectoryQuery(
				"page",
				"must be an integer between 1 and 100000",
			)
		}
		query.Page = int32(parsedPage)
	}

	queryText, exists, err := readDirectorySingle(values, "q")
	if err != nil {
		return publicview.DirectoryQuery{}, err
	}
	if exists {
		query.Query = strings.TrimSpace(queryText)
		if utf8.RuneCountInString(query.Query) > directoryMaximumQueryLength {
			return publicview.DirectoryQuery{}, invalidDirectoryQuery(
				"q",
				"must contain at most 100 characters",
			)
		}
	}

	query.PrimaryTags, err = readDirectorySlugs(values, "primary")
	if err != nil {
		return publicview.DirectoryQuery{}, err
	}
	query.SecondaryTags, err = readDirectorySlugs(values, "secondary")
	if err != nil {
		return publicview.DirectoryQuery{}, err
	}
	query.Warnings, err = readDirectorySlugs(values, "warning")
	if err != nil {
		return publicview.DirectoryQuery{}, err
	}
	query.Technologies, err = readDirectoryTechnologies(values)
	if err != nil {
		return publicview.DirectoryQuery{}, err
	}
	query.AccessScopes, err = readDirectoryAccessScopes(values)
	if err != nil {
		return publicview.DirectoryQuery{}, err
	}
	if err := readDirectoryEnums(values, &query); err != nil {
		return publicview.DirectoryQuery{}, err
	}
	return query, nil
}

func readDirectorySingle(values url.Values, name string) (string, bool, error) {
	items, exists := values[name]
	if !exists {
		return "", false, nil
	}
	if len(items) != 1 {
		return "", false, invalidDirectoryQuery(name, "must be provided once")
	}
	return items[0], true, nil
}

func readDirectorySlugs(values url.Values, name string) ([]string, error) {
	items, err := readDirectoryFilterValues(values, name)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if !directorySlugPattern.MatchString(item) {
			return nil, invalidDirectoryQuery(name, "contains an invalid value")
		}
	}
	return items, nil
}

func readDirectoryTechnologies(values url.Values) ([]string, error) {
	items, err := readDirectoryFilterValues(values, "technology")
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index] = strings.ToLower(items[index])
	}
	return items, nil
}

func readDirectoryFilterValues(values url.Values, name string) ([]string, error) {
	rawItems := values[name]
	if len(rawItems) > directoryMaximumFilters {
		return nil, invalidDirectoryQuery(name, "may contain at most 20 values")
	}
	items := make([]string, 0, len(rawItems))
	seen := make(map[string]struct{}, len(rawItems))
	for _, rawItem := range rawItems {
		item := strings.TrimSpace(rawItem)
		if item == "" || utf8.RuneCountInString(item) > directoryMaximumFilterValue {
			return nil, invalidDirectoryQuery(name, "contains an invalid value")
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		items = append(items, item)
	}
	return items, nil
}

func readDirectoryAccessScopes(values url.Values) ([]string, error) {
	items, err := readDirectoryFilterValues(values, "access")
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		switch item {
		case "ALL", "CN_ONLY", "GLOBAL_ONLY":
		default:
			return nil, invalidDirectoryQuery("access", "contains an invalid value")
		}
	}
	return items, nil
}

func readDirectoryEnums(values url.Values, query *publicview.DirectoryQuery) error {
	status, exists, err := readDirectorySingle(values, "status")
	if err != nil {
		return err
	}
	if exists {
		switch publicview.DirectoryStatus(status) {
		case publicview.DirectoryStatusNormal, publicview.DirectoryStatusAbnormal:
			query.Status = publicview.DirectoryStatus(status)
		default:
			return invalidDirectoryQuery("status", "must be normal or abnormal")
		}
	}

	feed, exists, err := readDirectorySingle(values, "feed")
	if err != nil {
		return err
	}
	if exists {
		switch publicview.DirectoryFeed(feed) {
		case publicview.DirectoryFeedAny, publicview.DirectoryFeedWith, publicview.DirectoryFeedWithout:
			query.Feed = publicview.DirectoryFeed(feed)
		default:
			return invalidDirectoryQuery("feed", "must be any, with, or without")
		}
	}

	sortMode, exists, err := readDirectorySingle(values, "sort")
	if err != nil {
		return err
	}
	if exists {
		switch publicview.DirectorySort(sortMode) {
		case publicview.DirectorySortRandom, publicview.DirectorySortJoined, publicview.DirectorySortUpdated:
			query.Sort = publicview.DirectorySort(sortMode)
		default:
			return invalidDirectoryQuery("sort", "must be random, joined, or updated")
		}
	}

	order, exists, err := readDirectorySingle(values, "order")
	if err != nil {
		return err
	}
	if exists {
		switch publicview.DirectoryOrder(order) {
		case publicview.DirectoryOrderAscending, publicview.DirectoryOrderDescending:
			query.Order = publicview.DirectoryOrder(order)
		default:
			return invalidDirectoryQuery("order", "must be asc or desc")
		}
	}

	seed, exists, err := readDirectorySingle(values, "seed")
	if err != nil {
		return err
	}
	if exists {
		seed = strings.TrimSpace(seed)
		if len(seed) == 0 || len(seed) > directoryMaximumSeedLength || !directorySeedPattern.MatchString(seed) {
			return invalidDirectoryQuery("seed", "contains an invalid value")
		}
		query.Seed = seed
	}
	return nil
}

func invalidDirectoryQuery(name, reason string) error {
	return apperror.New(
		apperror.KindBadRequest,
		apperror.CodeBadRequest,
		"directory query is invalid",
	).WithInvalidParams([]apperror.InvalidParam{{Name: name, Reason: reason}})
}
