# API Module Guidance

This file refines the repository-level `AGENTS.md` for `apps/api`.

## Scope and Sources of Truth

- `apps/api` is the unified Go HTTP backend for the web, shell, edge, and worker applications.
- Treat `go.mod`, `go.sum`, source code, and `Taskfile.yml` as the current dependency, toolchain, and
  command truth.
- The `old` branch is a migration reference for product behavior, routes, use cases, domain rules,
  and tests. It is not dependency or implementation truth.
- Do not restore the old Fastify, Drizzle, Node database, or per-service connection model.

## Ownership and Service Boundary

- Own public and internal HTTP APIs, domain behavior, authoritative authentication and
  authorization, database schema and migrations, database access, and connection-pool lifecycle.
- Maintain the repository's single database-access boundary. Web, shell, edge, and worker services
  communicate with this application over HTTP and must not create their own database pools.
- Service-to-service communication uses HTTP exclusively.
- Return HTTP DTOs rather than database rows or persistence models.
- Keep API-specific configuration under an `API_` prefix when new environment variables are
  required. Never read real secret files during development or tests.

## Stack and Growth Path

- Use the Go toolchain and Gin versions declared by `go.mod`; do not hardcode versions elsewhere.
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

- Classify every endpoint as public, authenticated, administrative, or internal before
  implementation.
- Define the method, path, request fields, response DTO, status codes, authentication, authorization,
  rate limit, and timeout behavior together.
- Validate path, query, header, and body inputs at the transport boundary. Do not pass unchecked
  transport values into application or repository code.
- Keep handlers thin: parse and validate, call one application operation, then map its result or
  typed error to an HTTP response.
- Preserve stable external behavior when migrating old endpoints unless the current specification
  explicitly changes it.
- Do not leak internal errors, SQL details, upstream addresses, credentials, or stack traces.
- Internal HTTP endpoints require an explicit trust and authentication model; network placement is
  not authorization.
- Propagate request cancellation and deadlines through application, database, cache, and outbound
  HTTP calls.

## Database and Data Access

- Initialize the database pool once during application startup, inject it into repositories, and
  close it during graceful shutdown.
- Do not open database connections or pools per handler, feature, repository, or request.
- Keep database reads and writes behind repository or data-access packages. Do not write raw SQL in
  Gin handlers or application orchestration.
- Place transaction boundaries in the application operation that owns the complete write lifecycle.
- Maintain primary keys, foreign keys, uniqueness, indexes, and non-null constraints in Go-owned
  migrations and schema definitions.
- Treat schema changes as explicit architecture work. Update migrations, repository mappings,
  affected DTOs, callers, and integration tests coherently.
- Map persistence records to domain values and HTTP DTOs deliberately; do not share database entity
  shapes with TypeScript consumers.
- If a cache is introduced, initialize and own it centrally with an explicit invalidation and
  shutdown lifecycle. Other services must not bypass the API's data ownership.
- The selected data-access stack is pgx/v5 for PostgreSQL connections, Goose v3 for migrations,
  and sqlc for generated queries. Treat these choices as current architecture truth; explain and
  confirm any replacement or additional abstraction before changing dependencies or schema
  infrastructure.

## Migration Guidance

- For each feature migrated from `old`, inspect the corresponding route, use case, domain service,
  repository, and tests before implementing it in Go.
- Translate behavior and boundaries, not TypeScript structure. Fastify plugins become explicit Go
  startup dependencies; old route handlers become Gin transport adapters; use cases and domain
  rules remain framework-independent.
- Use the old database implementation only to understand behavior and data requirements. The Go
  schema and migrations become authoritative.
- Preserve externally accepted paths, validation, status codes, authorization, and response meaning
  unless the new specification requires a breaking change.
- Do not add compatibility shims solely to imitate obsolete internal APIs.

## Testing

- Use `net/http/httptest` or the narrowest appropriate test boundary for HTTP behavior.
- Handler tests cover routing, validation, authentication, authorization, status codes, headers,
  and response DTOs.
- Application and domain tests cover business rules without starting a real HTTP server.
- Repository and migration behavior that depends on database semantics requires focused integration
  tests against an isolated test database.
- Test shutdown, rollback, or failure behavior when changing pool, transaction, cache, or outbound
  client lifecycle.
- Never require production credentials or data, and never weaken a test to make validation pass.

## Commands and Validation

Run commands from the repository root:

- `task api:dev`: run the API locally.
- `task api:test`: run Go tests.
- `task api:format:check`: check Go formatting and imports.
- `task api:lint`: run golangci-lint.
- `task api:build`: build the API.
- `task api:verify`: run all current offline API checks.
- `task api:vulncheck`: run the network-backed vulnerability check when dependencies or security
  behavior change and network access is available.

## Completion Checks

- Confirm the endpoint classification, HTTP contract, authentication, authorization, and rate
  limit.
- Confirm handlers contain no domain logic or direct database access.
- Confirm all data access uses the application-owned pool and has a clear lifecycle.
- Confirm migrations enforce relational constraints and DTOs do not expose persistence models.
- Run `task api:verify`, focused integration tests, and `task api:vulncheck` when applicable.
