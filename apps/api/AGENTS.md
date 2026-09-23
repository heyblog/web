# API Module Guidance

This file refines the repository-level `AGENTS.md` for `apps/api`.

## Scope and Sources of Truth

- `apps/api` is the repository's unified Go HTTP backend. Invoke its tasks from the repository root
  with `mise run //apps/api:<task>` or from this module with `mise run :<task>`; module commands
  execute relative to `apps/api`.
- The API inherits the project release version from `mise.toml#vars.project_version`; it does not
  maintain a module-specific version or expose the version at runtime.
- Treat `go.mod`, `go.sum`, source code, and `mise.toml` as the current dependency and command truth;
  the repository-root `mise.toml` owns the Go toolchain version.
- Declare API-specific Go CLI dependencies in this module's `go.mod` `tool` block. Repository-wide
  Go tools belong to the root tool module and must not require separate global installations.
- Treat current task requirements, current HTTP contracts, schema requirements, and tests as
  migration behavior truth.
- Do not introduce Fastify, Drizzle, Node database access, or per-service connection pools.

## Ownership and Service Boundary

- Own public and internal HTTP APIs, domain behavior, authoritative authentication and
  authorization, database schema and migrations, database access, and connection-pool lifecycle.
- Maintain the repository's single database-access boundary. Other applications communicate with
  this application over HTTP and must not create their own database pools.
- Service-to-service communication uses HTTP exclusively.
- Return HTTP DTOs rather than database rows or persistence models.
- Keep API-specific configuration under an `API_` prefix when new environment variables are
  required. Never read real secret files during development or tests.
- API development tasks load the repository-root `.env.development`; tests use isolated fixtures
  and require no environment file. Development mail uses the Mailpit SMTP binding declared by
  `API_MAIL_SMTP_URL`. Production orchestrators inject the three external service URLs plus
  `API_HEALTHCHECK_TOKEN` and `API_WEB_TOKEN` with Docker
  `--env-file` or Compose `env_file`. User authentication additionally requires
  `API_AUTH_ACCESS_SECRET`, `API_AUTH_REFRESH_SECRET`, `API_GITHUB_CLIENT_ID`, and
  `API_GITHUB_CLIENT_SECRET`; the image does not load dotenv files.
  `internal/config` is the API's only process-environment reader and exports validated typed
  configuration.
- Keep the root development and production environment templates limited to application service
  bindings and tokens required by API and Web, grouped by owning module and shared boundary.
  Development dependency defaults belong in the development Compose file.
- `internal/config` exclusively owns YAML discovery/loading and process-environment loading. Other
  packages receive only validated typed configuration through constructors and must not call
  `os.Getenv`, `os.LookupEnv`, or read application configuration files.
- `config/default.yaml` is the documented non-sensitive development baseline. The optional, ignored
  `config/conf.yaml` declares `mode: development|production` when present and contains scenario
  differences; the application never creates or rewrites either file.
- `auth.web_base_url` is the browser-visible Web origin. The default configuration uses the local
  Astro origin; scenario overrides replace it for production. GitHub always callbacks through the Web route at
  `<auth.web_base_url>/auth/github/callback`; do not configure or expose a direct API callback.
- Configuration discovery first checks the real executable's sibling `config/` directory and falls
  back to the current working directory's `config/` only when the executable default is absent.
  Both files come from the same directory and use the fixed names `default.yaml` and `conf.yaml`.
- The service accepts no arguments. `--healthcheck` is the only supported invocation flag and is
  owned by bootstrap rather than application configuration.
- Environment variables own secrets, credentials, and deployment-injected external resource
  bindings. Non-sensitive server, logging, timeout, pool, CORS, proxy, and health policy belongs in
  YAML.
