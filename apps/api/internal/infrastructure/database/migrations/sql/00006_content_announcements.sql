-- +goose Up

SET LOCAL check_function_bodies = false;

CREATE SCHEMA content;

-- +goose StatementBegin

CREATE FUNCTION content.maintain_announcement() RETURNS trigger
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'content'
    AS $$
DECLARE
    changed_time timestamptz := clock_timestamp();
BEGIN
    IF pg_trigger_depth() > 1
        AND NEW.id IS NOT DISTINCT FROM OLD.id
        AND NEW.kind IS NOT DISTINCT FROM OLD.kind
        AND NEW.title IS NOT DISTINCT FROM OLD.title
        AND NEW.body_markdown IS NOT DISTINCT FROM OLD.body_markdown
        AND NEW.status IS NOT DISTINCT FROM OLD.status
        AND NEW.priority IS NOT DISTINCT FROM OLD.priority
        AND NEW.action_type IS NOT DISTINCT FROM OLD.action_type
        AND NEW.action_label IS NOT DISTINCT FROM OLD.action_label
        AND NEW.action_path IS NOT DISTINCT FROM OLD.action_path
        AND NEW.action_external_url IS NOT DISTINCT FROM OLD.action_external_url
        AND NEW.starts_at IS NOT DISTINCT FROM OLD.starts_at
        AND NEW.ends_at IS NOT DISTINCT FROM OLD.ends_at
        AND NEW.published_at IS NOT DISTINCT FROM OLD.published_at
        AND NEW.archived_at IS NOT DISTINCT FROM OLD.archived_at
        AND NEW.row_version IS NOT DISTINCT FROM OLD.row_version
        AND NEW.created_at IS NOT DISTINCT FROM OLD.created_at
        AND NEW.updated_at IS NOT DISTINCT FROM OLD.updated_at
        AND (
            NEW.created_by IS DISTINCT FROM OLD.created_by
            OR NEW.updated_by IS DISTINCT FROM OLD.updated_by
            OR NEW.published_by IS DISTINCT FROM OLD.published_by
            OR NEW.archived_by IS DISTINCT FROM OLD.archived_by
        )
        AND (
            NEW.created_by IS NOT DISTINCT FROM OLD.created_by
            OR (OLD.created_by IS NOT NULL AND NEW.created_by IS NULL)
        )
        AND (
            NEW.updated_by IS NOT DISTINCT FROM OLD.updated_by
            OR (OLD.updated_by IS NOT NULL AND NEW.updated_by IS NULL)
        )
        AND (
            NEW.published_by IS NOT DISTINCT FROM OLD.published_by
            OR (OLD.published_by IS NOT NULL AND NEW.published_by IS NULL)
        )
        AND (
            NEW.archived_by IS NOT DISTINCT FROM OLD.archived_by
            OR (OLD.archived_by IS NOT NULL AND NEW.archived_by IS NULL)
        ) THEN
        RETURN NEW;
    END IF;

    IF OLD.status = 'ARCHIVED' THEN
        RAISE EXCEPTION 'archived announcements are immutable'
            USING ERRCODE = '23514', CONSTRAINT = 'announcements_status_transition_check';
    END IF;

    IF OLD.status = 'DRAFT' AND NEW.status NOT IN ('DRAFT', 'PUBLISHED') THEN
        RAISE EXCEPTION 'draft announcements may only remain drafts or be published'
            USING ERRCODE = '23514', CONSTRAINT = 'announcements_status_transition_check';
    END IF;

    IF OLD.status = 'PUBLISHED' AND NEW.status NOT IN ('PUBLISHED', 'ARCHIVED') THEN
        RAISE EXCEPTION 'published announcements may only remain published or be archived'
            USING ERRCODE = '23514', CONSTRAINT = 'announcements_status_transition_check';
    END IF;

    IF OLD.status = 'PUBLISHED' AND OLD.starts_at <= changed_time THEN
        IF NEW.kind IS DISTINCT FROM OLD.kind
            OR NEW.starts_at IS DISTINCT FROM OLD.starts_at
            OR NEW.published_at IS DISTINCT FROM OLD.published_at
            OR NEW.published_by IS DISTINCT FROM OLD.published_by THEN
            RAISE EXCEPTION 'public announcement identity and publication start are immutable'
                USING ERRCODE = '23514', CONSTRAINT = 'announcements_public_identity_check';
        END IF;

        INSERT INTO content.announcement_revisions (
            announcement_id,
            revision,
            kind,
            title,
            body_markdown,
            priority,
            action_type,
            action_label,
            action_path,
            action_external_url,
            starts_at,
            ends_at,
            published_at,
            published_by,
            changed_by,
            changed_at
        ) VALUES (
            OLD.id,
            OLD.row_version,
            OLD.kind,
            OLD.title,
            OLD.body_markdown,
            OLD.priority,
            OLD.action_type,
            OLD.action_label,
            OLD.action_path,
            OLD.action_external_url,
            OLD.starts_at,
            OLD.ends_at,
            OLD.published_at,
            OLD.published_by,
            NEW.updated_by,
            changed_time
        );
    END IF;

    NEW.row_version := OLD.row_version + 1;
    NEW.updated_at := changed_time;
    RETURN NEW;
