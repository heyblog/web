# API Module Guidance

All file references are repository-root-relative. This file refines `AGENTS.md` for `apps/api`.

## Sources and Service Boundary

- `apps/api/go.mod`, `apps/api/go.sum`, and `apps/api/mise.toml` own API dependencies and tasks;
  root `mise.toml` owns managed tool versions and the project release version.
- API-specific CLI tools belong to the module's Go `tool` block. Repository-wide Go tools belong
  to root `go.mod`. Do not require separate global installations or independent release versions.
- Own domain rules, authoritative authentication/authorization, HTTP contracts, migrations,
  database access, and pool lifecycle. Other applications communicate over HTTP and do not create
  database pools. Return DTOs rather than database rows.

This is the module's sole instruction file. Package-specific behavior is grounded in the source
and focused tests referenced below, not additional package-level AGENTS files.

## Actual Package Structure

- `apps/api/main.go` is the minimal composition entry point; keep other Go implementation and
  tests in cohesive packages under `apps/api/internal`.
- `apps/api/internal/bootstrap` composes validated configuration, logging, shared dependencies,
  migrations, server startup, and shutdown.
- `apps/api/internal/features` groups complete business capabilities: `auth`, `apikey`, `announcement`, `siteaudit`,
  `dataimport`, `databasebackup`, `publicview`, `sitemanagement`, `sitestats`, `taxonomy`, and `exampleapi`. Preserve their feature boundaries and colocated
  operations, repositories, and tests.
- `apps/api/internal/platform` groups shared application mechanisms: `httpapi`, `apperror`,
  `ratelimit`, `config`, and `logging`. `platform/httpapi` owns Gin/Huma routing, middleware,
  endpoint adapters and Problem Details. Business route registration belongs to the owning feature.
- `apps/api/internal/features/publicview` owns public-view HTTP routes and assembles read-side DTOs. Its `Queries` interface is a
  composition-time contract; helpers receive only their consumer-specific query interfaces.
  Retain the `New` composition entry point and sqlc query types without pure-forwarding stores.
- `features/sitestats` owns independent cumulative counters and atomic click/impression idempotency.
  Publicview receives its recorder explicitly: primary display queries record retrievals before DTO
  assembly; successful views record responses. Auxiliary, management, import and outbound-target
  reads are excluded. Image generation uses the non-counted /api/web/public/sites/id/{identifier}/metadata read. Never update directory.sites to record metrics. Outbound resolution remains
  usable when metric writes or click limits fail; removed sites remain unavailable.
- `apps/api/internal/features/siteaudit` keeps permission, revision, merge, and decision flow in
  services. `Store` returns business types; `AuditTransaction` binds business operations to one
  transaction. SQL, pgtype conversions, row mapping, canonical writes, and transaction lifecycle
  belong in `repository*.go`. Services must not access `Repository.queries` or sqlc types.
  Check audit status and draft revision before decoding locked snapshots; notify only after commit.
- `apps/api/internal/domain` owns framework-independent domain values and invariants.
- `apps/api/internal/infrastructure` groups external-resource implementations: `database`, `cache`,
  `mail` and `siteicon`. `database` owns migrations, sqlc sources/generated queries, and pool access.
- `apps/api/internal/integration` owns cross-feature tests with real isolated infrastructure.
- `features`, `platform`, and `infrastructure` are directory namespaces, not aggregate Go packages.
  Bootstrap composes their concrete implementations; domain code stays independent of them.
- Domain code must not import handlers or database implementations. Split a feature package by
  responsibility when needed; do not add pure-forwarding repositories or generic layers without
  an actual ownership need.

## HTTP and Authorization

- Route families are `/api/web/{public,auth,submissions,account,management}`,
  `/api/external/v1` and `/api/internal/v1`. Public-view handlers live in `features/publicview`;
  bootstrap explicitly registers every feature. Health and development OpenAPI stay separate.
  No legacy route aliases are retained. Nginx exposes only the exact machine operations.

- Define endpoint audience, method/path, request/response DTOs, status codes, permissions, rate
  limits, cancellation, and timeouts together. Keep existing contracts unchanged in refactors.
- Gin hosts routing/middleware; business endpoints use Huma typed operations. Development-only
  OpenAPI/Swagger behavior is defined by `apps/api/internal/platform/httpapi/openapi.go`.
