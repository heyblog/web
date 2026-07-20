# AGENTS

This file provides guidance to agents working with code in this repository.

## Scope and Instruction Discovery

- These rules apply across the repository.
- Before editing, locate and read every `AGENTS.md` from the repository root to the target path.
- The nearest module-level file may refine or override module-specific rules for its subtree. Root
  rules remain the default for topics it does not address.
- For cross-module work, read the instructions for every affected module and satisfy each module's
  validation requirements.
- If a module has no local `AGENTS.md`, inspect its manifest, Taskfile, configuration, and existing
  code. Do not copy assumptions from another module.

## Project Mission

Build and maintain HeyBlog as a coherent workspace. Keep module ownership clear as it grows. Treat
explicit task requirements as authoritative. If requirements are absent or conflict, state the
ambiguity before implementing behavior.

## Repository Map

- `apps/api`: backend API application.
- `apps/web`: frontend web application.
- `packages/node/configs`: shared Node.js quality-tool configuration.
- `scripts`: repository automation and Git-hook support.
- `skills`: project-owned source Skills installed for development agents.
- `taskfiles`: repository-wide Task definitions included by `Taskfile.yml`.

Modules under `apps/` and `packages/` own their detailed architecture, commands, and conventions in
their nearest `AGENTS.md`.

## Canonical Sources

- Read `.nvmrc` for the Node.js version and `package.json#packageManager` for the pnpm version.
- Read `go.work` and module `go.mod` files for the Go toolchain and dependencies.
- Treat module manifests and lockfiles as dependency truth.
- Treat `Taskfile.yml` and included module Taskfiles as command truth.
- Treat `skills/` as the only source for project-owned Skills. Expose each project-owned Skill in
  `.agents/skills` through a relative symlink; do not maintain copied project-owned directories
  there.
- Treat downloaded third-party directories in `.agents/skills` and the `.claude/skills` link as
  installation outputs.
- `skills-lock.json` records downloaded third-party Skills only. Do not add project-owned or other
  local Skills to it.
- Do not edit package-manager lockfiles manually. Update them only through the owning package
  manager when a dependency change is required.

## Module Guidance Maintenance

- Review the nearest module-level `AGENTS.md` after adding, removing, or upgrading dependencies;
  changing a toolchain, framework, runtime, build, test, development command, environment contract,
  directory structure, ownership boundary, or deployment assumption; or introducing a new module.
- Update the module-level `AGENTS.md` in the same change when its sources of truth, architecture,
  conventions, commands, validation requirements, or completion checks are affected. If no update
  is needed, report that the review was performed.
- Add an `AGENTS.md` when a new module under `apps/` or `packages/` gains module-specific ownership,
  architecture, commands, or validation requirements.
- Keep this maintenance policy in the root `AGENTS.md`. Module-level files contain only
  module-specific sources of truth, architecture, conventions, commands, and validation rules; do
  not duplicate repository-wide or personal workflow preferences there.

## Commands

- `task --list-all`: discover available repository and module tasks.
- `task install`: install repository dependencies and Git hooks when setup is required.
- `task <module>:<task>`: run a focused module task, such as `task api:verify` or
  `task web:verify`.
- `task verify`: run all offline repository checks.
- `task verify:full`: run offline checks and network-backed vulnerability scanning.
- `task security`: run only the network-backed vulnerability checks.

Prefer the narrowest relevant module command while iterating, then run repository-wide validation.

## Principles

### Ownership and Namespace

- Every feature must have a clear owning module responsible for its semantics, configuration,
  lifecycle, and evolution.
- Read across module boundaries, but write within the owning module. Cross-owner changes must be
  intentional, scoped, and validated in every affected module.
- Shared behavior belongs in a shared package only after a real cross-module need is established.
- External tools, caches, and skill data may be read when needed. Do not write outside this
  repository or another owner's namespace without explicit authorization.

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
- In TypeScript, annotate expected types and avoid `unknown` plus inline guards when an existing
  library or domain type can express the contract.
- Exhaust existing library APIs, types, and repository patterns before introducing abstractions or
  projections.
- Separate concerns and split files when it clarifies ownership, testing, or lifecycle boundaries.
- Discuss a heuristic approach and its failure modes before implementing it.
- Treat public API and database schema changes as explicit design decisions. Update specifications
  and consumers with public API changes; do not place raw SQL in route handlers.
- Explain the need for every new dependency before adding it.

## Required Workflow

1. Read relevant root and module instructions and task-specific requirements.
2. Explore affected code and configuration before editing.
3. State a short implementation plan.
4. Make the smallest coherent change that preserves clear ownership.
5. Run the affected modules' focused checks.
6. Run `task verify` and any relevant integration or end-to-end checks.
7. Run `task verify:full` when security validation is required and network access is available.
8. Report changed files, validation evidence, risks, and follow-up work.

## Testing

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

## Definition of Done

A task is complete only when:

- acceptance criteria are satisfied;
- relevant tests, lint, type checking, formatting checks, and builds pass;
- documentation is updated when behavior, architecture, or workflow changes;
- the task introduces no unrelated file changes;
- validation evidence, remaining risks, and follow-up work are reported.