END;
$$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION content.prevent_announcement_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF OLD.status <> 'DRAFT' THEN
        RAISE EXCEPTION 'only draft announcements may be physically deleted'
            USING ERRCODE = '23514', CONSTRAINT = 'announcements_delete_draft_only_check';
    END IF;
    RETURN OLD;
END;
$$;

-- +goose StatementEnd

CREATE TABLE content.announcement_revisions (
    announcement_id uuid NOT NULL, -- Announcement whose prior published state was captured.
    revision bigint NOT NULL, -- Announcement row version represented by this immutable snapshot.
    kind text NOT NULL, -- Presentation kind captured in the snapshot.
    title text NOT NULL, -- Plain-text title captured in the snapshot.
    body_markdown text, -- Compact Markdown source captured in the snapshot.
    priority integer NOT NULL, -- Main announcement ordering priority captured in the snapshot.
    action_type text NOT NULL, -- Primary action kind captured in the snapshot.
    action_label text, -- Primary action button label captured in the snapshot.
    action_path text, -- Internal primary action target captured in the snapshot.
    action_external_url text, -- External primary action target captured in the snapshot.
    starts_at timestamp with time zone NOT NULL, -- Public visibility start captured in the snapshot.
    ends_at timestamp with time zone, -- Public visibility end captured in the snapshot.
    published_at timestamp with time zone NOT NULL, -- Publication commitment time captured in the snapshot.
    published_by uuid, -- Publication actor captured in the snapshot.
    changed_by uuid, -- User whose update caused the snapshot.
    changed_at timestamp with time zone DEFAULT now() NOT NULL, -- Time the immutable snapshot was recorded.
    CONSTRAINT announcement_revisions_action_external_url_check CHECK (((action_external_url IS NULL) OR ((action_external_url = btrim(action_external_url)) AND (action_external_url ~ '^https?://[^[:space:]]+$'::text)))),
    CONSTRAINT announcement_revisions_action_label_check CHECK (((action_label IS NULL) OR (btrim(action_label) <> ''::text))),
    CONSTRAINT announcement_revisions_action_path_check CHECK (((action_path IS NULL) OR ((action_path = btrim(action_path)) AND ("left"(action_path, 1) = '/'::text) AND ("left"(action_path, 2) <> '//'::text)))),
    CONSTRAINT announcement_revisions_action_shape_check CHECK ((((action_type = 'NONE'::text) AND (action_label IS NULL) AND (action_path IS NULL) AND (action_external_url IS NULL)) OR ((action_type = 'INTERNAL'::text) AND (action_label IS NOT NULL) AND (action_path IS NOT NULL) AND (action_external_url IS NULL)) OR ((action_type = 'EXTERNAL'::text) AND (action_label IS NOT NULL) AND (action_path IS NULL) AND (action_external_url IS NOT NULL)))),
    CONSTRAINT announcement_revisions_action_type_check CHECK ((action_type = ANY (ARRAY['NONE'::text, 'INTERNAL'::text, 'EXTERNAL'::text]))),
    CONSTRAINT announcement_revisions_banner_priority_check CHECK (((kind = 'MAIN'::text) OR (priority = 0))),
    CONSTRAINT announcement_revisions_body_markdown_check CHECK (((body_markdown IS NULL) OR (btrim(body_markdown) <> ''::text))),
    CONSTRAINT announcement_revisions_kind_check CHECK ((kind = ANY (ARRAY['MAIN'::text, 'BANNER'::text]))),
    CONSTRAINT announcement_revisions_revision_check CHECK ((revision >= 1)),
    CONSTRAINT announcement_revisions_title_check CHECK (((btrim(title) <> ''::text) AND (char_length(title) <= 256))),
    CONSTRAINT announcement_revisions_window_check CHECK (((ends_at IS NULL) OR (ends_at > starts_at)))
,
    CONSTRAINT announcement_revisions_pkey PRIMARY KEY (announcement_id, revision)
);

