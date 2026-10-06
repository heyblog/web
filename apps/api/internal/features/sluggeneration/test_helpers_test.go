package sluggeneration

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/infrastructure/tokenhub"
	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/config"
)

type memoryStore struct {
	settings   Settings
	cache      map[string]string
	occupied   map[string]string
	cacheError error
}

func (store *memoryStore) Settings(context.Context) (Settings, error) { return store.settings, nil }
func (store *memoryStore) SaveSettings(_ context.Context, input SaveSettingsInput) (Settings, error) {
	if input.ExpectedRevision != store.settings.Revision {
		return Settings{}, apperror.New(apperror.KindConflict, "settings_changed", "reload")
	}
	revision, _ := strconv.Atoi(store.settings.Revision)
	store.settings = Settings{ModelID: input.ModelID, Revision: strconv.Itoa(revision + 1)}
	return store.settings, nil
}
func (store *memoryStore) Cached(_ context.Context, key string) (string, error) {
	return store.cache[key], nil
}
func (store *memoryStore) Cache(_ context.Context, key, slug string) error {
	if store.cacheError != nil {
		return store.cacheError
	}
	store.cache[key] = slug
	return nil
}
func (store *memoryStore) Occupied(_ context.Context, slug, tagID string) (bool, error) {
	owner, found := store.occupied[slug]
	return found && owner != tagID, nil
}

type testProvider struct {
	calls       int
	batchUnique bool
	modelCalls  int
	err         error
	models      []string
	model       string
	input       tokenhub.Input
}

func (provider *testProvider) Generate(_ context.Context, model string, input tokenhub.Input) (string, error) {
	provider.calls++
	provider.model = model
	provider.input = input
	return "programming", provider.err
}
func (provider *testProvider) Models(context.Context) ([]string, error) {
	provider.modelCalls++
	return provider.models, provider.err
}

func (provider *testProvider) GenerateBatch(_ context.Context, model string, inputs []tokenhub.BatchInput) ([]tokenhub.BatchResult, error) {
	provider.calls++
	provider.model = model
	results := make([]tokenhub.BatchResult, 0, len(inputs))
	for _, input := range inputs {
		slug := "programming"
		if provider.batchUnique {
			slug += "-" + input.ID
		}
		results = append(results, tokenhub.BatchResult{ID: input.ID, Slug: slug})
	}
	return results, provider.err
}

type testGuard struct {
	allowed      int
	charged      int
	acquired     int
	released     int
	err          error
	afterAcquire func()
}

func (guard *testGuard) Allow(context.Context, Identity) error { guard.allowed++; return guard.err }
func (guard *testGuard) Acquire(context.Context, Attempt) (string, error) {
	guard.acquired++
	if guard.afterAcquire != nil {
		guard.afterAcquire()
	}
	return "lease", guard.err
}
func (guard *testGuard) Charge(context.Context, string) error  { guard.charged++; return guard.err }
func (guard *testGuard) Release(context.Context, string) error { guard.released++; return nil }

type testAuthentication struct {
	user auth.User
	err  error
}

func (authentication testAuthentication) Current(context.Context, *http.Request) (auth.User, error) {
	return authentication.user, authentication.err
}

func testService() (*Service, *memoryStore, *testProvider, *testGuard) {
	store := &memoryStore{settings: Settings{Revision: "0"}, cache: map[string]string{}, occupied: map[string]string{}}
	provider := &testProvider{models: []string{"deepseek/deepseek-flash", "unverified-model"}}
	guard := &testGuard{}
	return &Service{store: store, provider: provider, guard: guard, auth: testAuthentication{user: auth.User{ID: "user", Role: auth.RoleSysAdmin}}, config: config.AIConfig{APIKey: "test-key", DefaultModel: "deepseek/deepseek-flash", ModelsCacheTTL: 10 * time.Minute, Timeout: 15 * time.Second}}, store, provider, guard
}
