# AGENTS

This file governs this repository. All file references below are repository-root-relative.
Keep project instructions and their evidence inside this repository.

## Scope and Instruction Discovery

- These rules apply across the repository.
- Before editing, locate and read every `AGENTS.md` from the repository root to the target path.
- The nearest module-level file may refine or override module-specific rules for its subtree. Root
  rules remain the default for topics it does not address.
- For cross-module work, read the instructions for every affected module and satisfy each module's
  validation requirements.
- If a module has no local `AGENTS.md`, inspect its manifest, mise configuration, and existing
  code. Do not copy assumptions from another module.

## Project Mission

Build and maintain HeyBlog as a coherent workspace. Keep module ownership clear as it grows. Treat
explicit task requirements as authoritative. If requirements are absent or conflict, state the
ambiguity before implementing behavior.

## Repository Map

- `apps/api`: backend API application.
- `apps/web`: frontend web application.
- `packages/node/configs`: shared Node.js quality-tool configuration.
- `infra`: development dependencies and deployment infrastructure configuration.
- `.github`: repository automation, reusable actions, and CI workflows.
- `scripts`: repository automation and Git-hook support.
- `skills`: project-owned source Skills installed for development agents.
- `mise`: repository-wide mise task definitions loaded by `mise.toml`.

Modules under `apps/` and `packages/` own their detailed architecture, commands, and conventions in
their nearest `AGENTS.md`.

## Canonical Sources

- Treat `mise.toml#vars.project_version` as the only project release version for the API, Web
  application, and repository-owned packages. It contains three JavaScript-safe non-negative
  integers in strict `X.Y.Z` form. Do not add `version` fields to project `package.json` files.
- Read root `mise.toml` and `mise.lock` for exact managed tool versions. Read `go.work` and module
  `go.mod` files for Go workspace compatibility and dependencies.
- Treat the root `go.mod` and `go.sum` as repository-wide Go tool manifests only. Application
  dependencies belong to their owning module under `apps/`.
- Treat module manifests and lockfiles as dependency truth.
- Treat root and module `mise.toml` files plus `mise/tasks/*.toml` as command truth.
- Use the `.yaml` extension for every repository-owned YAML file and reference. Do not add `.yml`
  files or `.yml` compatibility patterns.
- Treat `skills/` as the only source for project-owned Skills. Expose each project-owned Skill in
  `.agents/skills` through a relative symlink; do not maintain copied project-owned directories
  there.
- Treat downloaded third-party directories in `.agents/skills` and the individual relative links
  inside `.claude/skills` as installation outputs. Do not rewrite downloaded Skill rules locally;
  project instructions own activation and conflict resolution.
- `skills-lock.json` records downloaded third-party Skills only. Do not add project-owned or other
  local Skills to it.
- Do not edit package-manager lockfiles manually. Update them only through the owning package
  manager when a dependency change is required.

## Module Guidance Maintenance

- Review the nearest module-level `AGENTS.md` after adding, removing, or upgrading dependencies;
  changing a toolchain, framework, runtime, build, test, development command, environment contract,
  directory structure, ownership boundary, or deployment assumption; or introducing a new module.
- Update the module-level `AGENTS.md` in the same change when its sources of truth, architecture,
  conventions, commands, validation requirements, or completion checks are affected.
- Add an `AGENTS.md` when a new module under `apps/` or `packages/` gains module-specific ownership,
  architecture, commands, or validation requirements.
- Keep this maintenance policy in the root `AGENTS.md`. Module-level files contain only
  module-specific sources of truth, architecture, conventions, commands, and validation rules; do
  not duplicate repository-wide or personal workflow preferences there.

## Commands

- `mise tasks ls --all`: discover available repository and module tasks.
- `mise run install`: install repository dependencies only.
- `mise run prepare`: sync generated Web content; `mise run setup` additionally installs
  dependencies and Git hooks.
- `mise run version:show|check|patch|minor|major`: inspect, validate, or increment the project
  version; use `mise run version:set -- X.Y.Z` to set an exact version.
- `mise run //<module-path>:<task>`: run a focused module task from the root, such as
  `mise run //apps/api:verify`; from that module, use `mise run :verify`.
- `mise run check`: run repository formatting, lint, type, SQL, mise, and workflow checks.
- `mise run verify`: run checks, ordinary tests, and application builds.
- `mise run verify:full`: run extended tests, builds, dependency and container security validation.
- `mise run security`: run only the network-backed vulnerability checks.