CREATE TABLE content.announcements (
    id uuid DEFAULT uuidv7() NOT NULL, -- UUIDv7 announcement primary key generated by PostgreSQL.
    kind text NOT NULL, -- Exclusive presentation kind: MAIN or BANNER.
    title text NOT NULL, -- Required plain-text announcement title limited to 256 characters.
    body_markdown text, -- Optional compact Markdown source rendered by API consumers.
    status text DEFAULT 'DRAFT'::text NOT NULL, -- Stored publication lifecycle state: DRAFT, PUBLISHED, or ARCHIVED.
    priority integer DEFAULT 0 NOT NULL, -- Main announcement ordering priority where larger values sort first.
    action_type text DEFAULT 'NONE'::text NOT NULL, -- Optional primary action kind: NONE, INTERNAL, or EXTERNAL.
    action_label text, -- Optional nonblank primary action button label.
    action_path text, -- Optional root-relative internal primary action target.
    action_external_url text, -- Optional absolute HTTP or HTTPS external primary action target.
    starts_at timestamp with time zone, -- Inclusive start of the public visibility window.
    ends_at timestamp with time zone, -- Optional exclusive end of the public visibility window.
    published_at timestamp with time zone, -- Time an administrator committed the announcement for publication.
    archived_at timestamp with time zone, -- Time an administrator permanently archived the announcement.
    created_by uuid, -- User that created the announcement draft.
    updated_by uuid, -- User responsible for the latest announcement update.
    published_by uuid, -- User that committed the announcement for publication.
    archived_by uuid, -- User that permanently archived the announcement.
    row_version bigint DEFAULT 1 NOT NULL, -- Optimistic concurrency version incremented on every managed update.
    created_at timestamp with time zone DEFAULT now() NOT NULL, -- Announcement creation time.
    updated_at timestamp with time zone DEFAULT now() NOT NULL, -- Latest managed announcement update time.
    CONSTRAINT announcements_action_external_url_check CHECK (((action_external_url IS NULL) OR ((action_external_url = btrim(action_external_url)) AND (action_external_url ~ '^https?://[^[:space:]]+$'::text)))),
    CONSTRAINT announcements_action_label_check CHECK (((action_label IS NULL) OR (btrim(action_label) <> ''::text))),
    CONSTRAINT announcements_action_path_check CHECK (((action_path IS NULL) OR ((action_path = btrim(action_path)) AND ("left"(action_path, 1) = '/'::text) AND ("left"(action_path, 2) <> '//'::text)))),
    CONSTRAINT announcements_action_shape_check CHECK ((((action_type = 'NONE'::text) AND (action_label IS NULL) AND (action_path IS NULL) AND (action_external_url IS NULL)) OR ((action_type = 'INTERNAL'::text) AND (action_label IS NOT NULL) AND (action_path IS NOT NULL) AND (action_external_url IS NULL)) OR ((action_type = 'EXTERNAL'::text) AND (action_label IS NOT NULL) AND (action_path IS NULL) AND (action_external_url IS NOT NULL)))),
    CONSTRAINT announcements_action_type_check CHECK ((action_type = ANY (ARRAY['NONE'::text, 'INTERNAL'::text, 'EXTERNAL'::text]))),
    CONSTRAINT announcements_banner_priority_check CHECK (((kind = 'MAIN'::text) OR (priority = 0))),
    CONSTRAINT announcements_body_markdown_check CHECK (((body_markdown IS NULL) OR (btrim(body_markdown) <> ''::text))),
    CONSTRAINT announcements_kind_check CHECK ((kind = ANY (ARRAY['MAIN'::text, 'BANNER'::text]))),
    CONSTRAINT announcements_lifecycle_shape_check CHECK ((((status = 'DRAFT'::text) AND (published_at IS NULL) AND (archived_at IS NULL)) OR ((status = 'PUBLISHED'::text) AND (starts_at IS NOT NULL) AND (published_at IS NOT NULL) AND (archived_at IS NULL)) OR ((status = 'ARCHIVED'::text) AND (starts_at IS NOT NULL) AND (published_at IS NOT NULL) AND (archived_at IS NOT NULL)))),
    CONSTRAINT announcements_row_version_check CHECK ((row_version >= 1)),
    CONSTRAINT announcements_status_check CHECK ((status = ANY (ARRAY['DRAFT'::text, 'PUBLISHED'::text, 'ARCHIVED'::text]))),
    CONSTRAINT announcements_title_check CHECK (((btrim(title) <> ''::text) AND (char_length(title) <= 256))),
    CONSTRAINT announcements_window_check CHECK (((ends_at IS NULL) OR ((starts_at IS NOT NULL) AND (ends_at > starts_at))))
,
    CONSTRAINT announcements_banner_window_exclude EXCLUDE USING gist (tstzrange(starts_at, ends_at, '[)'::text) WITH &&) WHERE (((kind = 'BANNER'::text) AND (status = 'PUBLISHED'::text)))
,
    CONSTRAINT announcements_pkey PRIMARY KEY (id)
);

