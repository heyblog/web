package sluggeneration

import (
	"context"
	"errors"
	"testing"

	"heyblog-api/internal/infrastructure/tokenhub"
	"heyblog-api/internal/platform/apperror"
)

func TestEnglishGenerationAndCollisionExclusion(t *testing.T) {
	service, store, provider, guard := testService()
	store.occupied["machine-learning"] = "other"
	got, err := service.Generate(context.Background(), Identity{UserID: "user"}, SlugGenerationInput{Name: " Machine  Learning "})
	if err != nil || got.Slug != "machine-learning" || got.State != "needs_confirmation" || got.Source != "local" || provider.calls != 0 || guard.charged != 0 {
		t.Fatalf("result=%+v error=%v paid=%d", got, err, guard.charged)
	}
	got, err = service.Generate(context.Background(), Identity{UserID: "user"}, SlugGenerationInput{Name: "Machine Learning", TagID: "other"})
	if err != nil || got.Slug != "machine-learning" {
		t.Fatalf("own slug result=%+v err=%v", got, err)
	}
}

func TestComplexNamesUseAIAndCacheIsContextAndModelSpecific(t *testing.T) {
	for _, name := range []string{"编程", "Go 开发", "アルゴリズム", "algorithme 算法", "C++", "C#", ".NET"} {
		t.Run(name, func(t *testing.T) {
			service, store, provider, guard := testService()
			input := SlugGenerationInput{Name: name, ParentName: "技术"}
			first, err := service.Generate(context.Background(), Identity{UserID: "user"}, input)
			if err != nil || first.Source != "ai" || provider.calls != 1 || guard.charged != 1 || guard.released != 1 {
				t.Fatalf("first=%+v error=%v", first, err)
			}
			store.occupied["programming"] = "other"
			cached, err := service.Generate(context.Background(), Identity{UserID: "user"}, input)
			if err != nil || cached.Source != "cache" || cached.Slug != "programming" || cached.State != "needs_confirmation" || provider.calls != 1 || guard.charged != 1 {
				t.Fatalf("cached=%+v error=%v", cached, err)
			}
			input.ParentName = "生活"
			if _, err = service.Generate(context.Background(), Identity{UserID: "user"}, input); err != nil {
				t.Fatal(err)
			}
			store.settings = Settings{Revision: "1", ModelID: "deepseek-v4-flash"}
			if _, err = service.Generate(context.Background(), Identity{UserID: "user"}, input); err != nil {
				t.Fatal(err)
			}
			if provider.calls != 2 || provider.model != "deepseek-v4-flash" || provider.input.ParentName != "" {
				t.Fatalf("calls=%d model=%s", provider.calls, provider.model)
			}
		})
	}
}

func TestLateCacheHitDoesNotConsumePaidBudget(t *testing.T) {
	service, store, provider, guard := testService()
	input := SlugGenerationInput{Name: "编程"}
	if _, err := service.Generate(context.Background(), Identity{}, input); err != nil {
		t.Fatal(err)
	}
	var key string
	for k := range store.cache {
		key = k
	}
	store.cache = map[string]string{}
	guard.afterAcquire = func() { store.cache[key] = "programming" }
	got, err := service.Generate(context.Background(), Identity{}, input)
	if err != nil || got.Source != "cache" || provider.calls != 1 || guard.charged != 1 || guard.released != 2 {
		t.Fatalf("late cache=%+v error=%v paid=%d", got, err, guard.charged)
	}
}

func TestProviderFailureIsSanitizedAndLeaseReleased(t *testing.T) {
	service, _, provider, guard := testService()
	provider.err = errors.New("secret provider body")
	_, err := service.Generate(context.Background(), Identity{}, SlugGenerationInput{Name: "编程"})
	var failure *apperror.Error
	if !errors.As(err, &failure) || failure.Code() != "slug_generation_unavailable" || guard.charged != 1 || guard.released != 1 {
		t.Fatalf("failure=%v paid=%d released=%d", err, guard.charged, guard.released)
	}
	provider.err = tokenhub.ErrInvalidOutput
	_, err = service.Generate(context.Background(), Identity{}, SlugGenerationInput{Name: "编程"})
	if !errors.As(err, &failure) || failure.Code() != "invalid_slug_output" {
		t.Fatalf("invalid output=%v", err)
	}
}

func TestMissingTokenAllowsLocalButDisablesProvider(t *testing.T) {
	service, _, provider, guard := testService()
	service.config.APIKey = ""
	if _, err := service.Generate(context.Background(), Identity{}, SlugGenerationInput{Name: "Go"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Generate(context.Background(), Identity{}, SlugGenerationInput{Name: "编程"}); err == nil {
		t.Fatal("AI unexpectedly enabled")
	}
	if provider.calls != 0 || guard.charged != 0 {
		t.Fatal("missing credential spent budget")
	}
}

func TestModelSettingsRestrictOnlineVerifiedModelsAndRevision(t *testing.T) {
	service, _, provider, _ := testService()
	initial, err := service.Settings(context.Background())
	if err != nil || initial.Revision != "0" || initial.ModelID != "deepseek/deepseek-flash" || !initial.Configured {
		t.Fatalf("initial=%+v error=%v", initial, err)
	}
	for _, model := range []string{"unverified-model", "deepseek-v4-flash"} {
		if _, err := service.SaveSettings(context.Background(), SaveSettingsInput{ModelID: model, ExpectedRevision: "0"}); err == nil {
			t.Fatalf("unavailable model %s accepted", model)
		}
	}
	result, err := service.SaveSettings(context.Background(), SaveSettingsInput{ModelID: "deepseek/deepseek-flash", ExpectedRevision: "0"})
	if err != nil || result.Revision != "1" || provider.modelCalls != 1 {
		t.Fatalf("save=%+v error=%v model requests=%d", result, err, provider.modelCalls)
	}
	if _, err := service.SaveSettings(context.Background(), SaveSettingsInput{ModelID: result.ModelID, ExpectedRevision: "0"}); err == nil {
		t.Fatal("stale revision accepted")
	}
}