- `mail.transport: auto` selects SMTP in development and SES in production. Development SMTP uses
  `API_MAIL_SMTP_URL`; production rejects SMTP. `mail.smtp.timeout`, `mail.ses.region`, and
  purpose-specific sender addresses belong in YAML. AWS SES credentials use the AWS SDK standard
  environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and optional
  `AWS_SESSION_TOKEN` for STS); never add credentials to YAML. Missing startup dependencies fail API
  bootstrap, while dependencies lost during runtime make mail operations unavailable.
- Production containers use internal port `10201`; host exposure changes through container port
  mapping rather than production YAML overrides.

## Stack and Growth Path

- Use the Go toolchain and Gin versions declared by `go.mod`; do not hardcode versions elsewhere.
- Keep Gin as the HTTP router and middleware host. Register business endpoints through Huma typed
  operations so request and response Go types generate the runtime OpenAPI 3.1 contract. Expose
  `/openapi.json`, `/openapi.yaml`, and the embedded Swagger UI at `/swagger` only in development;
  do not commit generated contract files or expose an OpenAPI 3.0 downgrade.
- Outbound email uses local SMTP in development and the official AWS SDK v2 SES v2 client in
  production. `internal/mail` owns the transports, message validation, and purpose-specific
  templates; callers provide content through its typed interfaces instead of constructing SMTP or
  SES requests directly.
- Treat the current root `main.go` as a minimal composition root, not a pattern for placing the
  whole application in one file.
- Keep `main.go` as the only Go source file in the module root. Place all other Go implementation
  and tests in cohesive packages under `internal`.
- As the service grows, preserve these architectural responsibilities regardless of physical
  package layout:
  - Bootstrap: configuration, logging, shared dependencies, HTTP server startup, and shutdown.
  - HTTP transport: routing, request validation, authentication, rate limiting, status codes, and
    DTO serialization.
  - Application: use-case orchestration, authorization decisions, transaction boundaries, and
    coordination of domain and infrastructure services.
  - Domain: business rules and domain types without Gin, SQL, or external-service dependencies.
  - Infrastructure: repositories, migrations, caches, and outbound HTTP clients.
- Dependencies point inward. Domain code must not import Gin handlers or database implementations.
- Keep packages cohesive and split bootstrap, handlers, use cases, and repositories when their
  lifecycles or reasons to change differ.
- Exhaust the Go standard library, Gin, and established repository patterns before adding an
  abstraction or dependency.

## HTTP Endpoint Rules

- Classify every endpoint as public, authenticated, administrative, web-internal, or internal before
  implementation.
- Define the method, path, request fields, response DTO, status codes, authentication, authorization,
  rate limit, and timeout behavior together.
- Validate path, query, header, and body inputs at the transport boundary. Do not pass unchecked
  transport values into application or repository code.
- Keep handlers thin: parse and validate, call one application operation, then map its result or
  typed error to an HTTP response.
- HTTP endpoints return `(Response, error)` through the module's Result-like endpoint adapter.
  Failure-capable middleware must return immediately on error and call the next endpoint only after
  its own checks succeed. ErrorBoundary is the sole Problem Details response writer.
- Expected failures use typed application errors and explicit error returns. Do not use panic for
  validation, business, authorization, rate-limit, or dependency failures, and do not log the same
  propagated error in multiple layers.
- Preserve stable external behavior when migrating existing endpoints unless the current specification
  explicitly changes it.
- Do not leak internal errors, SQL details, upstream addresses, credentials, or stack traces.
- Internal HTTP endpoints require an explicit trust and authentication model; network placement is
  not authorization.
- `GET /ping`, `GET /home`, `GET /sites`, `GET /sites/options`, `GET /sites/random`,
  `GET /sites/id/:identifier`, `GET /sites/id/:identifier/icon`, and `GET /sites/custom/:customId` are
  web-internal, have no application rate limit, and require the shared `X-HeyBlog-Web-Token`.
  Future direct third-party routes must be registered explicitly as public instead of weakening
  web-internal authentication. `GET /health/live` and `/health/ready` are
  internal, have no application rate limit, and require
  `Authorization: Bearer <API_HEALTHCHECK_TOKEN>`. The liveness response is immediate; readiness is
  bounded by `health.readiness_timeout`. Authentication failures return 401 without probing
  dependencies.
