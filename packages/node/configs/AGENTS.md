# Shared Node Configuration Guidance

This file refines the repository-level `AGENTS.md` for `packages/node/configs`.

## Scope and Sources of Truth

- `packages/node/configs` owns reusable ESLint, Prettier, and Stylelint configuration for Node
  workspace modules.
- Treat `package.json`, `tsconfig.json`, root wrapper configurations, and files under `shared/` as
  the package contract and implementation truth.
- Treat package exports as public workspace interfaces. Update affected consumers with any export
  or option change.

## Ownership and Conventions

- Keep application behavior and application-specific policy in the consuming module.
- Prefer shared factories or presets only when at least two consumers need the same behavior.
- Keep shared tool dependencies in the root `package.json` because the root and workspace modules
  execute these configurations through the repository toolchain.
- Extend existing factories and typed options before adding parallel configuration entry points.
- Do not weaken lint, formatting, or type-checking rules solely to make a consumer pass.

## Commands and Validation

Run commands from the repository root:

- `pnpm --filter @heyblog/configs run format:check`: check package formatting.
- `pnpm --filter @heyblog/configs run lint`: lint package sources.
- `pnpm --filter @heyblog/configs run typecheck`: type-check package sources.
- `task verify`: validate all consumers after changing an exported configuration or shared rule.
