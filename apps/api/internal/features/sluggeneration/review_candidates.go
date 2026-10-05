package sluggeneration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"heyblog-api/internal/infrastructure/tokenhub"
)

func (service *Service) reviewCandidates(ctx context.Context, identity Identity, model string, inputs []tokenhub.BatchInput) (map[string]string, error) {
	result := make(map[string]string, len(inputs))
	missing := make([]tokenhub.BatchInput, 0, len(inputs))
	keys := map[string]string{}
	for _, input := range inputs {
		if slug := localSlug(input.Name); slug != "" {
			result[input.ID] = slug
			continue
		}
		key, err := cacheKey(model, tokenhub.Input{Name: input.Name, Description: input.Description})
		if err != nil {
			return nil, err
		}
		slug, err := service.store.Cached(ctx, key)
		if err != nil {
			return nil, unavailable()
		}
		if slug != "" {
			result[input.ID] = slug
			continue
		}
		missing = append(missing, input)
		keys[input.ID] = key
	}
	if len(missing) == 0 {
		return result, nil
	}
	if service.config.APIKey == "" {
		return nil, unavailable()
	}
	provider, ok := service.provider.(batchProvider)
	if !ok {
		return nil, unavailable()
	}
	encoded, err := json.Marshal(struct {
		Model  string
		Inputs []tokenhub.BatchInput
	}{model, missing})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(encoded)
	lease, err := service.guard.Acquire(ctx, Attempt{UserID: identity.UserID, Key: hex.EncodeToString(digest[:])})
	if err != nil {
		return nil, err
	}
	defer func() {
		releaseContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = service.guard.Release(releaseContext, lease)
	}()
	if err = service.guard.Charge(ctx, identity.UserID); err != nil {
		return nil, err
	}
	candidates, err := provider.GenerateBatch(ctx, model, missing)
	if err != nil {
		return nil, unavailable()
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if _, exists := keys[candidate.ID]; !exists || seen[candidate.ID] || !validCandidate(candidate.Slug) {
			return nil, unavailable()
		}
		seen[candidate.ID] = true
	}
	if len(seen) != len(missing) {
		return nil, unavailable()
	}
	for _, candidate := range candidates {
		result[candidate.ID] = candidate.Slug
		if err = service.store.Cache(ctx, keys[candidate.ID], candidate.Slug); err != nil {
			return nil, unavailable()
		}
	}
	return result, nil
}
