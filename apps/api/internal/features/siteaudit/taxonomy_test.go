package siteaudit

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/features/auth"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type existingTagQueries struct {
	tag dbgen.DirectoryTag
}

type existingComponentQueries struct {
	component dbgen.DirectorySoftwareComponent
}

func (queries existingTagQueries) ListManagedTags(context.Context) ([]dbgen.DirectoryTag, error) {
	return []dbgen.DirectoryTag{queries.tag}, nil
}

func (queries existingTagQueries) GetTagByNormalizedName(_ context.Context, name string) (dbgen.DirectoryTag, error) {
	if name != queries.tag.NormalizedName {
		return dbgen.DirectoryTag{}, pgx.ErrNoRows
	}
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
		TagSnapshot{SuggestedName: "  aStRo  ", Role: "TERTIARY", Level: 3},
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
		TagSnapshot{SuggestedName: "分类", Description: "说明", Role: "PRIMARY", Level: 1},
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

type componentCreationQueries struct {
	lookupName string
	created    dbgen.CreateSoftwareComponentParams
}

func (*componentCreationQueries) GetSoftwareComponentByID(context.Context, pgtype.UUID) (dbgen.DirectorySoftwareComponent, error) {
	return dbgen.DirectorySoftwareComponent{}, pgx.ErrNoRows
}

func (queries *componentCreationQueries) GetSoftwareComponentByNormalizedName(_ context.Context, name string) (dbgen.DirectorySoftwareComponent, error) {
	queries.lookupName = name
	return dbgen.DirectorySoftwareComponent{}, pgx.ErrNoRows
}

func (queries *componentCreationQueries) CreateSoftwareComponent(_ context.Context, params dbgen.CreateSoftwareComponentParams) (dbgen.DirectorySoftwareComponent, error) {
	queries.created = params
	return dbgen.DirectorySoftwareComponent{
		ID: pgtype.UUID{Bytes: [16]byte{3}, Valid: true}, Name: params.Name, NormalizedName: params.NormalizedName,
		HomepageUrl: params.HomepageUrl, RepositoryUrl: params.RepositoryUrl, IsOpenSource: params.IsOpenSource, IsEnabled: true,
	}, nil
}

func TestResolveComponentCreatesCanonicalComponentWithNormalizedName(t *testing.T) {
	t.Parallel()

	for _, role := range []string{"SITE_PROGRAM", "FRAMEWORK", "LANGUAGE"} {
		t.Run(role, func(t *testing.T) {
			t.Parallel()

			// Given a new component with mixed casing and surrounding whitespace.
			queries := &componentCreationQueries{}
			component := ComponentSnapshot{
				SuggestedName: "  CuStOm Component  ", Role: role,
				HomepageURL: "https://component.example.test", IsOpenSource: boolPointer(true),
			}

			// When a taxonomy administrator approves the suggestion.
			resolved, err := resolveComponent(context.Background(), queries, auth.User{Role: auth.RoleSysAdmin}, component)
			if err != nil {
				t.Fatalf("resolveComponent() error = %v", err)
			}

			// Then creation uses the canonical lookup key and returns the catalog identity.
			if queries.lookupName != "custom component" || queries.created.NormalizedName != "custom component" {
				t.Errorf("lookup name = %q, created normalized name = %q, want %q for both", queries.lookupName, queries.created.NormalizedName, "custom component")
			}
			if queries.created.Name != "CuStOm Component" || resolved.Name != "CuStOm Component" {
				t.Errorf("created name = %q, resolved name = %q, want trimmed display name", queries.created.Name, resolved.Name)
			}
			if resolved.ID != "03000000-0000-0000-0000-000000000000" || resolved.SuggestedName != "" || resolved.Role != role {
				t.Errorf("resolved component = %#v, want catalog identity with role %q", resolved, role)
			}
		})
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
