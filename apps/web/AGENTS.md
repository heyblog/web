# Web Module Guidance

This file refines the repository-level `AGENTS.md` for `apps/web`.

## Scope and Sources of Truth

- `apps/web` is the browser-facing Astro and Svelte application.
- Treat `package.json`, `astro.config.ts`, `svelte.config.ts`, and `Taskfile.yml` as the current
  dependency, runtime, and command truth.
- Browser requests that require backend data enter through this application, which communicates
  with the Go API over HTTP.
- The `old` branch is a migration reference for routes, user-visible behavior, server-side API
  forwarding, and tests. Do not copy its dependency versions, commands, or obsolete backend
  assumptions.

## Ownership and Boundaries

- Own routing, rendering, layouts, UI composition, browser state, and the same-origin web boundary.
- Do not own domain rules, authoritative authorization, database schemas, migrations, connections,
  or pools. Those belong to `apps/api`.
- Consume HTTP request and response DTOs. Do not import, recreate, or expose database entities.
- Service communication uses HTTP exclusively.
- Keep changes inside `apps/web` unless a public HTTP contract or shared Node configuration must
  change intentionally.

The target request path is:

```text
Browser -> Astro server route/page -> Go API -> database
```

## Stack and Code Placement

- Use Astro for filesystem routing, layouts, content, server rendering, and page composition.
- Use Svelte for components that require client-side state or interaction. Do not hydrate static
  content without a user-facing need.
- Use the existing Tailwind and shared style setup before adding another styling system.
- For UI, styling, layout, component, accessibility, transition, or animation work, load and follow
  the project `aria-design-system` Skill. Its bundled design sources are authoritative over generic
  UI, Tailwind, framework, and animation guidance.
- Use the `astro-project` Skill for Astro application tasks and the installed Svelte Skills for
  interactive islands. Framework guidance must not override repository boundaries or Aria design
  values.
- Use the installed Playwright Skill only when browser automation is requested or the affected
  behavior cannot be verified reliably below the browser level.
- Keep TypeScript strict and use the `@/*` alias for `src/*` imports where it improves clarity.
- Keep Astro pages and endpoint files thin. Move feature orchestration and reusable logic out of
  route files and components.
- As features return from the old application, prefer these boundaries:
  - `src/pages`: Astro pages and same-origin HTTP endpoints.
  - `src/layouts`: page shells and cross-page layout composition.
  - `src/components`: focused Astro and Svelte presentation components.
  - `src/application/<feature>`: feature-specific orchestration and API adapters.
  - `src/shared`: genuinely cross-feature browser, server, UI, and integration utilities.
- Use `.server.ts` for server-only code and `.browser.ts` for browser-only code. Shared modules must
  not depend on either runtime implicitly.
- Split large feature files by responsibility. Do not rebuild the old application's uneven file
  structure verbatim.

## Rendering and HTTP Data Flow

- The target architecture requires an Astro server runtime for SSR and request interception. Check
  `astro.config.ts` before relying on request-time behavior; enabling SSR requires a coherent
  adapter, output mode, build, preview, and deployment change.
- Decide rendering per route. Request-dependent pages use SSR; content that does not need request
  state may be prerendered.
- Server-rendered pages may call the Go API through server-only application modules.
- Browser mutations, authenticated reads, and live refreshes go through same-origin Astro routes or
  actions. Do not expose the internal API base URL to browser code.
- Keep the API base URL in server-only configuration. Never place internal URLs or secrets in
  `PUBLIC_*` variables.
- Define cache behavior with each data path. Authentication and mutations default to `no-store`;
  cache public reads only with a clear invalidation strategy.
- For live or partial updates, refresh the affected data region instead of reloading the full page.

## Proxy, Authentication, and Error Rules

- Same-origin endpoints must be purpose-built; never create an unrestricted upstream proxy.
- Allow only the required upstream path, method, headers, query fields, and body shape.
- Forward cookies and authentication headers only when the endpoint requires them. Preserve
  relevant `Set-Cookie` headers from the API response.
- Preserve meaningful upstream status codes and response bodies. Convert connection failures to a
  stable `502` response without exposing internal addresses or stack traces.
- Do not forward hop-by-hop headers or stale `content-length`, `transfer-encoding`, or
  `content-encoding` values when rebuilding a response.
- Web middleware may provide redirects and user experience guards, but the Go API remains the
  authority for authentication and authorization.
- Validate browser input at the web boundary for usability; the API must validate it again for
  correctness and security.

## Migration Guidance

- Before rebuilding an old route, inspect its old Astro page, application module, API forwarding
  code, and focused tests together.
- Preserve accepted URLs, status handling, cookies, redirects, and user-visible behavior unless the
  current task or accepted HTTP contract changes them.
- Reimplement old direct or browser-facing backend calls through the current same-origin HTTP
  boundary.
- Do not restore old Node database access, Fastify assumptions, or direct imports from the former
  database package.

## Commands and Validation

Run commands from the repository root:

- `task web:dev`: start the Astro development server.
- `task web:check`: run Astro and TypeScript checks.
- `task web:lint`: run ESLint and Stylelint.
- `task web:format:check`: check formatting.
- `task web:build`: build the web application.
- `task web:verify`: run all current offline web checks.

The current module has no test task. When behavior changes, add focused tests for server adapters,
authentication, proxying, rendering decisions, or critical browser logic as appropriate. Do not
restore the old Vitest or Playwright setup without explaining why it is still the right choice.
Use browser-based end-to-end tests only when requested or when a critical interaction cannot be
verified reliably at a lower level.

## Background Development Server

Run a long-lived Astro development server in background mode:

```bash
pnpm --dir apps/web exec astro dev --background
```

Manage it from the repository root:

- `pnpm --dir apps/web exec astro dev status`
- `pnpm --dir apps/web exec astro dev logs`
- `pnpm --dir apps/web exec astro dev stop`

## Astro Documentation

Full documentation: https://docs.astro.build

Consult these guides before working on related tasks:

- [Adding pages, dynamic routes, or middleware](https://docs.astro.build/en/guides/routing/)
- [Working with Astro components](https://docs.astro.build/en/basics/astro-components/)
- [Using React, Vue, Svelte, or other framework components](https://docs.astro.build/en/guides/framework-components/)
- [Adding or managing content](https://docs.astro.build/en/guides/content-collections/)
- [Adding styles or using Tailwind](https://docs.astro.build/en/guides/styling/)
- [Supporting multiple languages](https://docs.astro.build/en/guides/internationalization/)

## Completion Checks

- Confirm the route's SSR or prerender decision and its cache behavior.
- Confirm browser code cannot reach internal service URLs or database concepts.
- Confirm HTTP forwarding preserves required cookies, statuses, and DTO boundaries.
- Run `task web:verify` and any focused tests introduced by the change.
