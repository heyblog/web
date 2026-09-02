package siteaudit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/auth"
	dbgen "heyblog-api/internal/database/gen"
)

var validSlug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type tagQueries interface {
	ListEnabledTags(context.Context) ([]dbgen.DirectoryTag, error)
	GetTagByNormalizedName(context.Context, string) (dbgen.DirectoryTag, error)
	CreateTag(context.Context, dbgen.CreateTagParams) (dbgen.DirectoryTag, error)
}

type componentQueries interface {
	GetSoftwareComponentByID(context.Context, pgtype.UUID) (dbgen.DirectorySoftwareComponent, error)
	GetSoftwareComponentByNormalizedName(context.Context, string) (dbgen.DirectorySoftwareComponent, error)
	CreateSoftwareComponent(context.Context, dbgen.CreateSoftwareComponentParams) (dbgen.DirectorySoftwareComponent, error)
}

func resolveTaxonomy(ctx context.Context, queries *dbgen.Queries, reviewer auth.User, snapshot Snapshot) (Snapshot, error) {
	for index, tag := range snapshot.Tags {
		resolved, err := resolveTag(ctx, queries, reviewer, tag)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Tags[index] = resolved
	}
	for index, component := range snapshot.Components {
		resolved, err := resolveComponent(ctx, queries, reviewer, component)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Components[index] = resolved
	}
	for index, dependency := range snapshot.ProgramDependencies {
		resolved, err := resolveComponent(ctx, queries, reviewer, dependency)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.ProgramDependencies[index] = resolved
	}
	if err := validateResolvedArchitecture(snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func validateResolvedArchitecture(snapshot Snapshot) error {
	programID := ""
	for _, component := range snapshot.Components {
		if component.Role == "SITE_PROGRAM" {
			programID = component.ID
			break
		}
	}
	seen := make(map[string]struct{}, len(snapshot.ProgramDependencies))
	for _, dependency := range snapshot.ProgramDependencies {
		if dependency.ID == programID {
			return newServiceError("invalid_program_dependency", http.StatusUnprocessableEntity, "a program cannot depend on itself")
		}
		key := dependency.Role + ":" + dependency.ID
		if _, exists := seen[key]; exists {
			return newServiceError("invalid_program_dependency", http.StatusUnprocessableEntity, "program dependencies must be unique")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func resolveTag(ctx context.Context, queries tagQueries, reviewer auth.User, tag TagSnapshot) (TagSnapshot, error) {
	if tag.ID != "" {
		id, err := parseUUID(tag.ID)
		if err != nil {
			return TagSnapshot{}, newServiceError("invalid_tag", http.StatusUnprocessableEntity, "a selected tag is invalid")
		}
		rows, err := queries.ListEnabledTags(ctx)
		if err != nil {
			return TagSnapshot{}, fmt.Errorf("list enabled tags during review: %w", err)
		}
		for _, row := range rows {
			if row.ID == id {
				tag.Name = row.Name
				tag.Slug = row.Slug
				tag.Description = row.Description
				tag.SuggestedName = ""
				return tag, nil
			}
		}
		return TagSnapshot{}, newServiceError("invalid_tag", http.StatusUnprocessableEntity, "a selected tag is no longer available")
	}
	name := strings.TrimSpace(tag.SuggestedName)
	normalized := strings.ToLower(name)
	if existing, err := queries.GetTagByNormalizedName(ctx, normalized); err == nil {
		if !existing.IsEnabled {
			return TagSnapshot{}, newServiceError("invalid_tag", http.StatusUnprocessableEntity, "the matching tag is no longer available")
		}
		tag.ID, _ = uuidString(existing.ID)
		tag.Name = existing.Name
		tag.Slug = existing.Slug
		tag.Description = existing.Description
		tag.SuggestedName = ""
		return tag, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return TagSnapshot{}, fmt.Errorf("find tag by normalized name: %w", err)
	}
	if !canManageTaxonomy(reviewer) {
		return TagSnapshot{}, newServiceError("taxonomy_permission_required", http.StatusForbidden, "taxonomy management permission is required to approve new tags")
	}
	if !validSlug.MatchString(tag.Slug) || strings.TrimSpace(tag.Description) == "" {
		return TagSnapshot{}, newServiceError("taxonomy_metadata_required", http.StatusUnprocessableEntity, "new tags require a valid slug and description")
	}
	created, err := queries.CreateTag(ctx, dbgen.CreateTagParams{Name: name, NormalizedName: normalized, Slug: tag.Slug, Description: strings.TrimSpace(tag.Description)})
	if err != nil {
		return TagSnapshot{}, fmt.Errorf("create reviewed tag: %w", err)
	}
	tag.ID, _ = uuidString(created.ID)
	tag.Name = created.Name
	tag.SuggestedName = ""
	return tag, nil
}

func resolveComponent(ctx context.Context, queries componentQueries, reviewer auth.User, component ComponentSnapshot) (ComponentSnapshot, error) {
	if component.ID != "" {
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
	name := strings.TrimSpace(component.SuggestedName)
	normalized := strings.ToLower(name)
	if existing, err := queries.GetSoftwareComponentByNormalizedName(ctx, normalized); err == nil {
		if !existing.IsEnabled {
			return ComponentSnapshot{}, newServiceError("invalid_component", http.StatusUnprocessableEntity, "the matching software component is no longer available")
		}
		return canonicalComponentSnapshot(component, existing), nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ComponentSnapshot{}, fmt.Errorf("find software component by normalized name: %w", err)
	}
	if !canManageTaxonomy(reviewer) {
		return ComponentSnapshot{}, newServiceError("taxonomy_permission_required", http.StatusForbidden, "taxonomy management permission is required to approve new software components")
	}
	if component.HomepageURL == "" && component.RepositoryURL == "" || component.IsOpenSource == nil {
		return ComponentSnapshot{}, newServiceError("taxonomy_metadata_required", http.StatusUnprocessableEntity, "new software components require a homepage or repository URL")
	}
	created, err := queries.CreateSoftwareComponent(ctx, dbgen.CreateSoftwareComponentParams{Name: name, NormalizedName: normalized, Description: "", HomepageUrl: stringPointer(component.HomepageURL), RepositoryUrl: stringPointer(component.RepositoryURL), IsOpenSource: *component.IsOpenSource})
	if err != nil {
		return ComponentSnapshot{}, fmt.Errorf("create reviewed software component: %w", err)
	}
	return canonicalComponentSnapshot(component, created), nil
}

func canonicalComponentSnapshot(component ComponentSnapshot, row dbgen.DirectorySoftwareComponent) ComponentSnapshot {
	component.ID, _ = uuidString(row.ID)
	component.Name = row.Name
	component.SuggestedName = ""
	component.HomepageURL = stringValue(row.HomepageUrl)
	component.RepositoryURL = stringValue(row.RepositoryUrl)
	component.IsOpenSource = boolPointer(row.IsOpenSource)
	return component
}