- `GET /sites/random` selects one `VISIBLE` blog, including blogs with warnings, through its own
  database query. Optional `level1` and `level2` filters are exact enabled classification names
  after trimming; `level2` requires its parent. Reject empty, repeated, unknown, overlong, disabled,
  or mismatched parameters with 400 Problem Details and stable `random_*` codes before selection.
  A valid filter with no visible candidate returns 200 with `site: null`; dependency failures
  remain 503. `preview` belongs to Web and is not an API parameter. Responses use `no-store`.
- Propagate request cancellation and deadlines through application, database, cache, and outbound
  HTTP calls.
- Site profiles expose nullable `iconHash` containing the cached icon's lowercase SHA-256 hex
  digest, without icon bytes. The icon endpoint accepts short IDs or UUIDs and reads cached
  `directory.site_icons` bytes only; visible and hidden sites are readable, removed sites are not.
  It returns `image/png`, a quoted digest ETag, and `Cache-Control: no-store`. Missing or invalid
  icons return 404; database failures return 503. Normalization accepts PNG, JPEG, GIF (first
  frame), WebP, and ICO (largest image), preserves aspect and alpha, and caps output at 128 pixels
  without upscaling. Input is bounded to 1 MiB, 2048 pixels per axis and four million pixels;
  ICO entry lengths and palette sizes are checked before decoding. `internal/siteicon` owns
  normalization using standard image codecs, `golang.org/x/image`, and `go-ico`.
- Browser authentication routes under `/auth/*` are web-internal and require `X-HeyBlog-Web-Token`.
  Local authentication uses Argon2id passwords, six-digit email verification, one-time password
  reset links, short-lived access JWTs, and Redis-backed rotating refresh sessions. Verification
  codes expire after ten minutes and password-reset links expire after thirty minutes. Register,
  resend-verification, and forgot-password requests retain their route/IP limits and share Redis
  mail controls: one request per normalized email per minute, ten per normalized email per hour,
  and five hundred globally per hour. Both session tokens are delivered only through HttpOnly
  SameSite=Lax cookies; account security changes increment `auth_version` to invalidate existing
  sessions.
- In production, Nginx accepts the EdgeOne-derived client address and the Web service forwards one
  validated address to the API. Configure `http.trusted_proxies` with only the private Web/API
  Compose subnet and keep it synchronized with `HEYBLOG_NETWORK_SUBNET`. Client IP metadata is for
  logging and applicable rate limits, not service API authorization. `/internal/v1/data-import`
  relies on its scoped INTERNAL API key; origin trust is separate deployment hardening.
- GitHub OAuth state is single-use in Redis. Login may match only a verified primary GitHub email;
  binding requires that email to match the authenticated account, and unbinding must leave a local
  password login method.
- `/management/users*` remains web-internal but also requires an authenticated `SYS_ADMIN` or an
  `ADMIN` with `user.manage`; role and permission mutations must enforce scope and self-management
  restrictions in the application layer.
- `/site-submissions*` is a web-internal anonymous submission boundary protected by the shared Web
  token and route-specific rate limits. CREATE, UPDATE, DELETE, and RESTORE requests return a
  one-time lookup credential; only its SHA-256 digest is persisted. `/management/site-audits*`
  additionally requires `site_audit.review`, and creating new canonical taxonomy entries during
  approval requires `taxonomy.manage`.
- `POST /internal/v1/data-import` is an internal service endpoint. It requires an INTERNAL API key
  with `data_import.write`, accepts only the cleaned migration bundles, and owns a ninety-minute
  request deadline and route-specific upload limit. Its single transaction
  requires PostgreSQL `max_locks_per_transaction >= 512` because existing site and friend-link
  graph wrappers hold transaction-level advisory locks until commit. Keep all removable
  feature logic and tests under `internal/dataimport`; its sqlc queries belong to the shared database
  query and generated-code boundary.
