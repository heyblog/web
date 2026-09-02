package siteaudit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	dbgen "heyblog-api/internal/database/gen"
)

func (service *Service) prepareSubmissionTaxonomy(ctx context.Context, snapshot Snapshot) (Snapshot, error) {
	tags, err := service.repository.queries.ListEnabledTags(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list canonical submission tags: %w", err)
	}
	tagsByID := make(map[string]dbgen.DirectoryTag, len(tags))
	for _, tag := range tags {
		id, idErr := uuidString(tag.ID)
		if idErr != nil {
			return Snapshot{}, idErr
		}
		tagsByID[id] = tag
	}
	for index, tag := range snapshot.Tags {
		canonical, exists := tagsByID[tag.ID]
		if !exists {
			return Snapshot{}, newServiceError("invalid_tag", http.StatusUnprocessableEntity, "a selected tag is no longer available")
		}
		snapshot.Tags[index].Name = canonical.Name
		snapshot.Tags[index].SuggestedName = ""
		snapshot.Tags[index].Slug = canonical.Slug
		snapshot.Tags[index].Description = canonical.Description
	}

	programIndex := -1
	for index, component := range snapshot.Components {
		if component.ID != "" {
			canonical, canonicalErr := canonicalSubmissionComponent(ctx, service.repository.queries, component)
			if canonicalErr != nil {
				return Snapshot{}, canonicalErr
			}
			snapshot.Components[index] = canonical
		}
		if component.Role == "SITE_PROGRAM" {
			programIndex = index
		}
	}
	if programIndex < 0 {
		snapshot.ProgramDependencies = []ComponentSnapshot{}
		return snapshot, nil
	}

	program := snapshot.Components[programIndex]
	if program.ID != "" {
		programID, parseErr := parseUUID(program.ID)
		if parseErr != nil {
			return Snapshot{}, newServiceError("invalid_component", http.StatusUnprocessableEntity, "the selected site program is invalid")
		}
		dependencies, listErr := service.repository.queries.ListSoftwareComponentDependencies(ctx, programID)
		if listErr != nil {
			return Snapshot{}, fmt.Errorf("list canonical program dependencies: %w", listErr)
		}
		snapshot.ProgramDependencies = make([]ComponentSnapshot, 0, len(dependencies))
		for _, dependency := range dependencies {
			id, idErr := uuidString(dependency.DependencyComponentID)
			if idErr != nil {
				return Snapshot{}, idErr
			}
			snapshot.ProgramDependencies = append(snapshot.ProgramDependencies, ComponentSnapshot{ID: id, Name: dependency.Name, Role: dependency.Role, HomepageURL: stringValue(dependency.HomepageUrl), RepositoryURL: stringValue(dependency.RepositoryUrl), IsOpenSource: boolPointer(dependency.IsOpenSource)})
		}
		return snapshot, nil
	}

	normalizedProgramName := strings.ToLower(strings.TrimSpace(program.SuggestedName))
	if _, lookupErr := service.repository.queries.GetSoftwareComponentByNormalizedName(ctx, normalizedProgramName); lookupErr == nil {
		return Snapshot{}, newServiceError("program_already_exists", http.StatusConflict, "the custom program already exists in the catalog")
	} else if !errors.Is(lookupErr, pgx.ErrNoRows) {
		return Snapshot{}, fmt.Errorf("find custom program by normalized name: %w", lookupErr)
	}
	for index, dependency := range snapshot.ProgramDependencies {
		if dependency.ID != "" {
			canonical, canonicalErr := canonicalSubmissionComponent(ctx, service.repository.queries, dependency)
			if canonicalErr != nil {
				return Snapshot{}, canonicalErr
			}
			snapshot.ProgramDependencies[index] = canonical
			continue
		}
		normalizedDependencyName := strings.ToLower(strings.TrimSpace(dependency.SuggestedName))
		if normalizedDependencyName == normalizedProgramName {
			return Snapshot{}, newServiceError("invalid_program_dependency", http.StatusUnprocessableEntity, "a program cannot depend on itself")
		}
		existing, lookupErr := service.repository.queries.GetSoftwareComponentByNormalizedName(ctx, normalizedDependencyName)
		if lookupErr == nil {
			if !existing.IsEnabled {
				return Snapshot{}, newServiceError("invalid_program_dependency", http.StatusUnprocessableEntity, "a selected program dependency is no longer available")
			}
			snapshot.ProgramDependencies[index] = canonicalComponentSnapshot(dependency, existing)
			continue
		}
		if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return Snapshot{}, fmt.Errorf("find custom dependency by normalized name: %w", lookupErr)
		}
	}
	return snapshot, nil
}

func canonicalSubmissionComponent(ctx context.Context, queries *dbgen.Queries, component ComponentSnapshot) (ComponentSnapshot, error) {
	id, err := parseUUID(component.ID)
	if err != nil {
		return ComponentSnapshot{}, newServiceError("invalid_component", http.StatusUnprocessableEntity, "a selected software component is invalid")
	}
	row, err := queries.GetSoftwareComponentByID(ctx, id)
	if err != nil || !row.IsEnabled {
		return ComponentSnapshot{}, newServiceError("invalid_component", http.StatusUnprocessableEntity, "a selected software component is no longer available")
	}
	return canonicalComponentSnapshot(component, row), nil
}
