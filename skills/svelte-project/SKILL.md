---
name: svelte-project
description: Use when creating, changing, or reviewing Svelte components or modules in HeyBlog, including Svelte 5 runes, reactivity, context, hydration, and Astro island behavior.
---

# Svelte Project

Apply current Svelte APIs without weakening the Web module's rendering, security, or design
boundaries.

## Establish context

1. Read the applicable repository and Web `AGENTS.md` files.
2. Treat `apps/web/package.json` and `apps/web/svelte.config.ts` as version and compiler truth when
   the task depends on Svelte version or configuration.
3. Inspect the owning Astro page, component, or application module before changing an island's
   hydration or data flow.

## Svelte semantics

- Use runes mode for new code and preserve the existing component's mode during focused fixes unless
  migration is part of the task.
- Make state reactive only when a template, derived value, or effect observes it. Prefer `$derived`
  for values computed from state and reserve `$effect` for synchronization with external systems or
  browser effects.
- Treat props as changing inputs. Derive dependent values instead of capturing initial props in
  ordinary variables.
- Keep SSR-request state out of shared mutable modules. Use component state or scoped context when
  state must be shared within a rendered subtree.
- Give keyed each blocks stable domain keys. Do not use an array index when item identity matters.
- Use Svelte 5 event attributes and snippets for new code. Do not replace a working action, store,
  class directive, or binding solely to satisfy a generic style preference.
- Keep TypeScript contracts explicit at component and application boundaries.

## Astro boundary

- Svelte components may render on the server without a client directive. Hydrate only when browser
  state or interaction requires it; preserve existing SSR-only Svelte composition.
- Keep server data acquisition and secrets in Astro or `.server.ts` modules. Browser Svelte code
  calls only the accepted same-origin boundary.
- Select the narrowest client directive that satisfies the interaction and preserve a stable SSR
  fallback.

Use `astro-project` for routing, SSR, prerendering, and island placement. For UI, accessibility,
styling, or motion work, also use `aria-design-system`; its design sources remain authoritative.

## Current documentation and optional analysis

Consult the official Svelte documentation when behavior is version-sensitive, unfamiliar, or
ambiguous. Run the Svelte autofixer only for complex reactivity, attachment, hydration, migration,
or a requested Svelte-specific review, and treat its findings as advisory.

Do not use an unpinned `npx @sveltejs/mcp` download. If a repository-managed autofixer is not
available, use the official documentation and the repository's existing checks instead.

## Validate

Run focused tests for changed behavior, then `mise run //apps/web:check`. Use
`mise run //apps/web:verify` when the change affects runtime behavior, rendering, or the production
build. Report any advisory autofixer finding that was intentionally not applied.
