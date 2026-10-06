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

	"heyblog-api/internal/features/auth"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

var validSlug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type tagQueries interface {
	tagLabelQueries
	ListManagedTags(context.Context) ([]dbgen.DirectoryTagDictionary, error)
	GetTagByNormalizedName(context.Context, string) (dbgen.DirectoryTagDictionary, error)
	EnableCanonicalTag(context.Context, pgtype.UUID) error
	CreateTag(context.Context, dbgen.CreateTagParams) (dbgen.DirectoryTagDictionary, error)
}

type componentQueries interface {
	GetSoftwareComponentByID(context.Context, pgtype.UUID) (dbgen.DirectorySoftwareComponent, error)
	GetSoftwareComponentByNormalizedName(context.Context, string) (dbgen.DirectorySoftwareComponent, error)
	CreateSoftwareComponent(context.Context, dbgen.CreateSoftwareComponentParams) (dbgen.DirectorySoftwareComponent, error)
}

func resolveTaxonomy(ctx context.Context, queries *dbgen.Queries, reviewer auth.User, snapshot, current Snapshot) (Snapshot, error) {
	var err error
	snapshot, err = normalizeSnapshotTaxonomy(ctx, queries, snapshot)
	if err != nil {
		return Snapshot{}, err
	}
	for index, tag := range snapshot.Tags {
		if tag.ID != "" {
			id, parseErr := parseUUID(tag.ID)
			if parseErr != nil {
				return Snapshot{}, parseErr
			}
			canonical, readErr := queries.GetCanonicalTag(ctx, id)
			if readErr != nil {
				return Snapshot{}, readErr
			}
			if _, labelErr := canonicalTagLabel(ctx, queries, tag, canonical, snapshotHasTagLabel(current, tag)); labelErr != nil {
				return Snapshot{}, labelErr
			}
		}
		resolved, err := resolveTag(ctx, queries, reviewer, tag)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Tags[index] = resolved
	}
	if err := validateTagConcepts(snapshot.Tags); err != nil {
		return Snapshot{}, err
	}
	snapshot, err = normalizeSnapshotTaxonomy(ctx, queries, snapshot)
	if err != nil {
		return Snapshot{}, err
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
	if tag.ID == "" && (tag.Level != 3 || tag.Role != "TERTIARY") {
		return TagSnapshot{}, newServiceError("invalid_tag", http.StatusUnprocessableEntity, "new tags may only be proposed as tertiary tags")
	}
	if tag.ID != "" {
		id, err := parseUUID(tag.ID)
		if err != nil {
			return TagSnapshot{}, newServiceError("invalid_tag", http.StatusUnprocessableEntity, "a selected tag is invalid")
		}
		rows, err := queries.ListManagedTags(ctx)
		if err != nil {
			return TagSnapshot{}, fmt.Errorf("list enabled tags during review: %w", err)
		}
		for _, row := range rows {
			if row.ID == id {
				return canonicalTagLabel(ctx, queries, tag, row, true)
			}
		}
		return TagSnapshot{}, newServiceError("invalid_tag", http.StatusUnprocessableEntity, "a selected tag is no longer available")
	}
	name := strings.TrimSpace(tag.SuggestedName)
	normalized := strings.ToLower(name)
	if existing, err := queries.GetTagByNormalizedName(ctx, normalized); err == nil {
		if !existing.IsEnabled {
			if !canManageTaxonomy(reviewer) {
				return TagSnapshot{}, newServiceError("taxonomy_permission_required", http.StatusForbidden, "taxonomy management permission is required to reactivate tags")
			}
			if err := queries.EnableCanonicalTag(ctx, existing.ID); err != nil {
				return TagSnapshot{}, err
			}
			existing.IsEnabled = true
		}
		return canonicalTagLabel(ctx, queries, tag, existing, false)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return TagSnapshot{}, fmt.Errorf("find tag by normalized name: %w", err)
	}
	if !canManageTaxonomy(reviewer) {
		return TagSnapshot{}, newServiceError("taxonomy_permission_required", http.StatusForbidden, "taxonomy management permission is required to approve new tags")
	}
	if !validSlug.MatchString(tag.Slug) {
		return TagSnapshot{}, newServiceError("taxonomy_metadata_required", http.StatusUnprocessableEntity, "new tags require a valid slug")
	}
	if ownerQueries, ok := queries.(interface {
		TaxonomySlugOwner(context.Context, string) ([]pgtype.UUID, error)
	}); ok {
		owners, err := ownerQueries.TaxonomySlugOwner(ctx, tag.Slug)
		if err != nil {
			return TagSnapshot{}, err
		}
		if len(owners) > 0 {
			return TagSnapshot{}, newServiceError("slug_conflict", http.StatusConflict, "the slug is already reserved")
		}
	}
	created, err := queries.CreateTag(ctx, dbgen.CreateTagParams{Name: name, NormalizedName: normalized, Slug: tag.Slug, Description: strings.TrimSpace(tag.Description)})
	if err != nil {
		return TagSnapshot{}, fmt.Errorf("create reviewed tag: %w", err)
	}
	return canonicalTagLabel(ctx, queries, tag, created, false)
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
