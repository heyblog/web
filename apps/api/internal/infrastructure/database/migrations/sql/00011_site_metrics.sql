-- +goose Up

SET LOCAL check_function_bodies = false;

CREATE TABLE directory.site_metric_events (
    event_id uuid NOT NULL, -- Random delivery identifier; retries retain the same value.
    site_id uuid NOT NULL, -- Site associated with the delivery identifier.
    kind text NOT NULL, -- Metric protected against duplicate delivery.
    CONSTRAINT site_metric_events_kind_check CHECK ((kind = ANY (ARRAY['CLICK'::text, 'IMPRESSION'::text])))
,
    CONSTRAINT site_metric_events_pkey PRIMARY KEY (event_id, site_id, kind)
);

CREATE TABLE directory.site_metrics (
    site_id uuid NOT NULL, -- Site whose independent cumulative counters are maintained here.
    click_count bigint DEFAULT 0 NOT NULL, -- Public homepage navigations, including random automatic navigation.
    impression_count bigint DEFAULT 0 NOT NULL, -- Qualified visible displays, deduplicated per document and site.
    response_count bigint DEFAULT 0 NOT NULL, -- Successful public display responses containing this site.
    query_count bigint DEFAULT 0 NOT NULL, -- Site records returned by public display queries.
    CONSTRAINT site_metrics_click_count_check CHECK ((click_count >= 0)),
    CONSTRAINT site_metrics_impression_count_check CHECK ((impression_count >= 0)),
    CONSTRAINT site_metrics_query_count_check CHECK ((query_count >= 0)),
    CONSTRAINT site_metrics_response_count_check CHECK ((response_count >= 0)),
    CONSTRAINT site_metrics_safe_counts CHECK ((GREATEST(click_count, impression_count, response_count, query_count) <= '9007199254740991'::bigint))
,
    CONSTRAINT site_metrics_pkey PRIMARY KEY (site_id)
) WITH (fillfactor = 70, autovacuum_vacuum_scale_factor = 0.02);

COMMENT ON TABLE directory.site_metric_events IS 'Minimal permanent idempotency keys without user identity, page address or attribution dimensions.';

COMMENT ON COLUMN directory.site_metric_events.event_id IS 'Random delivery identifier; retries retain the same value.';

COMMENT ON COLUMN directory.site_metric_events.site_id IS 'Site associated with the delivery identifier.';

COMMENT ON COLUMN directory.site_metric_events.kind IS 'Metric protected against duplicate delivery.';

COMMENT ON TABLE directory.site_metrics IS 'Independent all-time site metrics; writes never alter directory profile revision or timestamps.';

COMMENT ON COLUMN directory.site_metrics.site_id IS 'Site whose independent cumulative counters are maintained here.';

COMMENT ON COLUMN directory.site_metrics.click_count IS 'Public homepage navigations, including random automatic navigation.';

COMMENT ON COLUMN directory.site_metrics.impression_count IS 'Qualified visible displays, deduplicated per document and site.';

COMMENT ON COLUMN directory.site_metrics.response_count IS 'Successful public display responses containing this site.';

COMMENT ON COLUMN directory.site_metrics.query_count IS 'Site records returned by public display queries.';

GRANT SELECT,INSERT ON TABLE directory.site_metric_events TO api_runtime;

GRANT SELECT,INSERT,UPDATE ON TABLE directory.site_metrics TO api_runtime;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'The consolidated baseline is forward-only; restore the previous database backup.'; END $$;
-- +goose StatementEnd
