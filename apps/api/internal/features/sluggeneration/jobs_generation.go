package sluggeneration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"heyblog-api/internal/infrastructure/tokenhub"
)

type batchProvider interface {
	GenerateBatch(context.Context, string, []tokenhub.BatchInput) ([]tokenhub.BatchResult, error)
}

func cacheKey(model string, input tokenhub.Input) (string, error) {
	encoded, err := json.Marshal(struct {
		Model   string
		Version string
		Input   tokenhub.Input
	}{model, tokenhub.PromptVersion, input})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
func localSlug(name string) string {
	if !simpleName.MatchString(name) {
		return ""
	}
	value := strings.ToLower(strings.Join(strings.Fields(name), "-"))
	return strings.Join(strings.FieldsFunc(value, func(r rune) bool { return r == '-' }), "-")
}
func (service *JobService) generateChunk(ctx context.Context, job *jobRecord) error {
	provider, ok := service.generation.provider.(batchProvider)
	if !ok {
		return unavailable()
	}
	inputs := make([]tokenhub.BatchInput, 0, service.config.Size)
	keys := map[string]string{}
	for i, item := range job.Items {
		if item.State != "pending" {
			continue
		}
		if candidate := localSlug(item.Name); candidate != "" {
			job.Items[i].Slug = candidate
			job.Items[i].Source = "local"
			job.Items[i].State = "ready"
			continue
		}
		key, err := cacheKey(job.ModelID, tokenhub.Input{Name: item.Name, Description: item.Description})
		if err != nil {
			return err
		}
		candidate, err := service.generation.store.Cached(ctx, key)
		if err != nil {
			return unavailable()
		}
		if candidate != "" {
			job.Items[i].Slug = candidate
			job.Items[i].Source = "cache"
			job.Items[i].State = "ready"
			continue
		}
		if len(inputs) < service.config.Size {
			inputs = append(inputs, tokenhub.BatchInput{ID: item.TagID, Name: item.Name, Description: item.Description})
			keys[item.TagID] = key
		}
	}
	if len(inputs) == 0 {
		return nil
	}
	if service.generation.config.APIKey == "" {
		return unavailable()
	}
	identity := Identity{UserID: job.OwnerID, IPHash: job.IPHash}
	if err := service.generation.guard.Allow(ctx, identity); err != nil {
		return err
	}
	digest, err := json.Marshal(struct {
		Model  string
		Inputs []tokenhub.BatchInput
	}{job.ModelID, inputs})
	if err != nil {
		return err
	}
	hash := sha256.Sum256(digest)
	lease, err := service.generation.guard.Acquire(ctx, Attempt{UserID: job.OwnerID, Key: hex.EncodeToString(hash[:])})
	if err != nil {
		return err
	}
	defer func() {
		releaseContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = service.generation.guard.Release(releaseContext, lease)
	}()
	if err = service.generation.guard.Charge(ctx, job.OwnerID); err != nil {
		return err
	}
	for i, item := range job.Items {
		if _, exists := keys[item.TagID]; exists {
			job.Items[i].State = "running"
		}
	}
	// Persist dispatch state before the paid call so process death never triggers a hidden retry.
	saved, err := service.save(ctx, job)
	if err != nil {
		return err
	}
	if !saved {
		job.Status = "paused"
		return nil
	}
	results, callErr := provider.GenerateBatch(ctx, job.ModelID, inputs)
	if callErr == nil {
		seen := map[string]bool{}
		for _, result := range results {
			if _, exists := keys[result.ID]; !exists || seen[result.ID] || !validCandidate(result.Slug) {
				callErr = tokenhub.ErrInvalidOutput
				break
			}
			seen[result.ID] = true
		}
		if len(seen) != len(inputs) {
			callErr = tokenhub.ErrInvalidOutput
		}
	}
	if callErr != nil {
		for i, item := range job.Items {
			if _, exists := keys[item.TagID]; exists {
				job.Items[i].State = "failed"
				job.Items[i].ErrorCode = "slug_generation_failed"
			}
		}
		return nil
	}
	for _, result := range results {
		for i, item := range job.Items {
			if item.TagID == result.ID {
				job.Items[i].Slug = result.Slug
				job.Items[i].State = "ready"
				job.Items[i].Source = "ai"
				job.Items[i].ErrorCode = ""
			}
		}
	}
	// Persist paid results before optional cache writes; a cache outage cannot discard them.
	saved, err = service.save(ctx, job)
	if err != nil {
		return err
	}
	if !saved {
		job.Status = "paused"
		return nil
	}
	for _, result := range results {
		if err = service.generation.store.Cache(ctx, keys[result.ID], result.Slug); err != nil {
			return unavailable()
		}
	}
	return nil
}
func (service *JobService) allocate(ctx context.Context, job *jobRecord) error {
	slices.SortFunc(job.Items, func(a, b jobItemRecord) int { return strings.Compare(a.TagID, b.TagID) })
	reserved := map[string]bool{}
	for i, item := range job.Items {
		if item.State == "applied" {
			reserved[item.Slug] = true
			continue
		}
		if item.State != "ready" {
			continue
		}
		base := item.Slug
		found := false
		for suffix := 1; suffix <= 10000; suffix++ {
			candidate := base
			if suffix > 1 {
				tail := fmt.Sprintf("-%d", suffix)
				candidate = strings.TrimRight(base[:min(len(base), 128-len(tail))], "-") + tail
			}
			if reserved[candidate] {
				continue
			}
			occupied, err := service.generation.store.Occupied(ctx, candidate, item.TagID)
			if err != nil {
				return unavailable()
			}
			if !occupied {
				job.Items[i].Slug = candidate
				reserved[candidate] = true
				found = true
				break
			}
		}
		if !found {
			job.Items[i].State = "failed"
			job.Items[i].ErrorCode = "slug_collision"
		}
	}
	return nil
}