- Transport validates inputs, calls an operation, and maps typed results/errors. Gin `Adapt` and
  `errorBoundary`, plus Huma `Register`/`HTTPError`, in `apps/api/internal/platform/httpapi` own
  failure responses; do not emit a second Problem Details response or continue after failed middleware.
- Preserve typed expected failures, explicit error returns, and request contexts across database,
  cache, and outbound calls. Do not panic for validation, authorization, or dependency failures.
- Internal networks and client-IP metadata are not authorization. Keep Web-token, bearer health,
  user-session, and scoped API-key guards distinct. Trust forwarded IPs only under the configured
  production proxy boundary defined by `infra/nginx/heyblog.conf` and
  `infra/docker/docker-compose.production.yaml`.
- Never expose SQL details, internal addresses, credentials, or stack traces. Log a propagated
  failure once at its owning reporting boundary.
- Web-facing reads, authentication, and anonymous submissions retain their Web-token guards and
  route-specific limits. Health uses a separate bearer token and no-store responses; liveness
  does not probe dependencies. Audience and health contracts live in
  `apps/api/internal/platform/httpapi/router.go` and `apps/api/internal/platform/httpapi/router_test.go`.
- Browser auth preserves password/email verification, rotating refresh sessions, HttpOnly and
  SameSite cookie delivery, and operation-specific `auth_version` invalidation. GitHub login uses
  a verified primary email; binding must match the account and unbinding must leave password login.
  OAuth-created users confirm their account profile before authenticated business operations.
  Initial confirmation preserves the separate one-time username change allowance; display names remain editable.
  `CurrentSession` serves session/profile confirmation and `Current` additionally requires completed confirmation.
  Profile mutations serialize on the account row and rotate the current session after username changes.
  User management uses server pagination and timestamp revision checks; password setup/reset uses email links.
  User DTOs expose `deletion_pending`; scheduled-deletion accounts remain readable and cannot be managed
  until the existing deletion lifecycle permits an account-state transition.
  Preserve user-management role, permission, scope, and self-management restrictions in
  `apps/api/internal/features/auth/management.go`, `apps/api/internal/features/auth/github.go`, and
  `apps/api/internal/features/auth/repository_authorization.go`.
- Machine-credential management requires SYS_ADMIN. Never log or persist presented secrets;
  issuance responses are the only secret-delivery opportunity. Keep audience/scope/expiry policy,
  active-key conflicts, client-row serialization, and revoked/expired history coherent across
  `apps/api/internal/features/auth/routes_api_keys.go`, `apps/api/internal/features/apikey/issue.go`,
  `apps/api/internal/features/apikey/repository.go`, and `apps/api/internal/features/apikey/policy_test.go`.
  Direct scoped endpoints retain their method/response contracts in
  `apps/api/internal/features/exampleapi/route_test.go`; do not bypass scope or audience checks.
- Submission lookup credentials support repeated queries; persist only their digest and treat
  invalid credentials as not found. Preserve reviewer role/permission checks and additional
  taxonomy authorization. Original submissions are frozen, but pending reviewer corrections are
  editable with revision checks. Accepted snapshots apply atomically to canonical directory data;
  audit JSON remains history. Sources: `apps/api/internal/features/siteaudit/submission.go`,
  `apps/api/internal/features/siteaudit/review_draft.go`, and
  `apps/api/internal/features/siteaudit/repository_taxonomy.go`.
- Authenticated site control belongs to `features/sitemanagement`; authenticated directory editing
  and submission provenance belong to the account operations in `features/siteaudit`. Site claims
  bind applicant, site, exact canonical address and one-time proof; one ownership generation grants
  editing authority. Owner updates require approval and address changes consume a matching verified
  proof in the same transaction. Existing registered friend links apply directly through graph
  wrappers; new targets share CREATE audits and deferred recommendations are applied only while
  their captured ownership generation is still valid. Manual certification and ownership revocation use site-review authorization, with self-review and
  self-revocation forbidden. Ownership changes begin with applicant verification; management has no
  direct reassignment operation. Historical reassignment events remain readable. Duplicate pending
  claims reuse the existing application without reissuing its proof. Explicit regeneration changes
  only the stored digest and expiry, and verification decisions compare that digest under the claim
  lock after network verification. Automatic proof checks share an eight-second deadline. Revocation
  compares the ownership identifier and revision and invalidates pending claims and unused verified
  proofs in the same transaction. Account claim context and paginated claim lists use read-only,
  batched queries; applicant profiles are returned only to authorized reviewers. Never persist raw verification tokens,
  fetch private network addresses, or expose another submitter's contact through a shared target.
