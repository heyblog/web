---
name: astro-project
description: Use when developing or changing Astro routes, pages, layouts, middleware, endpoints, SSR, prerendering, content, integrations, configuration, or Astro/Svelte rendering boundaries in this repository.
---

# Astro Project

Apply current Astro APIs without weakening this repository's module, rendering, or design boundaries.

## Establish Current Context

1. Read every applicable `AGENTS.md`, including `apps/web/AGENTS.md`.
2. For dependency, integration, adapter, rendering-mode, or build work, read `apps/web/package.json`
   and `apps/web/astro.config.ts` and treat them as dependency and runtime truth.
3. Read the relevant module `mise.toml` when selecting commands, then use its narrowest applicable
   task.

## Resolve Astro APIs

Use the official Astro Docs MCP for version-sensitive or unfamiliar API behavior when it is
available. Its endpoint is `https://mcp.docs.astro.build/mcp`.

If that MCP is unavailable, consult `https://docs.astro.build`. Do not substitute stale remembered APIs or third-party summaries.

## Preserve Project Boundaries

- Use Astro for filesystem routing, pages, layouts, middleware, endpoints, server rendering, prerendering, content, integrations, configuration, and same-origin web boundaries.
- Svelte components may be SSR-only or hydrated islands. Add a client directive only for browser
  state or user interaction; preserve existing server-only composition without hydration.
- Use `svelte-project` for Svelte component semantics, runes, reactivity, context, hydration details,
  and Svelte-specific review.
- Follow the ownership, HTTP, security, rendering, migration, and validation rules in `apps/web/AGENTS.md`.
- Keep route files thin and preserve the repository's server-only and browser-only boundaries.
- Use the existing Tailwind setup. UI and motion work must use `aria-design-system` as the highest
  design authority.
- Do not let generic Astro, Svelte, Tailwind, UI, or animation guidance override repository instructions or Aria design rules.

## Validate

Run the focused web checks required by `apps/web/AGENTS.md` and its mise configuration. Confirm SSR versus prerender decisions, browser/server separation, same-origin data flow, and hydration necessity for the affected behavior.
