package siteaudit

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"heyblog-api/internal/domain/site"
)

var (
	ErrInvalidSubmission      = errors.New("invalid site submission")
	ErrSiteURLPurposeConflict = errors.New("site URL purpose conflict")
)

func siteURLPurposeConflict(detail string) error {
	return fmt.Errorf("%w: %w: %s", ErrInvalidSubmission, ErrSiteURLPurposeConflict, detail)
}

func BuildProposedSnapshot(input SiteInput, base Snapshot) (Snapshot, error) {
	name := strings.TrimSpace(input.Name)
	summary := strings.TrimSpace(input.Summary)
	if name == "" || len(name) > 160 {
		return Snapshot{}, fmt.Errorf("%w: name is required and must not exceed 160 characters", ErrInvalidSubmission)
	}
	if len(summary) > 2_000 {
		return Snapshot{}, fmt.Errorf("%w: summary must not exceed 2000 characters", ErrInvalidSubmission)
	}
	address, err := site.NormalizeAddress(input.URL)
	if err != nil {
		return Snapshot{}, fmt.Errorf("%w: address: %w", ErrInvalidSubmission, err)
	}
	feeds, err := normalizeFeeds(input.Feeds, address)
	if err != nil {
		return Snapshot{}, err
	}
	resources, err := normalizeResources(input.Resources, address)
	if err != nil {
		return Snapshot{}, err
	}
	tags, err := normalizeTags(input.Tags)
	if err != nil {
		return Snapshot{}, err
	}
	components, dependencies, err := normalizeArchitecture(input.Components, input.ProgramDependencies)
	if err != nil {
		return Snapshot{}, err
	}

	base.Name = name
	base.Scheme = address.Scheme
	base.NormalizedHost = address.NormalizedHost
	base.BasePath = address.BasePath
	base.Summary = summary
	base.Feeds = feeds
	base.Resources = resources
	base.Tags = tags
	base.Components = replaceProgramComponent(base.Components, components)
	base.ProgramDependencies = dependencies
	return base, nil
}

func NormalizeSubmission(action Action, input SubmissionInput) (SubmissionInput, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	input.Contact.Name = strings.TrimSpace(input.Contact.Name)
	input.Contact.Email = strings.TrimSpace(input.Contact.Email)
	if (action != ActionCreate && input.Reason == "") || len(input.Reason) > 2_000 {
		return SubmissionInput{}, fmt.Errorf("%w: reason is required and must not exceed 2000 characters", ErrInvalidSubmission)
	}
	if len(input.Contact.Name) > 100 {
		return SubmissionInput{}, fmt.Errorf("%w: contact name must not exceed 100 characters", ErrInvalidSubmission)
	}
	if (input.Contact.Name == "") != (input.Contact.Email == "") {
		return SubmissionInput{}, fmt.Errorf("%w: contact name and email must be supplied together", ErrInvalidSubmission)
	}
	if input.Contact.Email != "" {
		address, err := mail.ParseAddress(input.Contact.Email)
		if err != nil || address.Address != input.Contact.Email || address.Name != "" {
			return SubmissionInput{}, fmt.Errorf("%w: contact email is invalid", ErrInvalidSubmission)
		}
	}
	if input.Contact.NotifyByEmail && input.Contact.Email == "" {
		return SubmissionInput{}, fmt.Errorf("%w: contact email is required for notifications", ErrInvalidSubmission)
	}
	return input, nil
}

func normalizeFeeds(inputs []FeedInput, address site.Address) ([]FeedSnapshot, error) {
	if len(inputs) > 8 {
		return nil, fmt.Errorf("%w: at most eight feeds are allowed", ErrInvalidSubmission)
	}
	feeds := make([]FeedSnapshot, 0, len(inputs))
	defaultCount := 0
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		name := strings.TrimSpace(input.Name)
		if name == "" || len(name) > 100 {
			return nil, fmt.Errorf("%w: every feed requires a short name", ErrInvalidSubmission)
		}
		location, err := site.NormalizeLocation(input.URL, address, false)
		if err != nil {
			return nil, fmt.Errorf("%w: feed address: %w", ErrInvalidSubmission, err)
		}
		if location.URLKey == address.BasePath {
			return nil, siteURLPurposeConflict("feed address must differ from the site homepage")
		}
		if _, exists := seen[location.URLKey]; exists {
			return nil, fmt.Errorf("%w: feed addresses must be unique", ErrInvalidSubmission)
		}
		seen[location.URLKey] = struct{}{}
		format := strings.ToUpper(strings.TrimSpace(input.Format))
		if format == "" {
			format = "UNKNOWN"
		}
		if format != "UNKNOWN" && format != "RSS" && format != "ATOM" && format != "JSON" {
			return nil, fmt.Errorf("%w: unsupported feed format", ErrInvalidSubmission)
		}
		if input.IsDefault {
			defaultCount++
		}
		feeds = append(feeds, FeedSnapshot{Name: name, URL: input.URL, Format: format, IsDefault: input.IsDefault})
	}
	if len(feeds) > 0 && defaultCount != 1 {
		return nil, fmt.Errorf("%w: enabled feeds require exactly one default", ErrInvalidSubmission)
	}
	return feeds, nil
}

