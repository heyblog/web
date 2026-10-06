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
  `dataimport`, `publicview`, `taxonomy`, `sluggeneration`, and `exampleapi`. Preserve their feature boundaries and colocated
  operations, repositories, and tests.
- `apps/api/internal/platform` groups shared application mechanisms: `httpapi`, `apperror`,
  `ratelimit`, `config`, and `logging`. `platform/httpapi` owns Gin/Huma routing, middleware,
  endpoint adapters, Problem Details, and the existing public-view HTTP adapters.
- `apps/api/internal/features/publicview` assembles read-side DTOs. Its `Queries` interface is a
  composition-time contract; helpers receive only their consumer-specific query interfaces.
  Retain the `New` composition entry point and sqlc query types without pure-forwarding stores.
- `apps/api/internal/features/siteaudit` keeps permission, revision, merge, and decision flow in
  services. `Store` returns business types; `AuditTransaction` binds business operations to one
  transaction. SQL, pgtype conversions, row mapping, canonical writes, and transaction lifecycle
  belong in `repository*.go`. Services must not access `Repository.queries` or sqlc types.
  Check audit status and draft revision before decoding locked snapshots; notify only after commit.
- `apps/api/internal/domain` owns framework-independent domain values and invariants.
- `apps/api/internal/infrastructure` groups external-resource implementations: `database`, `cache`,
  `mail`, `tokenhub`, and `siteicon`. `database` owns migrations, sqlc sources/generated queries, and pool access.
- `apps/api/internal/integration` owns cross-feature tests with real isolated infrastructure.
- `features`, `platform`, and `infrastructure` are directory namespaces, not aggregate Go packages.
  Bootstrap composes their concrete implementations; domain code stays independent of them.
- Domain code must not import handlers or database implementations. Split a feature package by
  responsibility when needed; do not add pure-forwarding repositories or generic layers without
  an actual ownership need.

## HTTP and Authorization

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
- Random reads select VISIBLE sites, including warned sites, using exact canonical classification
  names rather than slugs. Validate before selection, retain a normal null result when no candidate
  exists, and keep preview Web-only. Preserve operation-specific errors/cache policy rather than
  mapping every database failure to unavailable; see
  `apps/api/internal/features/publicview/errors.go` and
  `apps/api/internal/platform/httpapi/public_view_random_test.go`.
- Sitemap reads at `/sitemap` retain the Web-token guard and public-view rate policy. Accept only
  `kind=sites|announcements` and an optional UUID cursor `after`; paginate by ascending UUID with
  at most 1,000 items. Include VISIBLE sites and the same announcements as public detail, including
  expired announcements and archives created after their start. API owns visibility and real
  publication/update timestamps; Web owns canonical URLs, XML, sharding, and caching. Sources:
  `apps/api/internal/features/publicview/sitemap.go`,
  `apps/api/internal/infrastructure/database/queries/sitemap.sql`, and
  `apps/api/internal/platform/httpapi/public_view_sitemap.go`.

## Configuration and External Services

- `features/taxonomy` owns a globally unique tag dictionary, shared SITE/ARTICLE path changes,
  preview fingerprints, and historical ID/system-key/slug aliases. Roles belong to paths and object
  associations; a path may use the same tag in both roles. Audit, import, and management writes share
  the taxonomy transaction advisory lock. Existing disabled references remain readable. Migration 19
  is forward-only: restore a pre-upgrade backup to undo dictionary consolidation.
- `features/sluggeneration` owns authorized slug generation and global model settings. Only SYS_ADMIN
  changes the model. `infrastructure/tokenhub` uses the official OpenAI Go SDK's Chat Completions with
  retries and thinking disabled; provider URLs and policy come from `config/default.yaml#ai`.
  `API_TOKENHUB_API_KEY` is an optional API-only environment secret. Never persist it or expose it to Web.
  Redis enforces request limits, paid daily budgets, concurrency, and generation deduplication; failures
  fail closed. Test with injected clients or isolated infrastructure, never live provider credentials.
  Durable batch previews live in `directory.slug_generation_jobs`; bootstrap owns the worker and
  stops it before closing shared pools. Workers recheck actor permissions and persist dispatch
  state before provider I/O. Rate/concurrency waits resume automatically; daily-budget pauses require
  an explicit resume. YAML `ai.batch` bounds request size, task size, and output tokens. Audit
  preparation generates at most twenty new tag slugs in two provider calls outside transactions,
  with a 35s total deadline and final revision/canonical checks inside the write transaction.
  Migrations 19 and 20 refuse destructive downgrade; restore a pre-upgrade backup for production
  rollback. Legacy migrations retain isolated rollback coverage.

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
  `apps/api/internal/platform/httpapi/public_view_icon_test.go`.

## Data Access and Lifecycle

- Internal data import accepts paired version-3 `blogs`/`graph` bundles. The default `initial`
  mode requires an empty directory. Explicit multipart `mode=incremental` accepts `nodes`/`edges`
  provenance, preserves registered profiles and existing edges, and inserts missing sites,
  new-site feeds, provenance, and edges in one transaction. Incremental input sites must be
  VISIBLE and have no resources, taxonomy, or components. Its all-status edge reader is a
  runtime-granted database function; do not reconcile through visibility-filtered public reads.

- Apply Goose migrations before opening runtime PostgreSQL/Redis connections. Initialize one
  application pool, inject it, and close it during graceful shutdown; never create per-request or
  per-feature pools.
- Keep SQL behind data-access boundaries, not handlers or application orchestration. The owning
  operation defines the complete write transaction and failure/rollback lifecycle.
- Preserve the pgx/Goose/sqlc stack and database constraints. Do not edit generated sqlc files;
  change sources and use the declared generation/vet/diff tasks when a schema change is requested.
- Keep each cache's initialization, invalidation, and shutdown lifecycle explicit. Other modules
  must not bypass the API's data ownership.
- Bootstrap separates cluster administration, non-superuser migrator ownership and explicit
  api_runtime grants. AGE installation/preloading, roles and migration schema are prerequisites;
  Goose migrations must not require superuser or role-management privileges. Sources:
  `infra/docker/initdb` and `apps/api/internal/infrastructure/database/migrations/sql/00002_age_runtime.sql`.
- Keep identity/directory/content ownership and migration-owned constraints/comments. Site friend
  links use authoritative directory_graph through typed directory wrappers; triggers synchronize
  SiteRef. Do not add relational mirrors, Go dual writes or deleted legacy schemas. Graph sources:
  `apps/api/internal/infrastructure/database/migrations/sql/00005_directory_graph.sql` and
  `apps/api/internal/infrastructure/database/migrations/sql/00008_directory_registered_friend_links.sql`.
- Document every migration-owned schema, table, column, function and trigger with matching
  PostgreSQL comments. Document AGE properties at their typed wrapper functions.
- Preserve account deletion scheduling/cancellation, terminal anonymization, OAuth removal and
  authentication-version monotonicity in `apps/api/internal/infrastructure/database/migrations/sql/00003_identity.sql`
  and `apps/api/internal/infrastructure/database/migrations/sql/00009_authentication.sql`. Preserve site visibility
  lifecycle, address/base-path and component/tag cycle invariants in
  `apps/api/internal/infrastructure/database/migrations/sql/00004_directory.sql` and
  `apps/api/internal/domain/site/address.go`; do not replace visibility transitions with hard deletion.
- Announcement publication windows, allowed transitions, revision history, actor deletion and
  draft-only physical deletion remain database-enforced in
  `apps/api/internal/infrastructure/database/migrations/sql/00007_content_announcements.sql`.
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
