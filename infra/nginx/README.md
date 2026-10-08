# HeyBlog Nginx deployment

`heyblog.conf` serves the Web application on `www.heyblog.net` and the service API on
`api.heyblog.net`. The API container binds to `127.0.0.1:10201`; only Nginx is public.

Before enabling the API virtual host:

1. Point `api.heyblog.net` through EdgeOne to the existing origin.
2. Install a certificate at `/etc/letsencrypt/live/api.heyblog.net/`.
3. Run `nginx -t` and reload Nginx gracefully.

`/internal/v1/data-import` is authorized by the application with an INTERNAL API key carrying
`data_import.write`. Nginx does not apply a source-IP allowlist. Restricting direct origin access to
trusted edge infrastructure is separate deployment hardening and is not part of this configuration.

Browser JavaScript may call only future explicitly anonymous read routes under `/v1/*`. Service API
keys are server-side credentials and must never be embedded in browser bundles. Keep API CORS
credentials disabled; preflight intentionally rejects `Authorization` and all methods except
`GET`/`HEAD`.

## Database JSON export and restore

The SYS_ADMIN management page `/management/database-backup` exports all business tables and the
complete logical AGE graph. The version 1 JSON format, schema version 2, covers 38 business tables,
including site claims, ownerships, ownership history and owner friend-link requests. It preserves business IDs, credential indexes,
timestamps and history. AGE internal IDs are rebuilt. Redis sessions and environment secrets are
not included. The source system administrator and their login/permission rows are excluded;
structured historical administrator references are mapped to the target system administrator.
The target administrator's account and login material remain unchanged.
Files from schema version 1 (34 tables) are rejected because they omit ownership data and new
audit fields. Export a new file with the current schema before restoring.

Restore requires a newly initialized database with exactly one active formal SYS_ADMIN and
unchanged migration seeds. Other business data prevents restoration. File inspection checks the
format and target readiness; restore rechecks these under database locks, validates SHA-256 and
commits all datasets and graph in one transaction. A failed restore rolls back completely. After
redeployment, users sign in again. Preserve authentication environment secrets if existing
verification/reset credentials must continue working.

The Web backup route streams uploads/downloads, accepts JSON files up to 512 MiB and allows
30-minute operations. Its Nginx location has a 513 MiB multipart limit and 31-minute deadlines.
Keep management routes inaccessible through the public API virtual host.

New database bootstrap grants TEMPORARY to `migrator`, required for the protected restore
transaction guard. Before applying the backup migrations to an existing database, its database
owner must grant TEMPORARY on that database to `migrator`. Do not grant this privilege to
`api_runtime`. This implementation does not perform deployment or export existing data.
