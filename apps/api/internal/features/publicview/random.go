package publicview

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/platform/apperror"
)

type RandomSiteQuery struct {
	Level1 string
	Level2 string
}

type RandomSiteView struct {
	Site *SiteCardView `json:"site"`
}

func (service *Service) RandomSite(ctx context.Context, query RandomSiteQuery) (RandomSiteView, error) {
	if err := validateRandomClassification(ctx, service.random, query); err != nil {
		return RandomSiteView{}, err
	}
	row, err := service.random.PickRandomVisibleSite(ctx, dbgen.PickRandomVisibleSiteParams{
		Level1TagName: query.Level1,
		Level2TagName: query.Level2,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return RandomSiteView{}, nil
	}
	if err != nil {
		return RandomSiteView{}, internalError(err, "pick random visible site")
	}
	cards, err := loadSiteCards(ctx, service.random, []dbgen.DirectorySite{row})
	if err != nil {
		return RandomSiteView{}, err
	}
	return RandomSiteView{Site: &cards[0]}, nil
}

func validateRandomClassification(ctx context.Context, queries ClassificationQueries, query RandomSiteQuery) error {
	if query.Level1 == "" {
		if query.Level2 != "" {
			return randomClassificationError("random_missing_level1", "level2")
		}
		return nil
	}
	cascades, err := queries.ListEnabledSiteTagCascades(ctx)
	if err != nil {
		return internalError(err, "load random site classifications")
	}
	var foundLevel1, foundLevel2, matched bool
	for _, cascade := range cascades {
		foundLevel1 = foundLevel1 || cascade.Level1Name == query.Level1
		foundLevel2 = foundLevel2 || cascade.Level2Name == query.Level2
		matched = matched || cascade.Level1Name == query.Level1 && cascade.Level2Name == query.Level2
	}
	if !foundLevel1 {
		return randomClassificationError("random_unknown_level1", "level1")
	}
	if query.Level2 == "" {
		return nil
	}
	if !foundLevel2 {
		return randomClassificationError("random_unknown_level2", "level2")
	}
	if !matched {
		return randomClassificationError("random_classification_mismatch", "level2")
	}
	return nil
}

func randomClassificationError(code, name string) error {
	return apperror.New(apperror.KindBadRequest, code, "random site classification is invalid").
		WithInvalidParams([]apperror.InvalidParam{{Name: name, Reason: "must match an enabled classification hierarchy"}})
}
