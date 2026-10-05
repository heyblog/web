-- +goose Up
CREATE TABLE content.system_ai_settings (
    singleton boolean DEFAULT true CHECK (singleton), -- Singleton global configuration record.
    model_id text NOT NULL CHECK (model_id <> '' AND char_length(model_id) <= 128), -- Selected verified provider model identifier.
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0), -- Optimistic concurrency version of the setting.
    updated_at timestamptz NOT NULL DEFAULT now(), -- Most recent successful settings change.
    CONSTRAINT system_ai_settings_primary_key PRIMARY KEY (singleton)
);
CREATE TABLE directory.slug_generation_cache (
    cache_key text NOT NULL CHECK (cache_key ~ '^[a-f0-9]{64}$'), -- Digest of model, prompt version and tag context.
    slug text NOT NULL CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND char_length(slug) <= 128), -- Validated base candidate; uniqueness is rechecked on each use.
    created_at timestamptz NOT NULL DEFAULT now(), -- Time the provider result was cached.
    CONSTRAINT slug_generation_cache_primary_key PRIMARY KEY (cache_key)
);
GRANT SELECT, INSERT, UPDATE ON content.system_ai_settings TO api_runtime;
GRANT SELECT, INSERT ON directory.slug_generation_cache TO api_runtime;
COMMENT ON TABLE content.system_ai_settings IS 'Global non-secret configuration for backend slug generation.';
COMMENT ON COLUMN content.system_ai_settings.singleton IS 'Singleton global configuration record.';
COMMENT ON COLUMN content.system_ai_settings.model_id IS 'Selected verified provider model identifier.';
COMMENT ON COLUMN content.system_ai_settings.revision IS 'Optimistic concurrency version of the setting.';
COMMENT ON COLUMN content.system_ai_settings.updated_at IS 'Most recent successful settings change.';
COMMENT ON TABLE directory.slug_generation_cache IS 'Reusable validated slug candidates keyed by model and input context.';
COMMENT ON COLUMN directory.slug_generation_cache.cache_key IS 'Digest of model, prompt version and tag context.';
COMMENT ON COLUMN directory.slug_generation_cache.slug IS 'Validated base candidate; uniqueness is rechecked on each use.';
COMMENT ON COLUMN directory.slug_generation_cache.created_at IS 'Time the provider result was cached.';

-- +goose Down
DROP TABLE directory.slug_generation_cache;
DROP TABLE content.system_ai_settings;
