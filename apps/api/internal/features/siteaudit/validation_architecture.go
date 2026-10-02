package siteaudit

import (
	"fmt"
	"net/url"
	"strings"
)

func normalizeArchitecture(programInputs, dependencyInputs []ComponentInput) ([]ComponentSnapshot, []ComponentSnapshot, error) {
	programs, err := normalizeProgram(programInputs)
	if err != nil {
		return nil, nil, err
	}
	dependencies, err := normalizeProgramDependencies(dependencyInputs)
	if err != nil {
		return nil, nil, err
	}
	if len(programs) == 0 && len(dependencies) > 0 {
		return nil, nil, fmt.Errorf("%w: program dependencies require a site program", ErrInvalidSubmission)
	}
	if len(programs) == 1 && programs[0].ID != "" && len(dependencies) > 0 {
		return nil, nil, fmt.Errorf("%w: existing program dependencies are managed by the catalog", ErrInvalidSubmission)
	}
	return programs, dependencies, nil
}

func normalizeProgram(inputs []ComponentInput) ([]ComponentSnapshot, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("%w: a site program is required", ErrInvalidSubmission)
	}
	if len(inputs) > 1 {
		return nil, fmt.Errorf("%w: at most one site program is allowed", ErrInvalidSubmission)
	}
	programs := make([]ComponentSnapshot, 0, len(inputs))
	for _, input := range inputs {
		role := strings.ToUpper(strings.TrimSpace(input.Role))
		if role != "SITE_PROGRAM" {
			return nil, fmt.Errorf("%w: public site components may contain only a site program", ErrInvalidSubmission)
		}
		id := strings.TrimSpace(input.ID)
		name := strings.TrimSpace(input.SuggestedName)
		if (id == "") == (name == "") {
			return nil, fmt.Errorf("%w: a program must reference an entry or contain a custom name", ErrInvalidSubmission)
		}
		if err := optionalHTTPURL(input.HomepageURL); err != nil {
			return nil, err
		}
		if err := optionalHTTPURL(input.RepositoryURL); err != nil {
			return nil, err
		}
		homepageURL := strings.TrimSpace(input.HomepageURL)
		repositoryURL := strings.TrimSpace(input.RepositoryURL)
		if id == "" {
			if len(name) > 128 {
				return nil, fmt.Errorf("%w: a custom program name must not exceed 128 characters", ErrInvalidSubmission)
			}
			if input.IsOpenSource == nil {
				return nil, fmt.Errorf("%w: a custom program requires an explicit open source status", ErrInvalidSubmission)
			}
			if homepageURL == "" && repositoryURL == "" {
				return nil, fmt.Errorf("%w: a custom program requires a homepage or repository", ErrInvalidSubmission)
			}
			if homepageURL == "" {
				homepageURL = repositoryURL
			}
		}
		programs = append(programs, ComponentSnapshot{ID: id, SuggestedName: name, Role: role, HomepageURL: homepageURL, RepositoryURL: repositoryURL, IsOpenSource: input.IsOpenSource})
	}
	return programs, nil
}

func normalizeProgramDependencies(inputs []ComponentInput) ([]ComponentSnapshot, error) {
	if len(inputs) > 12 {
		return nil, fmt.Errorf("%w: at most twelve program dependencies are allowed", ErrInvalidSubmission)
	}
	dependencies := make([]ComponentSnapshot, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		role := strings.ToUpper(strings.TrimSpace(input.Role))
		if role != "FRAMEWORK" && role != "LANGUAGE" {
			return nil, fmt.Errorf("%w: public program dependencies may be frameworks or languages", ErrInvalidSubmission)
		}
		id := strings.TrimSpace(input.ID)
		name := strings.TrimSpace(input.SuggestedName)
		if (id == "") == (name == "") {
			return nil, fmt.Errorf("%w: a dependency must reference an entry or contain a custom name", ErrInvalidSubmission)
		}
		if len(name) > 128 {
			return nil, fmt.Errorf("%w: a custom dependency name must not exceed 128 characters", ErrInvalidSubmission)
		}
		if err := optionalHTTPURL(input.HomepageURL); err != nil {
			return nil, err
		}
		if err := optionalHTTPURL(input.RepositoryURL); err != nil {
			return nil, err
		}
		key := role + ":" + id + strings.ToLower(name)
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("%w: program dependencies must be unique within each role", ErrInvalidSubmission)
		}
		seen[key] = struct{}{}
		dependencies = append(dependencies, ComponentSnapshot{
			ID: id, SuggestedName: name, Role: role,
			HomepageURL: strings.TrimSpace(input.HomepageURL), RepositoryURL: strings.TrimSpace(input.RepositoryURL),
			IsOpenSource: input.IsOpenSource,
		})
	}
	return dependencies, nil
}

func replaceProgramComponent(existing, programs []ComponentSnapshot) []ComponentSnapshot {
	components := make([]ComponentSnapshot, 0, len(existing)+len(programs))
	for _, component := range existing {
		if component.Role != "SITE_PROGRAM" {
			components = append(components, component)
		}
	}
	return append(components, programs...)
}

func optionalHTTPURL(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: component URLs must use HTTP or HTTPS", ErrInvalidSubmission)
	}
	return nil
}
