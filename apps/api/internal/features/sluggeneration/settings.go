package sluggeneration

import (
	"context"
	"slices"
	"strconv"
	"time"

	"heyblog-api/internal/platform/apperror"
)

var verifiedModels = []Model{
	{ID: "deepseek/deepseek-flash", Name: "DeepSeek-V4.1-Flash", Status: "unavailable"},
	{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash", Status: "unavailable"},
}

func (service *Service) Settings(ctx context.Context) (Settings, error) {
	settings, err := service.store.Settings(ctx)
	if err != nil {
		return Settings{}, unavailable()
	}
	if settings.Revision == "0" {
		settings.ModelID = service.config.DefaultModel
	}
	settings.Configured = service.config.APIKey != ""
	return settings, nil
}

func (service *Service) SaveSettings(ctx context.Context, input SaveSettingsInput) (Settings, error) {
	if revision, err := strconv.ParseInt(input.ExpectedRevision, 10, 64); err != nil || revision < 0 {
		return Settings{}, apperror.New(apperror.KindValidation, "invalid_revision", "a valid settings revision is required")
	}
	models, err := service.Models(ctx)
	if err != nil {
		return Settings{}, err
	}
	if !slices.ContainsFunc(models.Models, func(model Model) bool { return model.ID == input.ModelID && model.Status == "available" }) {
		return Settings{}, apperror.New(apperror.KindValidation, "model_unavailable", "select an available verified model")
	}
	settings, err := service.store.SaveSettings(ctx, input)
	if err != nil {
		return Settings{}, err
	}
	settings.Configured = service.config.APIKey != ""
	return settings, nil
}

func (service *Service) Models(ctx context.Context) (Models, error) {
	configured := service.config.APIKey != ""
	if !configured {
		return Models{Models: slices.Clone(verifiedModels), Configured: false}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, service.config.Timeout)
	defer cancel()
	service.modelsLock.Lock()
	defer service.modelsLock.Unlock()
	if ctx.Err() != nil {
		return Models{}, unavailable()
	}
	if time.Now().Before(service.modelsExpires) {
		return Models{Models: slices.Clone(service.modelsCache), Configured: true}, nil
	}
	online, err := service.provider.Models(ctx)
	if err != nil {
		return Models{}, unavailable()
	}
	models := slices.Clone(verifiedModels)
	// Availability means the verified ID is listed for this provider account,
	// not a real-time inference health check.
	for index := range models {
		if slices.Contains(online, models[index].ID) {
			models[index].Status = "available"
		}
	}
	service.modelsCache = models
	service.modelsExpires = time.Now().Add(service.config.ModelsCacheTTL)
	return Models{Models: slices.Clone(models), Configured: true}, nil
}
