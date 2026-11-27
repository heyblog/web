-- name: InsertMonitoringRound :execrows
INSERT INTO directory.site_monitoring_rounds (site_id, generation, round_id, round_at, cn_outcome, gl_outcome)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (site_id,generation,round_id) DO NOTHING;

-- name: ListMonitoringHealthWindow :many
SELECT * FROM directory.site_monitoring_rounds
WHERE site_id=$1 AND generation=$2 AND (cn_outcome='OK' OR gl_outcome='OK' OR (cn_outcome='FAIL' AND gl_outcome='FAIL'))
ORDER BY round_at DESC,round_id DESC LIMIT 10;

-- name: ListMonitoringScopeWindow :many
SELECT * FROM directory.site_monitoring_rounds
WHERE site_id=$1 AND generation=$2 AND cn_outcome<>'UNKNOWN' AND gl_outcome<>'UNKNOWN'
ORDER BY round_at DESC,round_id DESC LIMIT 5;

-- name: UpdateMonitoringSummary :exec
UPDATE directory.sites SET status=COALESCE(sqlc.narg(status)::text,status),
access_scope=COALESCE(sqlc.narg(access_scope)::text,access_scope),
last_checked_at=GREATEST(last_checked_at,sqlc.arg(checked_at)::timestamptz)
WHERE id=sqlc.arg(site_id)::uuid AND monitoring_generation=sqlc.arg(generation)::bigint;