func normalizeResources(inputs []ResourceInput, address site.Address) ([]ResourceSnapshot, error) {
	if len(inputs) > 2 {
		return nil, fmt.Errorf("%w: at most two site resources are allowed", ErrInvalidSubmission)
	}
	resources := make([]ResourceSnapshot, 0, len(inputs))
	seenKinds := make(map[string]struct{}, len(inputs))
	seenURLs := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		kind := strings.ToUpper(strings.TrimSpace(input.Kind))
		if kind != "SITEMAP" && kind != "LINK_PAGE" {
			return nil, fmt.Errorf("%w: unsupported resource kind", ErrInvalidSubmission)
		}
		if _, exists := seenKinds[kind]; exists {
			return nil, fmt.Errorf("%w: resource kinds must be unique", ErrInvalidSubmission)
		}
		location, err := site.NormalizeLocation(input.URL, address, false)
		if err != nil {
			return nil, fmt.Errorf("%w: resource address: %w", ErrInvalidSubmission, err)
		}
		if location.URLKey == address.BasePath {
			return nil, siteURLPurposeConflict("resource address must differ from the site homepage")
		}
		if _, exists := seenURLs[location.URLKey]; exists {
			return nil, siteURLPurposeConflict("resource addresses must be unique")
		}
		seenKinds[kind] = struct{}{}
		seenURLs[location.URLKey] = struct{}{}
		resources = append(resources, ResourceSnapshot{Kind: kind, URL: strings.TrimSpace(input.URL)})
	}
	return resources, nil
}

func validateSnapshotLocations(snapshot Snapshot) error {
	address := site.Address{Scheme: snapshot.Scheme, NormalizedHost: snapshot.NormalizedHost, BasePath: snapshot.BasePath}
	feeds := make([]FeedInput, 0, len(snapshot.Feeds))
	for _, feed := range snapshot.Feeds {
		feeds = append(feeds, FeedInput{Name: feed.Name, URL: feed.URL, Format: feed.Format, IsDefault: feed.IsDefault})
	}
	if _, err := normalizeFeeds(feeds, address); err != nil {
		return err
	}
	resources := make([]ResourceInput, 0, len(snapshot.Resources))
	for _, resource := range snapshot.Resources {
		resources = append(resources, ResourceInput(resource))
	}
	_, err := normalizeResources(resources, address)
	return err
}

func normalizeTags(inputs []TagInput) ([]TagSnapshot, error) {
	if len(inputs) < 2 || len(inputs) > 22 {
		return nil, fmt.Errorf("%w: one first-level, one second-level, and at most twenty tertiary tags are required", ErrInvalidSubmission)
	}
	tags := make([]TagSnapshot, 0, len(inputs))
	levelCounts := map[int]int{}
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		role := strings.ToUpper(strings.TrimSpace(input.Role))
		level := input.Level
		if level == 0 {
			switch role {
			case "PRIMARY":
				level = 1
			case "SECONDARY":
				level = 2
			case "TERTIARY":
				level = 3
			}
		}
		if level < 1 || level > 3 {
			return nil, fmt.Errorf("%w: every topic tag requires a valid level", ErrInvalidSubmission)
		}
		expectedRole := map[int]string{1: "PRIMARY", 2: "SECONDARY", 3: "TERTIARY"}[level]
		if role != expectedRole {
			return nil, fmt.Errorf("%w: tag roles must match their taxonomy levels", ErrInvalidSubmission)
		}
		levelCounts[level]++
		id := strings.TrimSpace(input.ID)
		name := strings.TrimSpace(input.SuggestedName)
		if level < 3 && (id == "" || name != "") {
			return nil, fmt.Errorf("%w: first- and second-level tags must select fixed options", ErrInvalidSubmission)
		}
		if level == 3 && (id == "") == (name == "") {
			return nil, fmt.Errorf("%w: each tertiary tag must select an existing tag or propose one name", ErrInvalidSubmission)
		}
		if level == 3 && name != "" {
			id = ""
			if len([]rune(name)) > 100 {
				return nil, fmt.Errorf("%w: tertiary tag proposals must not exceed 100 characters", ErrInvalidSubmission)
			}
		}
		key := id
		if key == "" {
			key = "name:" + strings.ToLower(name)
		}
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("%w: selected tags must be unique", ErrInvalidSubmission)
		}
		seen[key] = struct{}{}
		tags = append(tags, TagSnapshot{ID: id, SuggestedName: name, Slug: strings.TrimSpace(input.Slug), Description: strings.TrimSpace(input.Description), Role: expectedRole, Level: level, ParentID: strings.TrimSpace(input.ParentID)})
	}
	if levelCounts[1] != 1 || levelCounts[2] != 1 || levelCounts[3] > 20 {
		return nil, fmt.Errorf("%w: exactly one first-level and one second-level tag are required; tertiary tags are limited to twenty", ErrInvalidSubmission)
	}
	var level1ID string
	for _, tag := range tags {
		if tag.Level == 1 {
			level1ID = tag.ID
		}
	}
	for _, tag := range tags {
		if tag.Level == 2 && tag.ParentID != "" && tag.ParentID != level1ID {
			return nil, fmt.Errorf("%w: the second-level tag does not belong to the selected first-level tag", ErrInvalidSubmission)
		}
		if tag.Level == 3 && tag.ID != "" && (tag.ID == level1ID || anyTagIDAtLevel(tags, 2, tag.ID)) {
			return nil, fmt.Errorf("%w: tertiary tags must not repeat the selected classification", ErrInvalidSubmission)
		}
	}
	return tags, nil
}

func anyTagIDAtLevel(tags []TagSnapshot, level int, id string) bool {
	for _, tag := range tags {
		if tag.Level == level && tag.ID == id {
			return true
		}
	}
	return false
}
