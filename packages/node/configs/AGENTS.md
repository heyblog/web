# Shared Node Configuration Guidance

This file refines the repository-level `AGENTS.md` for `packages/node/configs`.

## Scope and Sources of Truth

- `packages/node/configs` owns reusable ESLint and Prettier configuration for Node workspace
  modules.
- Treat `package.json`, `tsconfig.json`, root wrapper configurations, and files under `shared/` as
  the package contract and implementation truth.
- Treat package exports as public workspace interfaces. Update affected consumers with any export
  or option change.

## Ownership and Conventions

- Keep application behavior and application-specific policy in the consuming module.
- Prefer shared factories or presets only when at least two consumers need the same behavior.
- Keep the shared Prettier preset plugin-free. Consuming modules compose parser and domain plugins
  from the exported base configuration according to their own file types and tooling.
- Keep the shared ESLint factory framework-neutral. Consuming modules compose framework configs and
  pass them and bare additional file extensions through its generic extension options.
- Keep dependencies imported by shared configurations in the root `package.json`. Consuming modules
  declare parsers and plugins imported only by their local configurations.
- Extend existing factories and typed options before adding parallel configuration entry points.
- Do not weaken lint, formatting, or type-checking rules solely to make a consumer pass.

## Commands and Validation

Run commands from the repository root:

- `task configs:format:check`: check package formatting.
- `task configs:lint`: lint package sources.
- `task configs:typecheck`: type-check package sources.
- `task configs:check`: run all package checks.
- `task configs:verify`: run the package formatting, lint, and type checks.
- `task verify`: validate all consumers after changing an exported configuration or shared rule.
