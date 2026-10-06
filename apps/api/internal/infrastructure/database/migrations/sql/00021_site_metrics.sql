-- +goose Up
CREATE TABLE directory.site_metrics (
    site_id uuid PRIMARY KEY REFERENCES directory.sites(id) ON DELETE CASCADE, -- Site whose independent cumulative counters are maintained here.
    click_count bigint NOT NULL DEFAULT 0 CHECK (click_count >= 0), -- Public homepage navigations, including random automatic navigation.
    impression_count bigint NOT NULL DEFAULT 0 CHECK (impression_count >= 0), -- Qualified visible displays, deduplicated per document and site.
    response_count bigint NOT NULL DEFAULT 0 CHECK (response_count >= 0), -- Successful public display responses containing this site.
    query_count bigint NOT NULL DEFAULT 0 CHECK (query_count >= 0), -- Site records returned by public display queries.
    CONSTRAINT site_metrics_safe_counts CHECK (greatest(click_count, impression_count, response_count, query_count) <= 9007199254740991)
);
CREATE TABLE directory.site_metric_events (
    event_id uuid NOT NULL, -- Random delivery identifier; retries retain the same value.
    site_id uuid NOT NULL REFERENCES directory.sites(id) ON DELETE CASCADE, -- Site associated with the delivery identifier.
    kind text NOT NULL CHECK (kind IN ('CLICK', 'IMPRESSION')), -- Metric protected against duplicate delivery.
    PRIMARY KEY (event_id, site_id, kind)
);
GRANT SELECT, INSERT, UPDATE ON directory.site_metrics TO api_runtime;
GRANT SELECT, INSERT ON directory.site_metric_events TO api_runtime;
COMMENT ON TABLE directory.site_metrics IS 'Independent all-time site metrics; writes never alter directory profile revision or timestamps.';
COMMENT ON COLUMN directory.site_metrics.site_id IS 'Site whose independent cumulative counters are maintained here.';
COMMENT ON COLUMN directory.site_metrics.click_count IS 'Public homepage navigations, including random automatic navigation.';
COMMENT ON COLUMN directory.site_metrics.impression_count IS 'Qualified visible displays, deduplicated per document and site.';
COMMENT ON COLUMN directory.site_metrics.response_count IS 'Successful public display responses containing this site.';
COMMENT ON COLUMN directory.site_metrics.query_count IS 'Site records returned by public display queries.';
COMMENT ON TABLE directory.site_metric_events IS 'Minimal permanent idempotency keys without user identity, page address or attribution dimensions.';
COMMENT ON COLUMN directory.site_metric_events.event_id IS 'Random delivery identifier; retries retain the same value.';
COMMENT ON COLUMN directory.site_metric_events.site_id IS 'Site associated with the delivery identifier.';
COMMENT ON COLUMN directory.site_metric_events.kind IS 'Metric protected against duplicate delivery.';

-- +goose Down
DROP TABLE directory.site_metric_events;
DROP TABLE directory.site_metrics;