- Random reads select VISIBLE sites, including warned sites, using confirmed classification
  names with case-insensitive matching. Validate before selection, retain a normal null result when no candidate
  exists, and keep preview Web-only. Preserve operation-specific errors/cache policy rather than
  mapping every database failure to unavailable; see
  `apps/api/internal/features/publicview/errors.go` and
  `apps/api/internal/features/publicview/routes_random_test.go`.
- Sitemap reads at `/api/web/public/sitemap` retain the Web-token guard and public-view rate policy. Accept only
  `kind=sites|announcements` and an optional UUID cursor `after`; paginate by ascending UUID with
  at most 1,000 items. Include VISIBLE sites and the same announcements as public detail, including
  expired announcements and archives created after their start. API owns visibility and real
  publication/update timestamps; Web owns canonical URLs, XML, sharding, and caching. Sources:
  `apps/api/internal/features/publicview/sitemap.go`,
  `apps/api/internal/infrastructure/database/queries/sitemap.sql`, and
  `apps/api/internal/features/publicview/routes_sitemap.go`.

## Configuration and External Services

- `features/taxonomy` owns tags with one display name per identifier. `directory.tags` enforces
  globally unique `lower(btrim(name))` values; casing is retained for display. Different names
  remain independent tags. Roles belong to paths and object associations; a path may use the same
  tag in both roles. Audit, import, and management writes share the taxonomy transaction advisory
  lock. Existing disabled references remain readable. The fresh-install baseline defines independent names and retains typed TAG/LABEL aliases
  for immutable historical snapshots. New requests and responses use tag IDs and names only.
  The consolidated baseline is forward-only. Existing unmarked databases must be exported with
  the previous application and restored into a fresh database; retain the original database and
  application for rollback.

- `apps/api/internal/platform/config` alone discovers YAML/environment inputs and exports typed runtime
  configuration. Inject it into other packages. Secrets and service bindings stay in environment
  inputs; non-sensitive policy belongs in YAML. Preserve executable-relative discovery/fallback,
  strict decoding, mode/default handling and production validation in
  `apps/api/internal/platform/config/loader.go` and `apps/api/internal/platform/config/validation_test.go`.
  Loading must not create or rewrite configuration files; OAuth callbacks use the Web origin.
- Mail delivery uses the typed Sender/Message boundary and purpose-specific templates. Preserve
  address/subject/body validation, cancellation, timeouts and delivery-unavailable errors in
  `apps/api/internal/infrastructure/mail/message.go`. SMTP/SES selection belongs to
  `apps/api/internal/platform/config/mail_config.go`, not callers. Tests use injected clients or isolated
  Mailpit, never live credentials or real mail delivery.
- Cached icons retain bounded decoding, ICO safety, aspect ratio, alpha and no upscaling; emit PNG
  and expose only the profile hash, not bytes. Formats/limits are defined in
  `apps/api/internal/infrastructure/siteicon/normalize.go` and `apps/api/internal/infrastructure/siteicon/ico.go`.
  Preserve cached reads, digest ETags, no-store and safe failure mapping in
  `apps/api/internal/features/publicview/routes_icon_test.go`.

## Data Access and Lifecycle

- `features/databasebackup` owns versioned full-database JSON transfer at
  `/api/web/management/database-backup/{export,inspect,restore}`. All operations require both the Web token
  and an active SYS_ADMIN session. The 512 MiB file limit and 30-minute deadline apply only to
  these operations. Export and inspection remain request-bound. Restore uploads reserve the sole
  process-local slot before reading, as does inspection. Upload errors retain their causes:
  interrupted transfers and temporary-file I/O failures use the existing unavailable response,
  malformed fields remain validation failures and size limits remain 413. Safe diagnostics carry
  stage and received byte count; upload/inspection stage events use the existing request ID and
  structured logger. Never log backup contents, filenames, temporary paths or raw I/O errors.
  Accepted restores return 202 and execute independently of
  the request. `/restore-jobs/current`, `/restore-jobs/{id}`, and `/restore-jobs/{id}/cancel`
  expose the latest in-memory task with the same authorization. No task survives API restart.
  Explicit cancellation reports completion only after rollback; uncertain commit or rollback
  outcomes remain unknown. Cancellation cannot interrupt a commit already started. Private
  temporary files are removed after request completion or the accepted restore terminates.
  Bootstrap cancels and joins restore workers before closing the database pool.
