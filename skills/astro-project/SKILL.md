---
name: astro-project
description: Use when developing or changing Astro routes, pages, layouts, middleware, endpoints, SSR, prerendering, content, integrations, configuration, or Astro/Svelte rendering boundaries in this repository.
---

# Astro Project

Apply current Astro APIs without weakening this repository's module, rendering, or design boundaries.

## Establish Current Context

1. Read every applicable `AGENTS.md`, including `apps/web/AGENTS.md`.
2. Read `apps/web/package.json` and `apps/web/astro.config.ts` before every Astro task. Treat them as dependency and runtime truth; do not hard-code or assume dependency versions.
3. Read the relevant Taskfile before choosing commands, then use its narrowest applicable task.

## Resolve Astro APIs

Query the official Astro Docs MCP named `astro-docs` first for API behavior. Its endpoint is `https://mcp.docs.astro.build/mcp`.

If that MCP is unavailable, consult `https://docs.astro.build`. Do not substitute stale remembered APIs or third-party summaries.

No official Astro Skill is suitable for application development. Do not use `astro-maintainer-skills`; it targets maintenance of the Astro monorepo, not this application.

## Preserve Project Boundaries

- Use Astro for filesystem routing, pages, layouts, middleware, endpoints, server rendering, prerendering, content, integrations, configuration, and same-origin web boundaries.
- Use Svelte only for islands that require browser state or user interaction. Keep static content in Astro and do not hydrate it without a user-facing need.
- Follow the ownership, HTTP, security, rendering, migration, and validation rules in `apps/web/AGENTS.md`.
- Keep route files thin and preserve the repository's server-only and browser-only boundaries.
- Use the existing Tailwind setup. Apply `aria-design-system` as the highest authority for project UI and motion when it is available.
- Do not let generic Astro, Svelte, Tailwind, UI, or animation guidance override repository instructions or Aria design rules.

## Validate

Run the focused web checks required by `apps/web/AGENTS.md` and the Taskfile. Confirm SSR versus prerender decisions, browser/server separation, same-origin data flow, and hydration necessity for the affected behavior.
