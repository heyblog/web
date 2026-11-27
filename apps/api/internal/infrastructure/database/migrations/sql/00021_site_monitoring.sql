-- +goose Up
SELECT directory.backup_check_schema();

-- Freeze the schema-eight inventory before changing any columns or datasets.
-- +goose StatementBegin
DO $freeze$
DECLARE inventory text;
BEGIN
 SELECT string_agg(format('(%L::text,%L::text,%L::text)',dataset,column_name,type_name),',' ORDER BY dataset,column_name)
 INTO inventory FROM directory.backup_columns();
 EXECUTE 'CREATE FUNCTION directory.backup_schema8_columns() RETURNS TABLE(dataset text,column_name text,type_name text)
 LANGUAGE sql IMMUTABLE SET search_path TO pg_catalog AS ' || quote_literal('VALUES '||inventory);
END $freeze$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION directory.backup_schema8_columns() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION directory.backup_schema8_columns() TO api_runtime;
COMMENT ON FUNCTION directory.backup_schema8_columns() IS 'Frozen schema-eight source column inventory captured before schema-nine evolution.';

ALTER TABLE directory.sites
    ALTER COLUMN access_scope DROP NOT NULL,
    ALTER COLUMN access_scope DROP DEFAULT, -- Stable observed access region: CN, GL, ALL; null until enough complete samples.
    DROP CONSTRAINT sites_access_scope_check,
    ADD COLUMN status text CHECK (status IN ('OK', 'WARN', 'DOWN')), -- Observed health: OK, WARN, DOWN; null until ten valid rounds.
    ADD COLUMN monitoring_generation bigint NOT NULL DEFAULT 1 CHECK (monitoring_generation > 0), -- Sampling generation incremented when the canonical address changes or data is restored.
    ADD COLUMN last_checked_at timestamptz; -- Latest completed monitoring round, independent of content modification time.

CREATE TABLE directory.site_monitoring_rounds (
    site_id uuid NOT NULL REFERENCES directory.sites(id) ON DELETE CASCADE, -- Monitored canonical site.
    generation bigint NOT NULL CHECK (generation > 0), -- Address generation captured before the request.
    round_id text NOT NULL CHECK (btrim(round_id) <> ''), -- Stable task identifier shared by retries and recovery.
    round_at timestamptz NOT NULL, -- Logical round start time used to order sampling windows.
    cn_outcome text NOT NULL -- Mainland outcome: OK, FAIL, UNKNOWN.
        CHECK (cn_outcome IN ('OK', 'FAIL', 'UNKNOWN')),
    gl_outcome text NOT NULL -- Overseas outcome: OK, FAIL, UNKNOWN.
        CHECK (gl_outcome IN ('OK', 'FAIL', 'UNKNOWN')),
    PRIMARY KEY (site_id, generation, round_id)
);
CREATE INDEX site_monitoring_rounds_window_idx ON directory.site_monitoring_rounds (site_id, generation, round_at DESC, round_id DESC);
GRANT SELECT, INSERT, UPDATE, DELETE ON directory.site_monitoring_rounds TO api_runtime;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION directory.touch_site() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.scheme, NEW.normalized_host, NEW.base_path) IS DISTINCT FROM ROW(OLD.scheme, OLD.normalized_host, OLD.base_path) THEN
        NEW.status := NULL;
        NEW.access_scope := NULL;
        NEW.last_checked_at := NULL;
        NEW.monitoring_generation := OLD.monitoring_generation + 1;
        DELETE FROM directory.site_monitoring_rounds WHERE site_id = OLD.id;
    END IF;
    IF NEW.revision IS DISTINCT FROM OLD.revision OR
       (to_jsonb(NEW) - ARRAY['status','access_scope','monitoring_generation','last_checked_at','revision','updated_at'])
       IS DISTINCT FROM (to_jsonb(OLD) - ARRAY['status','access_scope','monitoring_generation','last_checked_at','revision','updated_at']) THEN
        NEW.revision := OLD.revision + 1;
        NEW.updated_at := clock_timestamp();
    ELSE
        NEW.revision := OLD.revision;
        NEW.updated_at := OLD.updated_at;
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
COMMENT ON FUNCTION directory.touch_site() IS 'Advances explicit business revisions and changed site content, resets address samples, and preserves content metadata for monitoring-only updates.';

-- Install the content-aware trigger before clearing previous conclusions.
UPDATE directory.sites SET access_scope = NULL, status = NULL;
ALTER TABLE directory.sites ADD CONSTRAINT sites_access_scope_check CHECK (access_scope IN ('CN', 'GL', 'ALL'));

COMMENT ON COLUMN directory.sites.last_checked_at IS 'Latest completed monitoring round, independent of content modification time.';
COMMENT ON COLUMN directory.sites.access_scope IS 'Stable observed access region: CN, GL, ALL; null until enough complete samples.';
COMMENT ON COLUMN directory.sites.status IS 'Observed health: OK, WARN, DOWN; null until ten valid rounds.';
COMMENT ON COLUMN directory.sites.monitoring_generation IS 'Sampling generation incremented when the canonical address changes or data is restored.';
COMMENT ON COLUMN directory.site_monitoring_rounds.generation IS 'Address generation captured before the request.';
COMMENT ON COLUMN directory.site_monitoring_rounds.round_id IS 'Stable task identifier shared by retries and recovery.';
COMMENT ON COLUMN directory.site_monitoring_rounds.site_id IS 'Monitored canonical site.';
COMMENT ON COLUMN directory.site_monitoring_rounds.round_at IS 'Logical round start time used to order sampling windows.';
COMMENT ON COLUMN directory.site_monitoring_rounds.cn_outcome IS 'Mainland outcome: OK, FAIL, UNKNOWN.';
COMMENT ON COLUMN directory.site_monitoring_rounds.gl_outcome IS 'Overseas outcome: OK, FAIL, UNKNOWN.';
COMMENT ON TABLE directory.site_monitoring_rounds IS 'Idempotent dual-region rounds; UNKNOWN platform outcomes never count as target failure.';
COMMENT ON COLUMN directory.site_monitoring_rounds.site_id IS 'Monitored canonical site.';

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'The monitoring migration is forward-only; restore the previous database backup.'; END $$;
-- +goose StatementEnd
