package sluggeneration

import (
	"context"
	"heyblog-api/internal/platform/apperror"
)

type slugConflictLookup interface {
	Conflicts(context.Context, string, string) ([]SlugConflict, error)
}

func (service *Service) candidateResult(ctx context.Context, candidate, tagID, source, model string) (Result, error) {
	result := Result{Slug: candidate, Source: source, ModelID: model, State: "ready", Conflicts: []SlugConflict{}}
	occupied, err := service.store.Occupied(ctx, candidate, tagID)
	if err != nil {
		return Result{}, unavailable()
	}
	if !occupied {
		return result, nil
	}
	result.State = "needs_confirmation"
	if lookup, ok := service.store.(slugConflictLookup); ok {
		result.Conflicts, err = lookup.Conflicts(ctx, candidate, tagID)
		if err != nil {
			return Result{}, unavailable()
		}
	}
	return result, nil
}

func needsConfirmation(name string, result Result) error {
	params := []apperror.InvalidParam{{Name: "tags." + name + ".slug", Reason: result.Slug}}
	for _, conflict := range result.Conflicts {
		params = append(params, apperror.InvalidParam{Name: "conflicts." + conflict.ID, Reason: conflict.Name + " (" + conflict.Slug + ")"})
	}
	return apperror.New(apperror.KindConflict, "slug_needs_confirmation", "confirm whether the matching tags are synonyms or supply a distinct slug").WithInvalidParams(params)
}