- Backup protocol version 1, schema version 5 covers all 33 identity/directory/content tables and complete AGE
  logical vertices and edges. The baseline freezes the schema inventory; schema evolution
  requires an explicit backup compatibility change. Schema versions 3, 4, and 5 are accepted; schema 3 is converted after original integrity verification. Binary columns use Base64 and bigint columns
  use decimal strings. Never decode row JSON through floating-point numbers.
- Restore preserves business identifiers and data, excludes source SYS_ADMIN authentication,
  and maps structured actor references to the retained target administrator. It accepts only
  untouched initialization seeds plus that administrator's authentication records. Migration 13 owns narrow SECURITY DEFINER entry points and a protected transaction-local guard; runtime
  never receives direct graph privileges or unrestricted restore privileges. Migrator requires
  database TEMPORARY permission for transaction-local guards. Graph and all table writes are
  locked, verified and committed atomically; ordinary database constraints remain active.

- Internal data import accepts paired version-3 `blogs`/`graph` bundles. The default `initial`
  mode requires an empty directory. Explicit multipart `mode=incremental` accepts `nodes`/`edges`
  provenance, preserves registered profiles and existing edges, and inserts missing sites,
  new-site feeds, provenance, and edges in one transaction. Incremental input sites must be
  VISIBLE and have no resources, taxonomy, or components. Its all-status edge reader is a
  runtime-granted database function; do not reconcile through visibility-filtered public reads.

- The migration session lock checks `migration.baseline` before changing Goose history. Only a fresh database or the marked current baseline may initialize or resume. The clean-seeds baseline uses versions 1–14, followed by additive migrations; databases initialized with previous numbering or seeds require export and restore into a fresh database. Migration 14 installs cross-feature foreign keys and logical initialization seeds after their dependencies exist.
- Fresh seeds contain 71 tags, 142 classification paths and 81 TAG import identities with only `id` and `name` in their snapshots. LABEL mappings are created only for real historical data, not synthesized on initialization. Preserve import system keys, merged-tag UUID resolution and historical LABEL lookup. Schema-version-3 backups retain their original historical JSON, including obsolete fields; the clean seed contract does not restrict restored snapshots.
- Apply Goose migrations before opening runtime PostgreSQL/Redis connections. Initialize one
  application pool, inject it, and close it during graceful shutdown; runtime connections disable
  PostgreSQL JIT to avoid compilation stalls on parameterized OLTP queries; never create per-request or
  per-feature pools.
- Keep SQL behind data-access boundaries, not handlers or application orchestration. The owning
  operation defines the complete write transaction and failure/rollback lifecycle.
- Preserve the pgx/Goose/sqlc stack and database constraints. Do not edit generated sqlc files;
  change sources and use the declared generation/vet/diff tasks when a schema change is requested.
- Keep each cache's initialization, invalidation, and shutdown lifecycle explicit. Other modules
  must not bypass the API's data ownership.
- Global graph snapshots, encoded JSON and gzip bytes are cached by the committed revision in
  `runtime_cache.public_graph_revision`. Site/SQL graph triggers and the Cypher mutation/restore
  wrappers invalidate atomically (AGE bypasses ordinary triggers); failed refreshes never serve stale snapshots. This derived
  schema is excluded from the 33 business backup tables and starts fresh on rebuild.
- Taxonomy list catalogs are cached by `runtime_cache.taxonomy_revision`; source-table triggers invalidate all catalog/revision inputs in the writing transaction. Preview and mutations still load the full graph under the existing taxonomy advisory lock. Cached lists retain that lock and return independent DTO slices.
- Public directory tag counts and classification rows reuse the same committed taxonomy generation.
  Only refreshes serialize; each response builds independent DTOs and reads software counts afresh.
- Graph displays combine query/response increments into synchronous weighted batches. One coordinator
  serializes batches, each with a one-second database deadline and at most eight disjoint partition writes; accepted calls wait for persistence
  even after cancellation. Bootstrap drains the recorder before closing its pool. Ordinary metric
  write failures remain best-effort and do not prevent public reads. Counter-table fillfactor and
  autovacuum settings support repeated HOT updates without changing profile timestamps.
