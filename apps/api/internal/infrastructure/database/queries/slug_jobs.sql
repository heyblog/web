-- name: SelectSlugJobTags :many
SELECT id,name,slug,description,is_enabled,updated_at
FROM directory.tag_dictionary
WHERE (sqlc.arg(kind)::text <> 'ids' OR id = ANY(sqlc.arg(ids)::uuid[]))
  AND (sqlc.arg(kind)::text <> 'invalid' OR slug LIKE 'legacy-%' OR slug !~ '^[a-z0-9]+(-[a-z0-9]+)*$' OR char_length(slug)>128)
  AND (sqlc.arg(kind)::text <> 'filter' OR
       ((sqlc.narg(enabled)::boolean IS NULL OR is_enabled=sqlc.narg(enabled)::boolean)
        AND (sqlc.arg(query)::text='' OR strpos(lower(name),lower(sqlc.arg(query)::text))>0 OR strpos(slug,lower(sqlc.arg(query)::text))>0)))
ORDER BY id LIMIT sqlc.arg(max_rows)::integer;

-- name: InsertSlugJob :one
INSERT INTO directory.slug_generation_jobs(owner_id,identity_ip_hash,model_id,items)
VALUES(sqlc.arg(owner_id)::uuid,sqlc.arg(identity_ip_hash)::text,sqlc.arg(model_id)::text,sqlc.arg(items)::jsonb)
RETURNING *;

-- name: GetSlugJob :one
SELECT * FROM directory.slug_generation_jobs WHERE id=$1;

-- name: LockSlugJob :one
SELECT * FROM directory.slug_generation_jobs WHERE id=$1 FOR UPDATE;

-- name: ListSlugJobs :many
SELECT * FROM directory.slug_generation_jobs
WHERE sqlc.arg(system_admin)::boolean OR owner_id=sqlc.arg(owner_id)::uuid
ORDER BY CASE WHEN status IN ('queued','running','paused','ready') THEN 0 ELSE 1 END,
created_at DESC,id DESC;

-- name: UpdateSlugJob :one
UPDATE directory.slug_generation_jobs
SET status=sqlc.arg(status)::text, items=sqlc.arg(items)::jsonb,
    pause_code=sqlc.arg(pause_code)::text,resume_after=sqlc.narg(resume_after)::timestamptz,
    revision=revision+1,updated_at=now(),
    lease_token=CASE WHEN sqlc.arg(status)::text='running' THEN lease_token ELSE '' END,
    lease_until=CASE WHEN sqlc.arg(status)::text='running' THEN lease_until ELSE NULL END
WHERE id=sqlc.arg(id)::uuid AND revision=sqlc.arg(expected_revision)::bigint
RETURNING *;

-- name: NextSlugJob :one
SELECT * FROM directory.slug_generation_jobs
WHERE status='queued' OR (status='running' AND lease_until<now())
   OR (status='paused' AND pause_code IN ('slug_rate_limited','slug_concurrency_limited','generation_in_progress') AND resume_after<=now())
ORDER BY created_at,id LIMIT 1 FOR UPDATE SKIP LOCKED;

-- name: LeaseSlugJob :one
UPDATE directory.slug_generation_jobs SET status='running',pause_code='',resume_after=NULL,lease_token=sqlc.arg(token)::text,
lease_until=now()+interval '90 seconds',revision=revision+1,updated_at=now(),items=sqlc.arg(items)::jsonb
WHERE id=sqlc.arg(id)::uuid RETURNING *;

-- name: FinishSlugJobClaim :one
UPDATE directory.slug_generation_jobs
SET status=sqlc.arg(status)::text,items=sqlc.arg(items)::jsonb,pause_code=sqlc.arg(pause_code)::text,
resume_after=sqlc.narg(resume_after)::timestamptz,revision=revision+1,updated_at=now(),
lease_until=CASE WHEN sqlc.arg(status)::text='running' THEN lease_until ELSE NULL END,
lease_token=CASE WHEN sqlc.arg(status)::text='running' THEN lease_token ELSE '' END
WHERE id=sqlc.arg(id)::uuid AND lease_token=sqlc.arg(token)::text AND status='running'
AND revision=sqlc.arg(expected_revision)::bigint AND lease_until>now()
RETURNING *;

-- name: SlugJobActorAuthorized :one
SELECT EXISTS(SELECT 1 FROM identity.users u
WHERE u.id=$1 AND u.access_status='ACTIVE' AND u.deleted_at IS NULL AND u.deletion_requested_at IS NULL
AND (u.role='SYS_ADMIN' OR (u.role='ADMIN' AND EXISTS(
SELECT 1 FROM identity.user_management_permissions p WHERE p.user_id=u.id AND p.permission_key='taxonomy.manage'))))::boolean;

-- name: LockSlugJobTag :one
SELECT id,name,slug,description,is_enabled,updated_at FROM directory.tag_dictionary WHERE id=$1 FOR UPDATE;

-- name: SetSlugJobTag :exec
UPDATE directory.tags SET slug=$2 WHERE id=$1;
