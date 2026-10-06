package sluggeneration

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/infrastructure/tokenhub"
	"heyblog-api/internal/platform/config"
)

type Authenticator interface {
	Current(context.Context, *http.Request) (auth.User, error)
}
type Provider interface {
	Generate(context.Context, string, tokenhub.Input) (string, error)
	Models(context.Context) ([]string, error)
}
type Store interface {
	Settings(context.Context) (Settings, error)
	SaveSettings(context.Context, SaveSettingsInput) (Settings, error)
	Cached(context.Context, string) (string, error)
	Cache(context.Context, string, string) error
	Occupied(context.Context, string, string) (bool, error)
}
type Guard interface {
	Allow(context.Context, Identity) error
	Acquire(context.Context, Attempt) (string, error)
	Charge(context.Context, string) error
	Release(context.Context, string) error
}
type Identity struct {
	UserID      string
	IP          string
	IPHash      string
	SystemAdmin bool
}
type Attempt struct {
	UserID string
	Key    string
}
type Dependencies struct {
	Pool   *pgxpool.Pool
	Redis  *redis.Client
	Auth   Authenticator
	Config config.AIConfig
}
type SlugGenerationInput struct {
	Name        string `json:"name" minLength:"1" maxLength:"128"`
	Description string `json:"description,omitempty" maxLength:"2000"`
	ParentName  string `json:"parent_name,omitempty" maxLength:"128"`
	TagID       string `json:"tag_id,omitempty" maxLength:"36"`
}
type Result struct {
	Slug      string         `json:"slug"`
	Source    string         `json:"source" enum:"local,ai,cache,existing"`
	ModelID   string         `json:"model_id"`
	State     string         `json:"state" enum:"ready,needs_confirmation"`
	Conflicts []SlugConflict `json:"conflicts"`
}
type Settings struct {
	ModelID    string `json:"model_id"`
	Revision   string `json:"revision"`
	Configured bool   `json:"configured"`
}
type SaveSettingsInput struct {
	ModelID          string `json:"model_id" minLength:"1" maxLength:"128"`
	ExpectedRevision string `json:"expected_revision" pattern:"^(0|[1-9][0-9]*)$"`
}
type Model struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status" enum:"available,unavailable"`
}
type Models struct {
	Models     []Model `json:"models"`
	Configured bool    `json:"configured"`
}

type SlugConflict struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}
