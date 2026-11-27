# Worker operations

The worker schedules and executes network tasks through the API's private
`/api/internal/v1/tasks` protocol version 2. Only the API connects to PostgreSQL,
parses feeds, verifies ownership proofs and signs requests. The worker receives
short-lived per-hop permits, never signing private keys.

Build with `mise run //apps/worker:build`, verify with
`mise run //apps/worker:verify`, or start development with
`mise run //apps/worker:dev`. The binary is `dist/heyblog-worker`.
Production Compose includes a separate worker service with a 30-second stop grace.

Supply `TASK_WORKER_API_URL` and the same dedicated `API_TASK_WORKER_TOKEN` as the
API. Supply the existing `API_EDGE_CF_URL/TOKEN` and `API_EDGE_EO_URL/TOKEN` pairs
to each process that executes Edge requests. Keep
`API_REQUEST_SIGNING_MASTER_KEY` exclusively in the API environment and separate
from database backups. Configuration examples belong to the root scenario templates.

The process polls every five seconds and executes at most 16 targets concurrently.
Leases last 120 seconds and renew every 20 seconds. Each process registers a fresh
identity. API loss prevents new sends; unconfirmed delivery remains unknown.
SIGINT/SIGTERM stops scheduling and acquisition, drains for up to 25 seconds,
then cancels remaining work for lease recovery. `/health/live` checks the process;
`/health/ready` requires recent API contact. The `--healthcheck` flag probes readiness.

Migrations 17–19 extend the deployed database with monitoring, the task center,
and schema-seven backups. Monitoring and RSS schedules start disabled.
Protocol mismatch prevents worker registration and acquisition.

In the task center, prepare signed request configurations, publish and activate
one key per platform, then enable schedules after [Edge acceptance](../edge/README.md).
Pause drains acquired targets; cancel preserves committed results. Manual retries
create linked runs with the original ordinary parameter snapshot. Archive terminal
runs before a system administrator cleans their details. Cumulative request facts,
monitoring samples, feed content and audit identity survive cleanup.

Schema-seven restores cancel all unfinished tasks, clear leases and runtime
backoff, disable schedules/configurations, reset monitoring generations, force
signing on and revoke restored keys. Generate and activate fresh keys and confirm
configurations before re-enabling execution. No history is automatically deleted.
