package publicview

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"heyblog-api/internal/apperror"
	dbgen "heyblog-api/internal/database/gen"
)

func TestRandomSiteMapsSelectedCardIncludingWarnings(t *testing.T) {
	t.Parallel()

	row := testSite("A1b2C3d4E")
	service := New(queryStub{
		directoryCascades: []dbgen.ListEnabledSiteTagCascadesRow{{Level1Name: "技术", Level2Name: "写作"}},
		pickRandom: func(_ context.Context, query dbgen.PickRandomVisibleSiteParams) (dbgen.DirectorySite, error) {
			if query.Level1TagName != "技术" || query.Level2TagName != "写作" {
				t.Fatalf("random query = %#v", query)
			}
			return row, nil
		},
		batchTags: []dbgen.ListPublicSiteTagsBySiteIDsRow{
			{SiteID: row.ID, Role: "WARNING", Name: "访问提示", Slug: "access-notice"},
		},
	})

	view, err := service.RandomSite(context.Background(), RandomSiteQuery{Level1: "技术", Level2: "写作"})
	if err != nil || view.Site == nil || view.Site.ShortID != row.ShortID || len(view.Site.Warnings) != 1 {
		t.Fatalf("RandomSite() = (%#v, %v)", view, err)
	}
}

func TestRandomSiteRejectsInvalidHierarchyBeforeSelection(t *testing.T) {
	t.Parallel()
	cases := []struct {
		query RandomSiteQuery
		code  string
	}{
		{RandomSiteQuery{Level2: "写作"}, "random_missing_level1"},
		{RandomSiteQuery{Level1: "未启用"}, "random_unknown_level1"},
		{RandomSiteQuery{Level1: "技术", Level2: "不存在"}, "random_unknown_level2"},
		{RandomSiteQuery{Level1: "技术", Level2: "旅行"}, "random_classification_mismatch"},
	}
	for _, testCase := range cases {
		service := New(queryStub{
			directoryCascades: []dbgen.ListEnabledSiteTagCascadesRow{
				{Level1Name: "技术", Level2Name: "写作"},
				{Level1Name: "生活", Level2Name: "旅行"},
			},
			pickRandom: func(context.Context, dbgen.PickRandomVisibleSiteParams) (dbgen.DirectorySite, error) {
				t.Fatal("invalid hierarchy reached selection")
				return dbgen.DirectorySite{}, nil
			},
		})
		_, err := service.RandomSite(context.Background(), testCase.query)
		var applicationError *apperror.Error
		if !errors.As(err, &applicationError) || applicationError.Code() != testCase.code {
			t.Fatalf("RandomSite(%#v) error = %v, want %s", testCase.query, err, testCase.code)
		}
	}
}

func TestRandomSiteLevel1DoesNotNarrowToOneChild(t *testing.T) {
	t.Parallel()
	service := New(queryStub{
		directoryCascades: []dbgen.ListEnabledSiteTagCascadesRow{
			{Level1Name: "技术", Level2Name: "写作"},
			{Level1Name: "技术", Level2Name: "开发"},
		},
		pickRandom: func(_ context.Context, query dbgen.PickRandomVisibleSiteParams) (dbgen.DirectorySite, error) {
			if query.Level1TagName != "技术" || query.Level2TagName != "" {
				t.Fatalf("level1-only query = %#v", query)
			}
			return dbgen.DirectorySite{}, pgx.ErrNoRows
		},
	})
	if _, err := service.RandomSite(context.Background(), RandomSiteQuery{Level1: "技术"}); err != nil {
		t.Fatal(err)
	}
}

func TestRandomSiteReturnsNullForNoMatch(t *testing.T) {
	t.Parallel()

	service := New(queryStub{
		pickRandom: func(context.Context, dbgen.PickRandomVisibleSiteParams) (dbgen.DirectorySite, error) {
			return dbgen.DirectorySite{}, pgx.ErrNoRows
		},
	})
	view, err := service.RandomSite(context.Background(), RandomSiteQuery{})
	if err != nil || view.Site != nil {
		t.Fatalf("RandomSite() = (%#v, %v), want null site", view, err)
	}
}

func TestRandomSitePropagatesQueryFailure(t *testing.T) {
	t.Parallel()

	service := New(queryStub{
		pickRandom: func(context.Context, dbgen.PickRandomVisibleSiteParams) (dbgen.DirectorySite, error) {
			return dbgen.DirectorySite{}, errors.New("database unavailable")
		},
	})
	if _, err := service.RandomSite(context.Background(), RandomSiteQuery{}); err == nil {
		t.Fatal("RandomSite() error = nil, want query failure")
	}
}
