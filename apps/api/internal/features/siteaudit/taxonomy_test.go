package siteaudit

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/features/auth"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type existingTagQueries struct {
	tag dbgen.DirectoryTagDictionary
}

type existingComponentQueries struct {
	component dbgen.DirectorySoftwareComponent
}

func (queries existingTagQueries) ListManagedTags(context.Context) ([]dbgen.DirectoryTagDictionary, error) {
	return []dbgen.DirectoryTagDictionary{queries.tag}, nil
}

func (queries existingTagQueries) GetTagByNormalizedName(context.Context, string) (dbgen.DirectoryTagDictionary, error) {
	return queries.tag, nil
}

func (existingTagQueries) CreateTag(context.Context, dbgen.CreateTagParams) (dbgen.DirectoryTagDictionary, error) {
	return dbgen.DirectoryTagDictionary{}, nil
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
		existingTagQueries{tag: dbgen.DirectoryTagDictionary{ID: existingID, Name: "Astro", NormalizedName: "astro", IsEnabled: true}},
		reviewer,
		TagSnapshot{SuggestedName: "Astro", Role: "TERTIARY", Level: 3},
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

func TestResolveTagRejectsCustomNonTertiaryTag(t *testing.T) {
	t.Parallel()

	_, err := resolveTag(
		context.Background(),
		existingTagQueries{},
		auth.User{Role: auth.RoleSysAdmin},
		TagSnapshot{SuggestedName: "分类", Slug: "classification", Description: "说明", Role: "PRIMARY", Level: 1},
	)

	if err == nil {
		t.Fatal("resolveTag() error = nil, want custom classification rejected")
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

func TestTertiaryAssignmentAcceptsIntrinsicClassificationLevel(t *testing.T) {
	tags, err := normalizeTags([]TagInput{
		{ID: "primary", Role: "PRIMARY", Level: 1},
		{ID: "secondary", Role: "SECONDARY", Level: 2, ParentID: "primary"},
		{ID: "another-primary", Role: "TERTIARY", Level: 1},
		{ID: "another-secondary", Role: "TERTIARY", Level: 2, ParentID: "another-primary"},
	})
	if err != nil || len(tags) != 4 || tags[2].Role != "TERTIARY" || tags[2].Level != 3 || tags[3].Level != 3 {
		t.Fatalf("assignment levels: %#v %v", tags, err)
	}
}

func (existingTagQueries) EnableCanonicalTag(context.Context, pgtype.UUID) error { return nil }

func (queries existingTagQueries) GetTagLabel(_ context.Context, id pgtype.UUID) (dbgen.DirectoryTagLabel, error) {
	return dbgen.DirectoryTagLabel{ID: id, TagID: queries.tag.ID, Name: queries.tag.Name, IsEnabled: queries.tag.IsEnabled}, nil
}
func (queries existingTagQueries) GetTagLabelByNormalizedName(context.Context, string) (dbgen.DirectoryTagLabel, error) {
	return dbgen.DirectoryTagLabel{ID: queries.tag.DefaultLabelID, TagID: queries.tag.ID, Name: queries.tag.Name, IsEnabled: queries.tag.IsEnabled}, nil
}
