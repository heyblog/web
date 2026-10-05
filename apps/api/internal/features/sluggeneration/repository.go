package sluggeneration

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/platform/apperror"
)

type Repository struct{ queries *dbgen.Queries }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{queries: dbgen.New(pool)} }

func (repository *Repository) Settings(ctx context.Context) (Settings, error) {
	row, err := repository.queries.GetSlugModelSetting(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{Revision: "0"}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	return Settings{ModelID: row.ModelID, Revision: strconv.FormatInt(row.Revision, 10)}, nil
}

func (repository *Repository) SaveSettings(ctx context.Context, input SaveSettingsInput) (Settings, error) {
	revision, err := strconv.ParseInt(input.ExpectedRevision, 10, 64)
	if err != nil {
		return Settings{}, apperror.New(apperror.KindValidation, "invalid_revision", "a valid settings revision is required")
	}
	result := Settings{}
	if revision == 0 {
		row, saveErr := repository.queries.SaveSlugModelSetting(ctx, dbgen.SaveSlugModelSettingParams{ModelID: input.ModelID, ExpectedRevision: revision})
		err = saveErr
		result.ModelID, result.Revision = row.ModelID, strconv.FormatInt(row.Revision, 10)
	} else {
		row, saveErr := repository.queries.UpdateSlugModelSetting(ctx, dbgen.UpdateSlugModelSettingParams{ModelID: input.ModelID, ExpectedRevision: revision})
		err = saveErr
		result.ModelID, result.Revision = row.ModelID, strconv.FormatInt(row.Revision, 10)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{}, apperror.New(apperror.KindConflict, "settings_changed", "system settings changed; reload before saving")
	}
	if err != nil {
		return Settings{}, unavailable()
	}
	return result, nil
}

func (repository *Repository) Cached(ctx context.Context, key string) (string, error) {
	slug, err := repository.queries.GetGeneratedSlug(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return slug, err
}
func (repository *Repository) Cache(ctx context.Context, key, slug string) error {
	return repository.queries.CacheGeneratedSlug(ctx, dbgen.CacheGeneratedSlugParams{CacheKey: key, Slug: slug})
}
func (repository *Repository) Occupied(ctx context.Context, slug, tagID string) (bool, error) {
	return repository.queries.SlugCandidateOccupied(ctx, dbgen.SlugCandidateOccupiedParams{Slug: slug, TagID: tagID})
}

func (repository *Repository) ExistingSlug(ctx context.Context, name string) (string, error) {
	slug, err := repository.queries.ExistingTagSlug(ctx, name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return slug, err
}