- Machine credentials are opaque, scoped API keys owned by `identity.api_clients`. Only SYS_ADMIN
  users may create, issue, rotate, disable, or revoke them through Web-authenticated management routes.
  INTERNAL keys expire within ninety days; EXTERNAL keys may be explicitly permanent. Never log or
  persist presented key secrets.
- `/v1/example` supports GET, HEAD, POST, PUT, PATCH, DELETE, and OPTIONS with `example.call` for
  either INTERNAL or EXTERNAL clients. `data_import.write` remains INTERNAL-only; `sites.read` is
  not a supported scope. Client scopes must be nonempty and unique.
- `POST /management/api-clients/{id}/keys` issues a credential only to an enabled client without
  active keys, under a client row lock shared with rotation and client updates. It returns 201 and
  the one-time credential, or 409 `api_client_has_active_key` / `api_client_disabled`. Issuance keeps
  revoked and expired key history and applies the same expiration rules as client creation.

## Database and Data Access

- Database bootstrap uses three roles: `postgres` for cluster administration, non-superuser
  `migrator` for Goose and application-object ownership, and `api_runtime` for explicit runtime
  grants. Never expose the `postgres` connection to application configuration.
- Bootstrap owns AGE installation, role creation, role-level AGE preloading, database grants, and
  the `migration` schema. Goose migrations validate those prerequisites and must remain runnable
  by `migrator` without superuser, database-creation, or role-management privileges.
- The greenfield v1 business schema contains `identity`, `directory`, and `content`. Do not restore
  deleted legacy schemas or compatibility tables.
- `identity` keeps reversible access status and email verification separate from account deletion.
  User-requested deletion has a thirty-day cancellation window; completion anonymizes account data,
  removes OAuth identities, releases the email, and permanently reserves the public username.
- Resolve path-relative site resources against the site's `base_path` before storing a host-root
  `url_ref`. Reconstruct same-host URLs from Scheme, Host, and `url_ref` without appending
  `base_path` again.
- `software_component_dependencies` owns reusable direct component-stack relationships;
  `site_software_components` owns site-specific usage evidence. Reject self-dependencies and
  direct or indirect dependency cycles.
- `content` owns MAIN and BANNER announcements, time-derived visibility, and immutable published
  revision history. Do not persist or schedule display-state transitions.
- Enforce non-overlapping published banner windows in PostgreSQL. Only drafts may be hard-deleted;
  runtime code may read but never mutate announcement revisions.
- Store compact Markdown source for announcements. Its syntax and link-safety validation belong to
  the HTTP/application boundary rather than database migrations.
- Apache AGE graph `directory_graph` is the authoritative store for directed site friend links.
  Do not add a relational friend-link mirror or directly modify AGE-generated label tables.
- Access AGE only through typed `directory` functions. Runtime code may execute the public friend
  link wrappers but must not receive direct graph-table access.
- Synchronize registered `SiteRef` vertices from `directory.sites` with database triggers in the
  same transaction. Do not implement relation-and-graph dual writes in Go.
- Initialize the database pool once during application startup, inject it into repositories, and
  close it during graceful shutdown.
- Apply all pending Goose migrations with `API_MIGRATION_DATABASE_URL` before opening runtime
  database and Redis connections.
- Do not open database connections or pools per handler, feature, repository, or request.
- Keep database reads and writes behind repository or data-access packages. Do not write raw SQL in
  Gin handlers or application orchestration.
- Place transaction boundaries in the application operation that owns the complete write lifecycle.
- Maintain primary keys, foreign keys, uniqueness, indexes, and non-null constraints in Go-owned
  migrations and schema definitions.
