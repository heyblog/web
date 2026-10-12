-- +goose Up

SET LOCAL check_function_bodies = false;

CREATE SCHEMA identity;

-- +goose StatementBegin

CREATE FUNCTION identity.enforce_user_account_state() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    changed_time timestamptz := clock_timestamp();
BEGIN
    IF OLD.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION 'deleted users cannot be updated'
            USING ERRCODE = '23514', CONSTRAINT = 'users_deleted_terminal_check';
    END IF;

    IF (
        NEW.deletion_requested_at IS DISTINCT FROM OLD.deletion_requested_at
        OR NEW.deletion_scheduled_for IS DISTINCT FROM OLD.deletion_scheduled_for
        OR NEW.deleted_at IS DISTINCT FROM OLD.deleted_at
    ) AND (
        NEW.email IS DISTINCT FROM OLD.email
        OR NEW.username IS DISTINCT FROM OLD.username
        OR NEW.display_name IS DISTINCT FROM OLD.display_name
        OR NEW.profile_confirmation_status IS DISTINCT FROM OLD.profile_confirmation_status
        OR NEW.username_changes_used IS DISTINCT FROM OLD.username_changes_used
        OR NEW.password_hash IS DISTINCT FROM OLD.password_hash
        OR NEW.role IS DISTINCT FROM OLD.role
        OR NEW.email_verified_at IS DISTINCT FROM OLD.email_verified_at
        OR NEW.profile IS DISTINCT FROM OLD.profile
        OR NEW.settings IS DISTINCT FROM OLD.settings
        OR NEW.last_login_at IS DISTINCT FROM OLD.last_login_at
    ) THEN
        RAISE EXCEPTION 'deletion transitions cannot modify account data'
            USING ERRCODE = '23514', CONSTRAINT = 'users_deletion_transition_shape_check';
    END IF;

    IF NEW.email IS DISTINCT FROM OLD.email THEN
        NEW.email_verified_at := NULL;
    END IF;

    IF OLD.deletion_requested_at IS NULL AND NEW.deletion_requested_at IS NOT NULL THEN
        IF OLD.access_status <> 'ACTIVE'
            OR NEW.access_status <> 'SUSPENDED'
            OR NEW.deleted_at IS NOT NULL THEN
            RAISE EXCEPTION 'deletion can only be requested for an active user'
                USING ERRCODE = '23514', CONSTRAINT = 'users_deletion_request_transition_check';
        END IF;
    ELSIF OLD.deletion_requested_at IS NOT NULL AND NEW.deleted_at IS NOT NULL THEN
        IF NEW.deletion_requested_at IS DISTINCT FROM OLD.deletion_requested_at
            OR NEW.deletion_scheduled_for IS DISTINCT FROM OLD.deletion_scheduled_for
            OR OLD.deletion_scheduled_for > changed_time THEN
            RAISE EXCEPTION 'user deletion cannot complete before its scheduled time'
                USING ERRCODE = '23514', CONSTRAINT = 'users_deletion_completion_transition_check';
        END IF;

        NEW.email := NULL;
        NEW.display_name := NEW.username;
        NEW.password_hash := NULL;
        NEW.role := 'USER';
        NEW.access_status := 'SUSPENDED';
        NEW.email_verified_at := NULL;
        NEW.profile := '{}'::jsonb;
        NEW.settings := '{}'::jsonb;
        NEW.last_login_at := NULL;
    ELSIF OLD.deletion_requested_at IS NOT NULL AND NEW.deletion_requested_at IS NULL THEN
        IF OLD.deletion_scheduled_for <= changed_time
            OR NEW.deletion_scheduled_for IS NOT NULL
            OR NEW.deleted_at IS NOT NULL
            OR NEW.access_status <> 'ACTIVE' THEN
            RAISE EXCEPTION 'scheduled deletion can only be cancelled before its deadline'
                USING ERRCODE = '23514', CONSTRAINT = 'users_deletion_cancellation_transition_check';
        END IF;
    ELSIF OLD.deletion_requested_at IS NOT NULL THEN
        RAISE EXCEPTION 'users pending deletion can only be cancelled or deleted'
            USING ERRCODE = '23514', CONSTRAINT = 'users_deletion_pending_update_check';
    ELSIF NEW.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION 'user deletion requires a completed deletion window'
            USING ERRCODE = '23514', CONSTRAINT = 'users_deletion_completion_transition_check';
    END IF;

    IF NEW.auth_version < OLD.auth_version THEN
        RAISE EXCEPTION 'user authentication version cannot decrease'
            USING ERRCODE = '23514', CONSTRAINT = 'users_auth_version_monotonic_check';
    END IF;

    IF NEW.username IS DISTINCT FROM OLD.username
        OR NEW.email IS DISTINCT FROM OLD.email
        OR NEW.password_hash IS DISTINCT FROM OLD.password_hash
        OR NEW.role IS DISTINCT FROM OLD.role
        OR NEW.access_status IS DISTINCT FROM OLD.access_status
        OR NEW.email_verified_at IS DISTINCT FROM OLD.email_verified_at
        OR NEW.deletion_requested_at IS DISTINCT FROM OLD.deletion_requested_at
        OR NEW.deletion_scheduled_for IS DISTINCT FROM OLD.deletion_scheduled_for
        OR NEW.deleted_at IS DISTINCT FROM OLD.deleted_at THEN
        NEW.auth_version := greatest(NEW.auth_version, OLD.auth_version + 1);
    END IF;

    RETURN NEW;