CREATE INDEX announcement_revisions_changed_idx ON content.announcement_revisions USING btree (announcement_id, changed_at DESC, revision DESC);

CREATE INDEX announcements_active_main_idx ON content.announcements USING btree (priority DESC, starts_at DESC, id DESC) WHERE ((kind = 'MAIN'::text) AND (status = 'PUBLISHED'::text));

CREATE INDEX announcements_management_idx ON content.announcements USING btree (status, kind, updated_at DESC, id DESC);

CREATE INDEX announcements_public_archive_idx ON content.announcements USING btree (starts_at DESC, id DESC) WHERE ((kind = 'MAIN'::text) AND (status = ANY (ARRAY['PUBLISHED'::text, 'ARCHIVED'::text])));

CREATE TRIGGER announcements_maintain BEFORE UPDATE ON content.announcements FOR EACH ROW EXECUTE FUNCTION content.maintain_announcement();

CREATE TRIGGER announcements_prevent_delete BEFORE DELETE ON content.announcements FOR EACH ROW EXECUTE FUNCTION content.prevent_announcement_delete();

COMMENT ON SCHEMA content IS 'Authoritative publishable content and immutable publication history.';

COMMENT ON FUNCTION content.maintain_announcement() IS 'Validates announcement transitions, permits referential actor cleanup, captures public revisions, and maintains row versions and update times.';

COMMENT ON FUNCTION content.prevent_announcement_delete() IS 'Rejects physical deletion of every announcement except drafts.';

COMMENT ON TABLE content.announcement_revisions IS 'Immutable typed snapshots captured before updates to announcements that have become public.';

COMMENT ON COLUMN content.announcement_revisions.announcement_id IS 'Announcement whose prior published state was captured.';

COMMENT ON COLUMN content.announcement_revisions.revision IS 'Announcement row version represented by this immutable snapshot.';

COMMENT ON COLUMN content.announcement_revisions.kind IS 'Presentation kind captured in the snapshot.';

COMMENT ON COLUMN content.announcement_revisions.title IS 'Plain-text title captured in the snapshot.';

COMMENT ON COLUMN content.announcement_revisions.body_markdown IS 'Compact Markdown source captured in the snapshot.';

COMMENT ON COLUMN content.announcement_revisions.priority IS 'Main announcement ordering priority captured in the snapshot.';

COMMENT ON COLUMN content.announcement_revisions.action_type IS 'Primary action kind captured in the snapshot.';

COMMENT ON COLUMN content.announcement_revisions.action_label IS 'Primary action button label captured in the snapshot.';