Use the validation matrix below; root `verify` does not include API race tests, container
integration tests, or the Web standalone smoke task.

## Principles

### Ownership and Namespace

- Every feature must have a clear owning module responsible for its semantics, configuration,
  lifecycle, and evolution.
- Read across module boundaries, but write within the owning module. Cross-owner changes must be
  intentional, scoped, and validated in every affected module.
- Shared behavior belongs in a shared package only after a real cross-module need is established.
- Keep project changes and instruction references within the repository and the requested owner's
  namespace.

### Architecture First

- When compatibility is not an explicit requirement, prefer a clean architectural refactor over a
  compatibility shim.
- Complete breaking changes coherently: update affected callers, contracts, tests, and
  documentation in the same change.
- Adopt a clearly better structure when it is within task scope. Do not preserve technical debt for
  incremental compatibility.
- Avoid unrelated redesigns. Architectural improvement must serve the requested outcome.

### Code Quality

- Use each language's type system directly. Fix typing problems at the correct boundary instead of
  weakening types locally.
- In TypeScript, use explicit internal contracts. Untrusted JSON may enter as `unknown` and be
  parsed at its boundary; do not replace runtime parsing with an unchecked type assertion.
- Exhaust existing library APIs, types, and repository patterns before introducing abstractions or
  projections.
- Separate concerns and split files when it clarifies ownership, testing, or lifecycle boundaries.
- Discuss a heuristic approach and its failure modes before implementing it.
- Treat public API and database schema changes as explicit design decisions. Update specifications
  and consumers with public API changes; do not place raw SQL in route handlers.
- Explain the need for every new dependency before adding it.

### Environment Configuration

- Before adding an environment variable, compare its meaning, sensitivity, lifecycle, defaults,
  and ownership with existing variables. Prefer reuse when those semantics are consistent; do not
  create module-specific aliases for the same value.
- Runtime application environment reads belong to each module's typed configuration boundary:
  `apps/api/internal/config` and `apps/web/src/config.server.ts`. Other runtime code receives that
  configuration. Build integrations and task scripts may read their own build-time inputs; keep
  those inputs out of browser runtime configuration and production runner secrets.
- Task and Compose files select and inject scenario-specific environment files; application modules
  own configuration validation.
- Scenario templates contain only variables required or intentionally overridden in that scenario.
  Do not enumerate optional variables that already have suitable code defaults.

## Required Workflow

1. Read relevant root and module instructions and task-specific requirements.
2. Explore affected code and configuration before editing.
3. For non-trivial work, state a short implementation plan.
4. Make the smallest coherent change that preserves clear ownership.
5. Validate using the matrix below. Preserve behavior and public contracts during structural
   refactors unless the task explicitly requests a behavior change.
6. Report changed files, validation evidence, remaining risks, and follow-up work.

## Testing

| Change scope | Required validation |
| --- | --- |
| One module | Focused tests while iterating, then the owning module's `:verify` |
| Shared configuration or cross-module | Affected module checks and root `mise run verify` |
| Database, auth, infrastructure, or lifecycle changes | Relevant integration, race, smoke, or container tasks in addition to the above |
| Security or dependency risk | Relevant security tasks; network-backed checks require network access |

Use `verify:full` for an explicitly requested full gate or when its combined coverage is needed.
Report any required check that could not run; a smaller passing check does not replace it.

- Never disable, delete, or weaken a test or quality gate to make a task pass.
- Always run existing tests relevant to changed behavior.
- Add tests when requested, when behavior changes, or when a critical path cannot be verified
  reliably otherwise. Do not add component tests solely to increase coverage.
- Use browser-based, integration, or end-to-end tests only when the task requests them or the
  affected behavior cannot be verified adequately at a lower level.
- Stop on required-check failures and report the exact command and first actionable failure. Fix
  failures caused by the task; report pre-existing failures separately.

## Engineering and Safety Rules

- Keep changes within task scope. Preserve and report unrelated worktree changes separately.
- Do not read, modify, or commit real `.env` files, credentials, tokens, secrets, or production
  data.
- Do not modify generated files directly or introduce dependencies without justification.
- Stop and report conflicting requirements, unclear ownership, or a change that needs broader
  authority.
- When commits are requested, use Conventional Commits with a maximum 72-character header, as
  enforced by `commitlint.config.cjs`.