- Document every migration column both inline with `--` and in the PostgreSQL catalog with a
  matching `COMMENT ON COLUMN` statement.
- Document every business schema, table, function, and trigger with the matching PostgreSQL
  `COMMENT` statement. Keep AGE property documentation in `plan.md` and wrapper function comments.
- Treat schema changes as explicit architecture work. Update migrations, repository mappings,
  affected DTOs, callers, and integration tests coherently.
- Map persistence records to domain values and HTTP DTOs deliberately; do not share database entity
  shapes with TypeScript consumers.
- `directory.site_audits` stores immutable aggregate evidence for site lifecycle requests. Approved
  reviews apply snapshots to the normalized directory tables in one transaction; the JSONB
  snapshots are audit history, not a second canonical site model.
- If a cache is introduced, initialize and own it centrally with an explicit invalidation and
  shutdown lifecycle. Other services must not bypass the API's data ownership.
- The selected data-access stack is pgx/v5 for PostgreSQL connections, Goose v3 for migrations,
  and sqlc for generated queries. Treat these choices as current architecture truth; explain and
  confirm any replacement or additional abstraction before changing dependencies or schema
  infrastructure.

## Migration Guidance

- For each migrated feature, define the route, use case, domain rules, repository boundary, and
  focused tests from current requirements and current contracts before implementing it in Go.
- Translate behavior and boundaries into explicit Go startup dependencies and Gin transport
  adapters; keep use cases and domain rules framework-independent.
- Derive data requirements from current contracts and schema requirements. The Go schema and
  migrations are authoritative.
- Preserve externally accepted paths, validation, status codes, authorization, and response meaning
  unless the new specification requires a breaking change.
- Do not add compatibility shims solely to imitate obsolete internal APIs.

## Testing

- Use `net/http/httptest` or the narrowest appropriate test boundary for HTTP behavior.
- Handler tests cover routing, validation, authentication, authorization, status codes, headers,
  and response DTOs.
- Test outbound email with injected clients or senders. Automated tests must not require AWS
  credentials or send real email.
- Application and domain tests cover business rules without starting a real HTTP server.
- Repository and migration behavior that depends on database semantics requires focused integration
  tests against an isolated test database.
- Test shutdown, rollback, or failure behavior when changing pool, transaction, cache, or outbound
  client lifecycle.
- Never require production credentials or data, and never weaken a test to make validation pass.

## Commands and Validation

Run commands from the repository root:

- `mise run //apps/api:dev`: run the API locally.
- `mise run //apps/api:test`: run Go tests.
- `mise run //apps/api:test:race`: run Go tests with the race detector.
- `mise run //apps/api:test:integration`: run PostgreSQL/AGE, Redis, and Mailpit container integration tests,
  including the internal import transaction, graph boundary, and SMTP delivery path.
- `mise run //apps/api:format:check`: check Go formatting and imports.
- `mise run //apps/api:lint`: run golangci-lint.
- `mise run //apps/api:build`: invoke the API build from the repository root; the module command runs in
  `apps/api`.
- `mise run //apps/api:check`: run API static checks.
- `mise run //apps/api:security`: run the network-backed API vulnerability check.
- `mise run //apps/api:verify`: run API checks, race-enabled tests, and the build.
- `mise run //apps/api:vulncheck`: run the network-backed vulnerability check when dependencies or security
  behavior change and network access is available.

Integration tests own the Testcontainers resources they create and clean them up at test completion;
mise does not start or stop developer-owned Compose services.

## Completion Checks

- Confirm the endpoint classification, HTTP contract, authentication, authorization, and rate
  limit.
- Confirm handlers contain no domain logic or direct database access.
- Confirm all data access uses the application-owned pool and has a clear lifecycle.
- Confirm migrations enforce relational constraints and DTOs do not expose persistence models.
- Run `mise run //apps/api:verify`, focused integration tests, and
  `mise run //apps/api:vulncheck` when applicable.