END;
$$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION identity.remove_deleted_user_oauth() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
        DELETE FROM identity.oauth_identities WHERE user_id = NEW.id;
    END IF;
    RETURN NULL;
END;
$$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION identity.touch_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = clock_timestamp();
    RETURN NEW;
END;
$$;

-- +goose StatementEnd

CREATE TABLE identity.oauth_identities (
    id uuid DEFAULT uuidv7() NOT NULL, -- UUIDv7 OAuth identity primary key.
    user_id uuid NOT NULL, -- Owning local user account.
    provider text NOT NULL, -- External authentication provider currently limited to GITHUB.
    provider_user_id text NOT NULL, -- Stable provider-side subject identifier.
    provider_login text, -- Latest provider username or login for display.
    profile jsonb DEFAULT '{}'::jsonb NOT NULL, -- Last normalized public provider profile snapshot.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Time the provider identity was linked.
    updated_at timestamp with time zone DEFAULT now() NOT NULL, -- Last provider identity refresh time.
    CONSTRAINT oauth_identities_profile_object_check CHECK ((jsonb_typeof(profile) = 'object'::text)),
    CONSTRAINT oauth_identities_provider_check CHECK ((provider = 'GITHUB'::text)),
    CONSTRAINT oauth_identities_provider_login_check CHECK (((provider_login IS NULL) OR (btrim(provider_login) <> ''::text))),
    CONSTRAINT oauth_identities_provider_user_id_check CHECK ((btrim(provider_user_id) <> ''::text))
,
    CONSTRAINT oauth_identities_pkey PRIMARY KEY (id)
,
    CONSTRAINT oauth_identities_provider_subject_unique UNIQUE (provider, provider_user_id)
,
    CONSTRAINT oauth_identities_user_provider_unique UNIQUE (user_id, provider)
);

