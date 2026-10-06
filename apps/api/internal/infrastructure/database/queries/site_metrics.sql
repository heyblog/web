-- name: RecordSiteMetricEvents :exec
WITH accepted AS (
    INSERT INTO directory.site_metric_events (event_id, site_id, kind)
    SELECT event.event_id, site.id, sqlc.arg(kind)::text
      FROM (SELECT unnest(sqlc.arg(event_ids)::uuid[]) AS event_id,
                   unnest(sqlc.arg(short_ids)::text[]) AS short_id) AS event
      JOIN directory.sites AS site ON site.short_id = event.short_id
     WHERE site.visibility IN ('VISIBLE', 'HIDDEN')
     ORDER BY site.id, event.event_id
    ON CONFLICT DO NOTHING
    RETURNING site_id, kind
)
INSERT INTO directory.site_metrics (site_id, click_count, impression_count)
SELECT site_id, count(*) FILTER (WHERE kind = 'CLICK'), count(*) FILTER (WHERE kind = 'IMPRESSION')
  FROM accepted GROUP BY site_id ORDER BY site_id
ON CONFLICT (site_id) DO UPDATE SET
    click_count = directory.site_metrics.click_count + EXCLUDED.click_count,
    impression_count = directory.site_metrics.impression_count + EXCLUDED.impression_count;

-- name: IncrementSiteDisplayMetrics :exec
INSERT INTO directory.site_metrics (site_id, query_count, response_count)
SELECT site.id, sqlc.arg(query_increment)::bigint, sqlc.arg(response_increment)::bigint
  FROM directory.sites AS site
 WHERE site.short_id = ANY(sqlc.arg(short_ids)::text[])
 ORDER BY site.id
ON CONFLICT (site_id) DO UPDATE SET
    query_count = directory.site_metrics.query_count + EXCLUDED.query_count,
    response_count = directory.site_metrics.response_count + EXCLUDED.response_count;

-- name: GetSiteMetrics :many
SELECT site.short_id,
       coalesce(metric.click_count, 0)::bigint AS click_count,
       coalesce(metric.impression_count, 0)::bigint AS impression_count,
       coalesce(metric.response_count, 0)::bigint AS response_count,
       coalesce(metric.query_count, 0)::bigint AS query_count
  FROM directory.sites AS site
  LEFT JOIN directory.site_metrics AS metric ON metric.site_id = site.id
 WHERE site.short_id = ANY(sqlc.arg(short_ids)::text[]);

-- name: GetSiteOutboundTarget :one
SELECT scheme, normalized_host, base_path FROM directory.sites
 WHERE short_id = $1 AND visibility IN ('VISIBLE', 'HIDDEN');