COMMENT ON COLUMN content.announcement_revisions.action_path IS 'Internal primary action target captured in the snapshot.';

COMMENT ON COLUMN content.announcement_revisions.action_external_url IS 'External primary action target captured in the snapshot.';

COMMENT ON COLUMN content.announcement_revisions.starts_at IS 'Public visibility start captured in the snapshot.';

COMMENT ON COLUMN content.announcement_revisions.ends_at IS 'Public visibility end captured in the snapshot.';

COMMENT ON COLUMN content.announcement_revisions.published_at IS 'Publication commitment time captured in the snapshot.';

COMMENT ON COLUMN content.announcement_revisions.published_by IS 'Publication actor captured in the snapshot.';

COMMENT ON COLUMN content.announcement_revisions.changed_by IS 'User whose update caused the snapshot.';

COMMENT ON COLUMN content.announcement_revisions.changed_at IS 'Time the immutable snapshot was recorded.';

COMMENT ON TABLE content.announcements IS 'Main and banner announcements with time-driven visibility and controlled lifecycle.';

COMMENT ON COLUMN content.announcements.id IS 'UUIDv7 announcement primary key generated by PostgreSQL.';

COMMENT ON COLUMN content.announcements.kind IS 'Exclusive presentation kind: MAIN or BANNER.';

COMMENT ON COLUMN content.announcements.title IS 'Required plain-text announcement title limited to 256 characters.';

COMMENT ON COLUMN content.announcements.body_markdown IS 'Optional compact Markdown source rendered by API consumers.';

COMMENT ON COLUMN content.announcements.status IS 'Stored publication lifecycle state: DRAFT, PUBLISHED, or ARCHIVED.';

COMMENT ON COLUMN content.announcements.priority IS 'Main announcement ordering priority where larger values sort first.';

COMMENT ON COLUMN content.announcements.action_type IS 'Optional primary action kind: NONE, INTERNAL, or EXTERNAL.';

COMMENT ON COLUMN content.announcements.action_label IS 'Optional nonblank primary action button label.';

COMMENT ON COLUMN content.announcements.action_path IS 'Optional root-relative internal primary action target.';

COMMENT ON COLUMN content.announcements.action_external_url IS 'Optional absolute HTTP or HTTPS external primary action target.';

COMMENT ON COLUMN content.announcements.starts_at IS 'Inclusive start of the public visibility window.';

COMMENT ON COLUMN content.announcements.ends_at IS 'Optional exclusive end of the public visibility window.';

COMMENT ON COLUMN content.announcements.published_at IS 'Time an administrator committed the announcement for publication.';

COMMENT ON COLUMN content.announcements.archived_at IS 'Time an administrator permanently archived the announcement.';

COMMENT ON COLUMN content.announcements.created_by IS 'User that created the announcement draft.';

COMMENT ON COLUMN content.announcements.updated_by IS 'User responsible for the latest announcement update.';

COMMENT ON COLUMN content.announcements.published_by IS 'User that committed the announcement for publication.';

COMMENT ON COLUMN content.announcements.archived_by IS 'User that permanently archived the announcement.';

COMMENT ON COLUMN content.announcements.row_version IS 'Optimistic concurrency version incremented on every managed update.';

COMMENT ON COLUMN content.announcements.created_at IS 'Announcement creation time.';

COMMENT ON COLUMN content.announcements.updated_at IS 'Latest managed announcement update time.';

COMMENT ON TRIGGER announcements_maintain ON content.announcements IS 'Maintains announcement lifecycle, actor references, revision history, row version, and update time.';

COMMENT ON TRIGGER announcements_prevent_delete ON content.announcements IS 'Allows physical deletion only while an announcement remains a draft.';

GRANT USAGE ON SCHEMA content TO api_runtime;

REVOKE ALL ON FUNCTION content.maintain_announcement() FROM PUBLIC;

REVOKE ALL ON FUNCTION content.prevent_announcement_delete() FROM PUBLIC;

GRANT SELECT ON TABLE content.announcement_revisions TO api_runtime;

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE content.announcements TO api_runtime;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'The consolidated baseline is forward-only; restore the previous database backup.'; END $$;
-- +goose StatementEnd