CREATE TABLE identity.users (
    id uuid DEFAULT uuidv7() NOT NULL, -- UUIDv7 user primary key generated by PostgreSQL.
    email text, -- Trimmed lowercase login and notification email, cleared after deletion completes.
    username text NOT NULL COLLATE pg_catalog."C", -- Globally unique lowercase public username.
    display_name text NOT NULL, -- Trimmed user-facing display name not used for identity matching.
    password_hash text, -- Reserved optional password verifier for future password login.
    role text DEFAULT 'USER'::text NOT NULL, -- Base authorization role: USER, ADMIN, or SYS_ADMIN.
    access_status text DEFAULT 'ACTIVE'::text NOT NULL, -- Reversible account access state: ACTIVE or SUSPENDED.
    email_verified_at timestamp with time zone, -- Time the current email address was verified.
    auth_version integer DEFAULT 1 NOT NULL, -- Version incremented to invalidate active authentication sessions.
    profile jsonb DEFAULT '{}'::jsonb NOT NULL, -- Public descriptive profile properties validated by the API.
    settings jsonb DEFAULT '{}'::jsonb NOT NULL, -- Private user preferences validated by the API.
    last_login_at timestamp with time zone, -- Most recent successful authentication time.
    deletion_requested_at timestamp with time zone, -- Time the reversible account-deletion window began.
    deletion_scheduled_for timestamp with time zone, -- Time deletion becomes irreversible, thirty days after the request.
    deleted_at timestamp with time zone, -- Time account anonymization and OAuth removal completed.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Account creation time.
    updated_at timestamp with time zone DEFAULT now() NOT NULL, -- Last account or profile update time maintained by trigger.
    profile_confirmation_status text DEFAULT 'NOT_REQUIRED' NOT NULL, -- First OAuth profile confirmation state: NOT_REQUIRED, PENDING, or CONFIRMED.
    username_changes_used integer DEFAULT 0 NOT NULL, -- Completed optional username changes, limited to one per account.
    CONSTRAINT users_profile_confirmation_status_check CHECK (profile_confirmation_status IN ('NOT_REQUIRED', 'PENDING', 'CONFIRMED')),
    CONSTRAINT users_username_changes_used_check CHECK (username_changes_used BETWEEN 0 AND 1),
    CONSTRAINT users_access_status_check CHECK ((access_status = ANY (ARRAY['ACTIVE'::text, 'SUSPENDED'::text]))),
    CONSTRAINT users_auth_version_check CHECK ((auth_version >= 1)),
    CONSTRAINT users_deleted_data_check CHECK (((deleted_at IS NULL) OR ((display_name = username) AND (password_hash IS NULL) AND (role = 'USER'::text) AND (access_status = 'SUSPENDED'::text) AND (email_verified_at IS NULL) AND (profile = '{}'::jsonb) AND (settings = '{}'::jsonb) AND (last_login_at IS NULL)))),
    CONSTRAINT users_deletion_access_check CHECK (((deletion_requested_at IS NULL) OR (access_status = 'SUSPENDED'::text))),
    CONSTRAINT users_deletion_timeline_check CHECK ((((deletion_requested_at IS NULL) AND (deletion_scheduled_for IS NULL) AND (deleted_at IS NULL)) OR ((deletion_requested_at IS NOT NULL) AND (deletion_scheduled_for IS NOT NULL) AND (deletion_requested_at >= created_at) AND (deletion_scheduled_for = (deletion_requested_at + '30 days'::interval)) AND ((deleted_at IS NULL) OR (deleted_at >= deletion_scheduled_for))))),
    CONSTRAINT users_display_name_check CHECK (((display_name = btrim(display_name)) AND (display_name <> ''::text) AND (char_length(display_name) <= 128))),
    CONSTRAINT users_email_normalized_check CHECK (((email IS NULL) OR ((email = lower(btrim(email))) AND (email <> ''::text)))),
    CONSTRAINT users_email_presence_check CHECK ((((deleted_at IS NULL) AND (email IS NOT NULL)) OR ((deleted_at IS NOT NULL) AND (email IS NULL)))),
    CONSTRAINT users_email_verified_at_check CHECK (((email_verified_at IS NULL) OR ((email IS NOT NULL) AND (email_verified_at >= created_at)))),
    CONSTRAINT users_last_login_at_check CHECK (((last_login_at IS NULL) OR (last_login_at >= created_at))),
    CONSTRAINT users_profile_object_check CHECK ((jsonb_typeof(profile) = 'object'::text)),
    CONSTRAINT users_role_check CHECK ((role = ANY (ARRAY['USER'::text, 'ADMIN'::text, 'SYS_ADMIN'::text]))),
    CONSTRAINT users_settings_object_check CHECK ((jsonb_typeof(settings) = 'object'::text)),
    CONSTRAINT users_username_check CHECK ((username ~ '^[a-z0-9_]{3,32}$'::text))
,
    CONSTRAINT users_pkey PRIMARY KEY (id)
);

CREATE INDEX users_deletion_schedule_idx ON identity.users USING btree (deletion_scheduled_for, id) WHERE ((deletion_scheduled_for IS NOT NULL) AND (deleted_at IS NULL));

CREATE UNIQUE INDEX users_email_unique_idx ON identity.users USING btree (email);

CREATE INDEX users_role_access_status_idx ON identity.users USING btree (role, access_status);

CREATE UNIQUE INDEX users_username_unique_idx ON identity.users USING btree (username);

CREATE TRIGGER oauth_identities_touch_updated_at BEFORE UPDATE ON identity.oauth_identities FOR EACH ROW EXECUTE FUNCTION identity.touch_updated_at();

CREATE TRIGGER users_enforce_account_state BEFORE UPDATE ON identity.users FOR EACH ROW EXECUTE FUNCTION identity.enforce_user_account_state();

CREATE TRIGGER users_remove_oauth_after_deletion AFTER UPDATE ON identity.users FOR EACH ROW EXECUTE FUNCTION identity.remove_deleted_user_oauth();

CREATE TRIGGER users_touch_updated_at BEFORE UPDATE ON identity.users FOR EACH ROW EXECUTE FUNCTION identity.touch_updated_at();

COMMENT ON SCHEMA identity IS 'Authoritative user accounts and OAuth login identities.';

COMMENT ON FUNCTION identity.enforce_user_account_state() IS 'Enforces account access, deletion, anonymization, and authentication-version transitions.';

