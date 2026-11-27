# Dual Edge executors

Build and verify from the repository root:

```sh
mise run //apps/edge:verify
```

The resulting `dist/cloudflare.mjs` and `dist/edgeone.js` accept authenticated
`POST /v1/execute` requests from the independent worker. They perform one GET or HEAD, return a
bounded base64 body and selected response headers, and never follow redirects.
The shared Go request library resolves and checks every hop; the API issues a fresh
proof and durably authorizes each dispatch. Edge responses separate target HTTP status from execution
errors. A valid 304 has an empty body and remains a successful transport result.

## Deployment

Deployment changes remote state. Run these tasks only for an explicitly authorized
deployment, with credentials supplied through the deployment environment:

1. For Cloudflare, set `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_API_TOKEN`,
   `CLOUDFLARE_WORKER_NAME`, and an `EDGE_SERVICE_TOKEN` of at least 32 characters.
   Run `mise run //apps/edge:deploy:cf` to upload the module and its secret binding.
   In the Worker dashboard, open **Settings → Domains & Routes** and add a
   Custom Domain, such as `edge-cf.example.com`, in an active zone you own.
   Choose a hostname without an existing CNAME. Cloudflare provisions the DNS
   record and TLS certificate. Once HTTPS works, use
   `https://edge-cf.example.com/v1/execute` as the API and worker URL.
   See [Cloudflare Custom Domains](https://developers.cloudflare.com/workers/configuration/routing/custom-domains/).
2. For EdgeOne, create an edge function in the intended zone and add an accelerated
   hostname, such as `edge-eo.example.com`. Complete its DNS access setup and HTTPS
   certificate configuration. Set a function trigger that matches that hostname and
   the exact path `/v1/execute`, pointing to the function. In the EdgeOne console,
   add an encrypted function environment variable named `EDGE_SERVICE_TOKEN`
   with at least 32 characters. Set `TENCENT_SECRET_ID`, `TENCENT_SECRET_KEY`,
   `EDGEONE_ZONE_ID`, and `EDGEONE_FUNCTION_ID` in the deployment environment and run
   `mise run //apps/edge:deploy:eo` to update the existing function. It uploads
   only the bundle. Rotate any token previously embedded in function source.
   Use `https://edge-eo.example.com/v1/execute` as the API and worker URL.
   See [EdgeOne environment variables](https://cloud.tencent.com/document/product/1552/109151).
   See [EdgeOne function trigger rules](https://cloud.tencent.com/document/product/1552/111380).

Use a different service token for each executor. Configure the API and worker with
each full URL and the corresponding token as shown below. Request-proof signing
keys remain exclusively in the API.

Deploy the matching Edge bundles before updating the API when the executor wire
protocol changes. The worker sends `headerEncoding: latin1` to preserve HTTP header
bytes, including Unicode UA text and opaque RSS validators; older strict executors
reject this field. Local verification does not deploy either service.

## Network isolation and acceptance

Cloudflare uses its [documented egress isolation](https://developers.cloudflare.com/workers/reference/security-model/)
and `global_fetch_strictly_public`, with no private network bindings. This does not
pin a socket to Go's DNS answer. EdgeOne uses `resolveOverride` with the validated
public address through the documented [`RequestInit.eo.resolveOverride`](https://edgeone.ai/document/52690#RequestInitEoProperties)
extension. Fetches to the executor's own hostname can enter EdgeOne CDN routing,
where the override has no effect, so the adapter rejects them before Fetch.
See [EdgeOne Fetch](https://edgeone.ai/document/52687). EdgeOne's native timeouts
supplement the shared abort deadline. Both adapters set outbound no-cache headers,
preserve manual redirects and bound response size and duration. Cloudflare also
sets the Fetch `cache: 'no-store'` option. The Go
caller checks DNS and every redirect; neither adapter treats a probe error or quota
failure as evidence that the target is down.

Local tests cover the shared handler and Go wire contract. They do not establish
provider deployment, geographic placement, DNS-rebinding behavior, or the actual
platform implementation of isolation and timeouts. Before enabling schedules,
perform authenticated provider requests against controlled public redirect,
private-target, maximum-size binary response, oversized-response and slow-response
fixtures. Inspect request counts and node metadata in the task records. Confirm the
EdgeOne same-host request returns `unsafe_target`, and use repeated requests to a
controlled public origin to verify cache and timeout behavior. Measure Cloudflare
CPU usage with the largest supported response on the intended Workers plan.
Workers Free has a 10 ms CPU limit.
See [Cloudflare Workers limits](https://developers.cloudflare.com/workers/platform/limits/#cpu-time).

Cloudflare records its serving `colo`. Its country field and EdgeOne's `request.eo.geo`
describe the caller, so they are not reported as serving-node location. EdgeOne currently
returns an empty node object because the documented request interface does not expose
a serving-node identifier. To use mainland China nodes, choose an acceleration
region that includes mainland China and complete the site's required
[ICP filing](https://cloud.tencent.com/document/product/1552/111007). Use provider
evidence to confirm where requests execute. The region match in a function trigger
describes the caller's IP region.

## API and worker setup

Run `mise run build` for the API, standalone worker, Web application and both Edge bundles.
The API and worker executables are `apps/api/dist/heyblog-api` and
`apps/worker/dist/heyblog-worker`. The worker calls the API directly on the private
service network; the public reverse proxy does not expose its internal routes.

Supply `API_TASK_WORKER_TOKEN` (at least 32 non-whitespace characters) to both services.
Supply `TASK_WORKER_API_URL` (API origin, without a path) to the worker.
Supply these executor settings to the API and worker:

- Set `API_EDGE_CF_URL` and `API_EDGE_EO_URL` to the full HTTPS `/v1/execute` endpoints.
- Set `API_EDGE_CF_TOKEN` to the Cloudflare service token and `API_EDGE_EO_TOKEN` to the
  EdgeOne service token.
- API-only `API_REQUEST_SIGNING_MASTER_KEY`: base64-encoded 32-byte encryption key, stored separately
  from database backups. Losing this key prevents decryption of stored private signing keys.

Start the worker under a process supervisor after the API is ready. It checks schedules
every five seconds, renews leases while executing and stops on SIGINT/SIGTERM. Multiple
workers share API-owned transactional dispatch and lease fencing.

As SYS_ADMIN, generate and publish one signing key per platform before activating it.
The hourly monitoring/RSS templates start disabled in Asia/Shanghai.
Configure request settings and enable schedules only after platform acceptance. Key rotation
publishes a new key before activation; revoked keys cannot sign queued work. A restored
database disables schedules and request configurations, cancels unfinished tasks and revokes every restored signing key;
generate and activate fresh keys before resuming dispatch.

Request statistics record authorization, reported sends, results and unknown delivery,
including redirects, retries, fallback and HTTP verification. Platform rate-limit responses
trigger shared backoff; target HTTP 429 does not pause a whole platform. These
counts do not replace provider billing reports. The public verification page distinguishes valid, expired, revoked and missing
proofs; proof platform claims attest HeyBlog authorization, not physical node location.
See [worker operations](../worker/README.md) for protocol upgrades, draining and recovery.

Statistics count each request hop for `authorized`, `sent`, `received`, `succeeded`,
`failed` and `unknown`; `duration_ms` sums hop durations. `retries` counts distinct
`(target_id, platform, logical_retry)` values greater than zero, `redirects` counts
hops greater than zero, and `fallbacks` counts targets using an alternate platform.
These are observed system requests and do not represent provider billing or quotas.

Before enabling production traffic, verify both META and FILE ownership checks
create a verification run, target and attempt before a worker result can establish
ownership. DNS TXT checks must complete through the API resolver without an Edge
endpoint or worker. For monitoring, an authorization alone must leave sampling
unstarted; the first confirmed send starts the five-minute window. A second
platform, retry, redirect or lease recovery must preserve that time and round ID.

Run the API integration task (`mise run //apps/api:test:integration`) with working
Docker/testcontainers and available ports. Local tests cover backup normalization,
first-send sampling, worker-only HTTP verification, and retry/redirect/fallback
aggregation. Provider acceptance uses the controlled public fixtures described
above on both deployed platforms.
