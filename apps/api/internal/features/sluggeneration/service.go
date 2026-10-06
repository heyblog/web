package sluggeneration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"

	"heyblog-api/internal/infrastructure/tokenhub"
	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/config"
)

var simpleName = regexp.MustCompile(`^[A-Za-z0-9]+(?:[ -]+[A-Za-z0-9]+)*$`)

type Service struct {
	store         Store
	provider      Provider
	guard         Guard
	auth          Authenticator
	config        config.AIConfig
	modelsLock    sync.Mutex
	modelsCache   []Model
	modelsExpires time.Time
	jobs          *JobService
}

func NewService(dependencies Dependencies) *Service {
	service := &Service{store: NewRepository(dependencies.Pool), provider: tokenhub.New(dependencies.Config),
		guard: NewRedisGuard(dependencies.Redis, dependencies.Config), auth: dependencies.Auth, config: dependencies.Config}
	service.jobs = NewJobService(service, NewJobRepository(dependencies.Pool), dependencies.Config.Batch)
	return service
}

func unavailable() error {
	return apperror.New(apperror.KindUnavailable, "slug_generation_unavailable", "slug generation is temporarily unavailable")
}

func (service *Service) Generate(ctx context.Context, identity Identity, input SlugGenerationInput) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, service.config.Timeout)
	defer cancel()
	if err := service.guard.Allow(ctx, identity); err != nil {
		return Result{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.ParentName = ""
	if input.Name == "" {
		return Result{}, apperror.New(apperror.KindValidation, "invalid_tag_name", "tag name is required")
	}
	if lookup, ok := service.store.(tagSlugLookup); ok {
		slug, err := lookup.ExistingSlug(ctx, strings.ToLower(input.Name))
		if err != nil {
			return Result{}, unavailable()
		}
		if slug != "" {
			if input.TagID != "" {
				return service.candidateResult(ctx, slug, input.TagID, "existing", "")
			}
			return Result{Slug: slug, Source: "existing", State: "ready", Conflicts: []SlugConflict{}}, nil
		}
	}
	if simpleName.MatchString(input.Name) {
		candidate := strings.ToLower(strings.Join(strings.Fields(input.Name), "-"))
		// Repeated separators have a single canonical representation.
		candidate = strings.Join(strings.FieldsFunc(candidate, func(r rune) bool { return r == '-' }), "-")
		return service.candidateResult(ctx, candidate, input.TagID, "local", "")
	}
	settings, err := service.Settings(ctx)
	if err != nil {
		return Result{}, err
	}
	data := tokenhub.Input{Name: input.Name, Description: input.Description, ParentName: input.ParentName}
	encoded, err := json.Marshal(struct {
		Model   string
		Version string
		Input   tokenhub.Input
	}{settings.ModelID, tokenhub.PromptVersion, data})
	if err != nil {
		return Result{}, unavailable()
	}
	digest := sha256.Sum256(encoded)
	key := hex.EncodeToString(digest[:])
	candidate, err := service.store.Cached(ctx, key)
	if err != nil {
		return Result{}, unavailable()
	}
	if candidate != "" {
		return service.candidateResult(ctx, candidate, input.TagID, "cache", settings.ModelID)
	}
	if !settings.Configured {
		return Result{}, unavailable()
	}
	lease, err := service.guard.Acquire(ctx, Attempt{UserID: identity.UserID, Key: key})
	if err != nil {
		return Result{}, err
	}
	// Release survives browser cancellation; the bounded lease also recovers process crashes.
	defer func() {
		releaseContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = service.guard.Release(releaseContext, lease)
	}()
	candidate, err = service.store.Cached(ctx, key)
	if err != nil {
		return Result{}, unavailable()
	}
	if candidate != "" {
		return service.candidateResult(ctx, candidate, input.TagID, "cache", settings.ModelID)
	}
	if err := service.guard.Charge(ctx, identity.UserID); err != nil {
		return Result{}, err
	}
	candidate, err = service.provider.Generate(ctx, settings.ModelID, data)
	if err != nil {
		if errors.Is(err, tokenhub.ErrInvalidOutput) {
			return Result{}, apperror.New(apperror.KindUnavailable, "invalid_slug_output", "slug provider returned an invalid candidate")
		}
		return Result{}, unavailable()
	}
	if err := service.store.Cache(ctx, key, candidate); err != nil {
		return Result{}, unavailable()
	}
	return service.candidateResult(ctx, candidate, input.TagID, "ai", settings.ModelID)
}
