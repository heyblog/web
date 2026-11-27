# Edge Module Guidance

All file references below are relative to `apps/edge` unless explicitly rooted at
another module. Inherit the repository-root `AGENTS.md`.

## Sources and Ownership

- `package.json` owns Hono/Zod and build dependencies; `mise.toml` owns module tasks.
  `tsconfig.json`, `eslint.config.ts` and the shared Node configuration own checks.
  Use the repository toolchain and lockfile; do not add an independent release version.
- `src/app.ts` owns the authenticated Hono HTTP boundary. `src/protocol.ts` owns Zod
  parsing, wire DTOs, adapter contracts and stable execution error codes.
- `src/target.ts` owns target/header validation; `src/execute.ts` owns one bounded
  fetch, response streaming and cleanup. Keep platform details in `src/cloudflare.ts`,
  `src/edgeone.ts` and `src/edgeone-fetch.ts`.
- `scripts/build.mjs` owns the two deployment bundles. `scripts/deploy-cf.mjs`,
  `scripts/deploy-eo.mjs` and `cloudflare.json` own deployment inputs and provider calls.
  `README.md` is the operator guide for setup and platform acceptance.
- The API owns authorization, proof signing, durable task state and business interpretation.
  `packages/go/edgerequest` owns DNS/public-target checks and transport for API and worker;
  the worker orchestrates scheduling, retries, fallback and redirects. Edge does not
  access the database, store private signing keys or decide site health/RSS validity.

## HTTP and Request Identity

- Preserve authenticated `POST /v1/execute`, a bounded JSON request and `no-store`
  responses. Authenticate before processing input. Validate untrusted JSON through
  `requestSchema`; do not cast it to an internal request type.
  The 256 KiB envelope cap includes Go's JSON escaping overhead and RSS validators;
  oversized bodies return 413 before fetch. It is separate from the target response byte limit.
- Coordinate wire changes with `packages/go/edgerequest`.
  Preserve GET/HEAD-only requests, platform matching, millisecond deadlines, byte
  limits, base64 response bodies, selected response headers and node metadata.
  Keep limits aligned with Go's configuration and serialization, including the
  additional proof header and UA suffix applied during signing.
- Request `headerEncoding: 'latin1'` maps raw Go HTTP bytes to Unicode code points
  0–255 for JSON transport; response headers always use that byte-string convention.
  Preserve this encoding so opaque ETags survive round trips. Direct text requests
  default to UTF-8. Never pass UTF-16 text directly to Fetch's ByteString headers or
  re-encode an already byte-preserving value.
- Service bearer credentials and outbound request proofs have separate purposes.
  The bearer authenticates the API to Edge; the proof attests the API's authorization
  of the target request. Never forward inbound service headers or cookies to targets.
- Construct outbound headers only from validated request data. Preserve the reserved
  header exclusions and the `X-HeyBlog-Proof` exception in `targetHeaders`; do not
  allow callers to override Host, connection framing, proxy or platform headers.
- The signed URL must equal the URL used by native Fetch. Preserve the serialized
  URL check in `safeTarget`: normalization of host, port or encoded dot segments
  must not silently attach a proof to another request. Any future normalization must
  happen before Go signs the request and remain consistent at every hop.
- Keep `redirect: 'manual'` and target cache-control headers.
  Return redirect responses to Go for validation and signing of the next hop; do not
  add autonomous redirects, retries, fallback or response caches.

## Bounded Execution and Failure Semantics

- Validate protocol, destination port and public destination addresses before fetch.
  Keep Go and Edge address restrictions consistent. A validated DNS answer is part
  of the trusted API request, but does not replace platform egress isolation.
- Bound the response while reading its stream, including decompressed bytes exposed
  by the runtime. Release readers and abort timers on success, rejection and failure.
  Keep the deadline active through body consumption, not only receipt of headers.
- Preserve target HTTP status separately from executor errors. HTTP errors are target
  responses; `unsafe_target`, `target_network`, `target_timeout`, `response_limit` and
  `platform` have distinct meanings consumed by Go. Do not expose raw exceptions,
  upstream bodies or credentials in error responses or diagnostics.
- Empty bodies are valid for HEAD and 304. Preserve validators and redirect Location
  where allowed; Edge does not parse RSS, HTML ownership proofs or verification files.

## Platform Adapters and Secrets

- Cloudflare uses a module-worker default export and native Fetch. `cloudflare.json`
  must retain `global_fetch_strictly_public`; private network bindings are prohibited.
  Keep `cache: 'no-store'` in the Cloudflare adapter. Workers plan defaults govern CPU.
  This isolation does not pin the outgoing socket to Go's DNS result.
- EdgeOne uses the fetch-event entry point. `src/edgeone-fetch.ts` supplies the
  documented `RequestInit.eo.resolveOverride` and native timeout settings alongside
  the shared abort deadline. Reject targets on the normalized ingress hostname before
  Fetch with `unsafe_target`, because CDN routing can ignore the override. Preserve
  the original URL/Host for TLS and HTTP identity.
- Cloudflare's `cf.colo` describes the serving node. Caller country and EdgeOne
  `request.eo.geo` are not serving-node evidence. EdgeOne currently returns an empty
  node object because its documented interface supplies no serving-node identifier.
  Do not invent geographic metadata or treat a signed platform claim as node proof.
- Runtime secrets enter through Cloudflare's binding or EdgeOne's encrypted environment variables.
  Deployment scripts validate their own environment inputs; runtime modules receive
  adapter values and must not read Node's environment. Private signing/encryption keys
  remain in the API. Never write service secrets into committed files, local bundles,
  logs, target headers or public responses.
- Preserve Cloudflare's module bundle and EdgeOne's IIFE bundle. Do not introduce Node
  runtime APIs into either executor. Deployment tasks update remote state and require
  explicit deployment authorization; build and verify remain local operations.

## Validation

Run from the repository root:

- `mise run //apps/edge:test`: shared handler and adapter boundary tests.
- `mise run //apps/edge:check`: formatting, strict TypeScript and ESLint.
- `mise run //apps/edge:build`: both bundles, without credentials or remote writes.
- `mise run //apps/edge:verify`: checks, tests and both builds.

Use existing Node test tooling in `tests/`. For behavior changes, exercise the real
Hono request boundary or fetch adapter at the narrowest relevant seam. Cover accepted
requests as well as rejection before fetch, request identity, response limits and
error classification. Protocol changes also require the affected Go checks and root
verification under the repository validation matrix.

Local tests and bundles do not establish provider deployment, mainland placement,
DNS-rebinding isolation or native timeout behavior. Use the controlled platform
acceptance scenarios in `README.md` when deployments are authorized, and report
unperformed provider checks explicitly. Do not replace a platform check with a mocked
Fetch result or enable schedules as part of deployment.