- Bootstrap separates cluster administration, non-superuser migrator ownership and explicit
  api_runtime grants. AGE installation/preloading, roles and migration schema are prerequisites;
  Goose migrations must not require superuser or role-management privileges. Sources:
  `infra/docker/initdb` and `apps/api/internal/infrastructure/database/migrations/sql/00002_age_runtime.sql`.
- Keep identity/directory/content ownership and migration-owned constraints/comments. Site friend
  links use authoritative directory_graph through typed directory wrappers; triggers synchronize
  SiteRef. Do not add relational mirrors, Go dual writes or deleted legacy schemas. Graph sources:
  `apps/api/internal/infrastructure/database/migrations/sql/00005_directory_graph.sql` and
  `apps/api/internal/infrastructure/database/migrations/sql/00014_initialization.sql`.
- Document every migration-owned schema, table, column, function and trigger with matching
  PostgreSQL comments. Document AGE properties at their typed wrapper functions.
- Preserve account deletion scheduling/cancellation, terminal anonymization, OAuth removal and
  authentication-version monotonicity in `apps/api/internal/infrastructure/database/migrations/sql/00003_identity.sql`
  and `apps/api/internal/infrastructure/database/migrations/sql/00007_authentication.sql`. Preserve site visibility
  lifecycle, address/base-path and component/tag cycle invariants in
  `apps/api/internal/infrastructure/database/migrations/sql/00004_directory.sql` and
  `apps/api/internal/domain/site/address.go`; do not replace visibility transitions with hard deletion.
- Announcement publication windows, allowed transitions, revision history, actor deletion and
  draft-only physical deletion remain database-enforced in
  `apps/api/internal/infrastructure/database/migrations/sql/00006_content_announcements.sql`.
- Internal imports retain the INTERNAL/data_import.write guard, bounded input/deadline, strict
  JSON/duplicate-key rejection, validation/error ordering, mapping order and random-ID calls.
  Blogs plus graph import requires one transaction, advisory lock, empty-directory and lock-capacity
  checks; taxonomy replacement is atomic but uses a narrower scope without those lock checks.
  Sources: `apps/api/internal/features/dataimport/route.go`, `apps/api/internal/features/dataimport`,
  `apps/api/internal/features/dataimport/repository.go`, and `apps/api/internal/features/dataimport/import_test.go`.

## Validation

Use commands from `apps/api/mise.toml`, invoked from the repository root:

- `mise run //apps/api:test`: ordinary Go tests; use focused package tests while iterating.
- `mise run //apps/api:verify`: sqlc checks, formatting, lint, race-enabled tests, and build.
- `mise run //apps/api:test:integration`: isolated PostgreSQL/AGE, Redis, and Mailpit infrastructure
  plus import transaction tests. Testcontainers owns created resources and cleanup; do not disturb
  developer-owned Compose services.
- `mise run //apps/api:security`: network-backed vulnerability checks when required by task scope.

HTTP tests cover guards, methods, status, headers, and DTOs. Domain/application tests stay narrow;
database semantics require isolated integration tests. Preserve integration runner order and
cleanup during test-file splits. Consult the affected package's source contracts and focused tests;
report any required gate that could not run instead of treating build-tag compilation as a real
container integration pass.

Database backup exports use format version 1 and schema version 5. Schema version 3 inputs are
verified against their original manifest before shared inspect/restore conversion initializes account
profile confirmation from OAuth bindings and grants one optional username change. New backups
preserve confirmation and usage state. The consolidated baseline owns these identity fields.

OAuth authentication shares provider adapters, one-use Redis state and account mutations for
GitHub and BlogsClub. BlogsClub credentials use `API_BLOGSCLUB_CLIENT_ID` and
`API_BLOGSCLUB_CLIENT_SECRET`. Its public callback is `/auth/blogsclub/callback`; GitHub retains
`/api/web/auth/github/callback`. JSON OAuth start/callback responses support the Web popup bridge.
BlogsClub site claims require fresh server-fetched verified profiles and exact normalized addresses,
with no stored access tokens or challenge regeneration.
OAuth callback completion separates the public result from server-only failures. Routes report
those failures once through `httpapi.LogError` while preserving JSON/popup or redirect delivery.
BlogsClub upstream failures, malformed responses and identity mismatches map to stable business
error codes; raw upstream bodies, URLs, authorization codes and tokens never enter diagnostics or
user-facing responses.
