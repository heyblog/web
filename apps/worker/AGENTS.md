# Worker Module

Own scheduling polls, instance registration, leased execution, heartbeat,
per-hop API authorization, result delivery and bounded graceful shutdown.
Never import `apps/api/internal`, open a database pool, parse feeds, decide
ownership or sign proofs. Internal DTOs belong to `packages/go/taskprotocol`;
Edge safety and transport belong to `packages/go/edgerequest`.

`internal/config` alone reads runtime environment. Use `TASK_WORKER_API_URL`,
`API_TASK_WORKER_TOKEN`, `API_EDGE_CF_URL/TOKEN`, `API_EDGE_EO_URL/TOKEN`;
the signing master key belongs exclusively to the API. Every process gets a
fresh instance UUID. Protocol mismatch, failed authorization and unconfirmed
leases stop sending. An uncertain send is never blindly replayed.

`mise.toml` owns install, dev, check, test, build and verify. Tests use isolated
HTTP servers and the race detector. `--healthcheck` probes local readiness;
`/health/live` checks the process and `/health/ready` checks confirmed API
contact. The worker drains for at most 25 seconds on SIGTERM/SIGINT.
