package siteaudit

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/auth"
	dbgen "heyblog-api/internal/database/gen"
)

type existingTagQueries struct {
	tag dbgen.DirectoryTag
}

type existingComponentQueries struct {
	component dbgen.DirectorySoftwareComponent
}

func (queries existingTagQueries) ListEnabledTags(context.Context) ([]dbgen.DirectoryTag, error) {
	return []dbgen.DirectoryTag{queries.tag}, nil
}

func (queries existingTagQueries) GetTagByNormalizedName(context.Context, string) (dbgen.DirectoryTag, error) {
	return queries.tag, nil
}

func (existingTagQueries) CreateTag(context.Context, dbgen.CreateTagParams) (dbgen.DirectoryTag, error) {
	return dbgen.DirectoryTag{}, nil
}

func (queries existingComponentQueries) GetSoftwareComponentByID(context.Context, pgtype.UUID) (dbgen.DirectorySoftwareComponent, error) {
	return queries.component, nil
}

func (queries existingComponentQueries) GetSoftwareComponentByNormalizedName(context.Context, string) (dbgen.DirectorySoftwareComponent, error) {
	return queries.component, nil
}

func (existingComponentQueries) CreateSoftwareComponent(context.Context, dbgen.CreateSoftwareComponentParams) (dbgen.DirectorySoftwareComponent, error) {
	return dbgen.DirectorySoftwareComponent{}, nil
}

func TestResolveTagMapsSuggestionToExistingEntryWithoutTaxonomyPermission(t *testing.T) {
	t.Parallel()

	existingID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	reviewer := auth.User{Role: auth.RoleAdmin, Permissions: []auth.Permission{auth.PermissionSiteAuditReview}}

	resolved, err := resolveTag(
		context.Background(),
		existingTagQueries{tag: dbgen.DirectoryTag{ID: existingID, Name: "Astro", NormalizedName: "astro", IsEnabled: true}},
		reviewer,
		TagSnapshot{SuggestedName: "Astro", Role: "SECONDARY"},
	)

	if err != nil {
		t.Fatalf("resolveTag() error = %v", err)
	}
	wantID, err := uuidString(existingID)
	if err != nil {
		t.Fatalf("uuidString() error = %v", err)
	}
	if resolved.ID != wantID {
		t.Errorf("resolved.ID = %q, want %q", resolved.ID, wantID)
	}
	if resolved.SuggestedName != "" {
		t.Errorf("resolved.SuggestedName = %q, want empty", resolved.SuggestedName)
	}
}

func TestResolveComponentMapsSuggestionToExistingEntryWithoutTaxonomyPermission(t *testing.T) {
	t.Parallel()

	existingID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	reviewer := auth.User{Role: auth.RoleAdmin, Permissions: []auth.Permission{auth.PermissionSiteAuditReview}}

	resolved, err := resolveComponent(
		context.Background(),
		existingComponentQueries{component: dbgen.DirectorySoftwareComponent{ID: existingID, Name: "Astro", NormalizedName: "astro", HomepageUrl: stringPointer("https://astro.build"), RepositoryUrl: stringPointer("https://github.com/withastro/astro"), IsOpenSource: true, IsEnabled: true}},
		reviewer,
		ComponentSnapshot{SuggestedName: "Astro", Role: "SITE_PROGRAM"},
	)

	if err != nil {
		t.Fatalf("resolveComponent() error = %v", err)
	}
	wantID, err := uuidString(existingID)
	if err != nil {
		t.Fatalf("uuidString() error = %v", err)
	}
	if resolved.ID != wantID {
		t.Errorf("resolved.ID = %q, want %q", resolved.ID, wantID)
	}
	if resolved.SuggestedName != "" {
		t.Errorf("resolved.SuggestedName = %q, want empty", resolved.SuggestedName)
	}
	if resolved.Name != "Astro" || resolved.HomepageURL != "https://astro.build" || resolved.RepositoryURL != "https://github.com/withastro/astro" {
		t.Errorf("resolved canonical metadata = %#v", resolved)
	}
	if resolved.IsOpenSource == nil || !*resolved.IsOpenSource {
		t.Errorf("resolved.IsOpenSource = %#v, want true", resolved.IsOpenSource)
	}
}
