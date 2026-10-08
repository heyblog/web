-- name: BeginDatabaseRestore :exec
SELECT directory.backup_restore_begin(sqlc.arg(admin_id)::uuid);

-- name: RestoreDatabaseRow :exec
SELECT directory.backup_restore_row(sqlc.arg(dataset)::text,sqlc.arg(row_data)::jsonb,sqlc.arg(source_admin_id)::uuid,sqlc.arg(target_admin_id)::uuid);

-- name: RestoreDatabaseGraphRow :exec
SELECT directory.backup_restore_graph_row(sqlc.arg(kind)::text,sqlc.arg(row_data)::jsonb);

-- name: FinishDatabaseRestore :exec
SELECT directory.backup_restore_finish();
