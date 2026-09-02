package siteaudit

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"heyblog-api/internal/domain/site"
)

var ErrInvalidSubmission = errors.New("invalid site submission")

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
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		kind := strings.ToUpper(strings.TrimSpace(input.Kind))
		if kind != "SITEMAP" && kind != "LINK_PAGE" {
			return nil, fmt.Errorf("%w: unsupported resource kind", ErrInvalidSubmission)
		}
		if _, exists := seen[kind]; exists {
			return nil, fmt.Errorf("%w: resource kinds must be unique", ErrInvalidSubmission)
		}
		if _, err := site.NormalizeLocation(input.URL, address, false); err != nil {
			return nil, fmt.Errorf("%w: resource address: %w", ErrInvalidSubmission, err)
		}
		seen[kind] = struct{}{}
		resources = append(resources, ResourceSnapshot{Kind: kind, URL: strings.TrimSpace(input.URL)})
	}
	return resources, nil
}

func normalizeTags(inputs []TagInput) ([]TagSnapshot, error) {
	if len(inputs) == 0 || len(inputs) > 12 {
		return nil, fmt.Errorf("%w: between one and twelve existing tags are required", ErrInvalidSubmission)
	}
	tags := make([]TagSnapshot, 0, len(inputs))
	primaryCount := 0
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		role := strings.ToUpper(strings.TrimSpace(input.Role))
		if role != "PRIMARY" && role != "SECONDARY" {
			return nil, fmt.Errorf("%w: public submissions may use only topic tag roles", ErrInvalidSubmission)
		}
		if role == "PRIMARY" {
			primaryCount++
		}
		id := strings.TrimSpace(input.ID)
		name := strings.TrimSpace(input.SuggestedName)
		if id == "" || name != "" {
			return nil, fmt.Errorf("%w: public submissions may select only existing tags", ErrInvalidSubmission)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("%w: selected tags must be unique", ErrInvalidSubmission)
		}
		seen[id] = struct{}{}
		tags = append(tags, TagSnapshot{ID: id, SuggestedName: name, Slug: strings.TrimSpace(input.Slug), Description: strings.TrimSpace(input.Description), Role: role})
	}
	if primaryCount != 1 {
		return nil, fmt.Errorf("%w: exactly one primary tag is required", ErrInvalidSubmission)
	}
	return tags, nil
}