COMMENT ON FUNCTION identity.remove_deleted_user_oauth() IS 'Removes external login identities after irreversible account deletion.';

COMMENT ON FUNCTION identity.touch_updated_at() IS 'Maintains updated_at on identity rows before update.';

COMMENT ON TABLE identity.oauth_identities IS 'GitHub OAuth login identities without provider access or refresh tokens.';

COMMENT ON COLUMN identity.oauth_identities.id IS 'UUIDv7 OAuth identity primary key.';

COMMENT ON COLUMN identity.oauth_identities.user_id IS 'Owning local user account.';

COMMENT ON COLUMN identity.oauth_identities.provider IS 'External authentication provider currently limited to GITHUB.';

COMMENT ON COLUMN identity.oauth_identities.provider_user_id IS 'Stable provider-side subject identifier.';

COMMENT ON COLUMN identity.oauth_identities.provider_login IS 'Latest provider username or login for display.';

COMMENT ON COLUMN identity.oauth_identities.profile IS 'Last normalized public provider profile snapshot.';

COMMENT ON COLUMN identity.oauth_identities.created_at IS 'Time the provider identity was linked.';

COMMENT ON COLUMN identity.oauth_identities.updated_at IS 'Last provider identity refresh time.';

COMMENT ON TABLE identity.users IS 'User account and profile root with separate access, verification, and deletion state.';

COMMENT ON COLUMN identity.users.id IS 'UUIDv7 user primary key generated by PostgreSQL.';

COMMENT ON COLUMN identity.users.email IS 'Trimmed lowercase login and notification email, cleared after deletion completes.';

COMMENT ON COLUMN identity.users.username IS 'Globally unique lowercase public username.';

COMMENT ON COLUMN identity.users.display_name IS 'Trimmed user-facing display name not used for identity matching.';

COMMENT ON COLUMN identity.users.profile_confirmation_status IS 'First OAuth profile confirmation state: NOT_REQUIRED, PENDING, or CONFIRMED.';

COMMENT ON COLUMN identity.users.username_changes_used IS 'Completed optional username changes, limited to one per account.';

COMMENT ON COLUMN identity.users.password_hash IS 'Reserved optional password verifier for future password login.';

COMMENT ON COLUMN identity.users.role IS 'Base authorization role: USER, ADMIN, or SYS_ADMIN.';

COMMENT ON COLUMN identity.users.access_status IS 'Reversible account access state: ACTIVE or SUSPENDED.';

COMMENT ON COLUMN identity.users.email_verified_at IS 'Time the current email address was verified.';

COMMENT ON COLUMN identity.users.auth_version IS 'Version incremented to invalidate active authentication sessions.';

COMMENT ON COLUMN identity.users.profile IS 'Public descriptive profile properties validated by the API.';

COMMENT ON COLUMN identity.users.settings IS 'Private user preferences validated by the API.';

COMMENT ON COLUMN identity.users.last_login_at IS 'Most recent successful authentication time.';

COMMENT ON COLUMN identity.users.deletion_requested_at IS 'Time the reversible account-deletion window began.';

COMMENT ON COLUMN identity.users.deletion_scheduled_for IS 'Time deletion becomes irreversible, thirty days after the request.';

COMMENT ON COLUMN identity.users.deleted_at IS 'Time account anonymization and OAuth removal completed.';

COMMENT ON COLUMN identity.users.created_at IS 'Account creation time.';

COMMENT ON COLUMN identity.users.updated_at IS 'Last account or profile update time maintained by trigger.';

COMMENT ON TRIGGER oauth_identities_touch_updated_at ON identity.oauth_identities IS 'Maintains the OAuth identity updated_at timestamp.';

COMMENT ON TRIGGER users_enforce_account_state ON identity.users IS 'Enforces valid user account state transitions before update.';

COMMENT ON TRIGGER users_remove_oauth_after_deletion ON identity.users IS 'Removes OAuth identities when account deletion completes.';

COMMENT ON TRIGGER users_touch_updated_at ON identity.users IS 'Maintains the user updated_at timestamp.';

GRANT USAGE ON SCHEMA identity TO api_runtime;

REVOKE ALL ON FUNCTION identity.enforce_user_account_state() FROM PUBLIC;

REVOKE ALL ON FUNCTION identity.remove_deleted_user_oauth() FROM PUBLIC;

REVOKE ALL ON FUNCTION identity.touch_updated_at() FROM PUBLIC;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE identity.oauth_identities TO api_runtime;

GRANT SELECT,INSERT,UPDATE ON TABLE identity.users TO api_runtime;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'The consolidated baseline is forward-only; restore the previous database backup.'; END $$;
-- +goose StatementEnd
