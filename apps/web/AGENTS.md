# Web Module Guidance

This file refines the repository-level `AGENTS.md` for `apps/web`.

## Scope and Sources of Truth

- `apps/web` is the browser-facing Astro and Svelte application.
- Treat `package.json`, `astro.config.ts`, `src/site.config.ts`, `svelte.config.ts`, and
  `Taskfile.yaml` as the current dependency, runtime, site metadata, and command truth.
- Treat `apps/web/contents` as a generated snapshot of the `contents/` directory from the
  `heyblog/.github` repository. `task web:content` resolves the remote `main` branch to a commit,
  requires every source declared in `content-sources.mjs` and consumed by `content.config.ts` to be
  non-empty, downloads the files from that immutable commit, and records the resolved source in
  `.source-revision`.
- Browser requests that require backend data enter through this application, which communicates
  with the Go API over HTTP.
- Treat current task requirements, accepted HTTP contracts, current routes, and tests as
  migration behavior truth.
- `apps/web/Dockerfile` builds from the workspace and uses `pnpm deploy --prod` to create the
  portable runtime tree. The runner must not copy workspace-wide development dependencies.

## Ownership and Boundaries

- Own routing, rendering, layouts, UI composition, browser state, and the same-origin web boundary.
- Do not own domain rules, authoritative authorization, database schemas, migrations, connections,
  or pools. Those belong to `apps/api`.
- Consume HTTP request and response DTOs. Do not import, recreate, or expose database entities.
- Service communication uses HTTP exclusively.
- Keep changes inside `apps/web` unless a public HTTP contract or shared Node configuration must
  change intentionally.
- `src/config.server.ts` is the only Web application environment reader. It validates the private
  `WEB_API_BASE_URL` and shared `API_WEB_TOKEN`; browser code must never import it.

The request path is:

```text
Browser -> Astro server route/page -> Go API -> database
```

## Stack and Code Placement

- Use Astro for filesystem routing, layouts, content, server rendering, and page composition.
- Use Svelte for components that require client-side state or interaction. Do not hydrate static
  content without a user-facing need.
- Use the existing Tailwind and shared style setup before adding another styling system.
- Own Astro/Svelte ESLint and Prettier integration and the complete Stylelint configuration inside
  this module; consume only framework-neutral base configuration from `@heyblog/configs`.
- Express component and page styling with complete Tailwind CSS v4 utility strings. Do not use
  `@apply` or introduce semantic component selectors solely to reference them from `class`.
- Use canonical Tailwind CSS v4 utilities before arbitrary values. Prefer the built-in scale and
  custom-property shorthand, such as `size-4.5`, `bg-canvas/88`, and
  `duration-(--motion-base)`; reserve bracket syntax for values or selectors with no native form.
- Let `eslint-plugin-better-tailwindcss` enforce Tailwind correctness, canonical utilities, and the
  official class order. Keep its internal line-wrapping rule disabled because it conflicts with the
  Astro Prettier printer; Prettier owns source layout but not Tailwind class order.
- Name reusable Tailwind class-string constants with a `Class` or `Classes` suffix so the ESLint
  selector includes them.
- Keep theme registration and global element defaults in `src/styles`. Place selectors that cannot
  be expressed reliably as utilities next to their owning Astro component or layout.
- Aim to keep every source CSS file at or below 250 lines as a review guideline. Split larger files
  by ownership when practical; this limit is not an automated lint gate.
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
- As features are implemented, prefer these boundaries:
  - `src/pages`: Astro pages and same-origin HTTP endpoints.
  - `src/layouts`: page shells and cross-page layout composition.
  - `src/components`: focused Astro and Svelte presentation components.
  - `src/application/<feature>`: feature-specific orchestration and API adapters.
  - `src/shared`: genuinely cross-feature browser, server, UI, and integration utilities.
- Use `.server.ts` for server-only code and `.browser.ts` for browser-only code. Shared modules must
  not depend on either runtime implicitly.
- Split large feature files by responsibility. Do not reproduce uneven legacy directory structures.

## Rendering and HTTP Data Flow

- The application uses Astro server output with the Node standalone adapter for SSR and request
  interception. Keep the adapter, output mode, build, preview, and deployment behavior coherent
  when changing request-time rendering.
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
- Implement GET forwarding endpoints with `src/application/api/endpoint.server.ts` and keep their
  `src/pages` route files declarative. The shared adapter is intentionally GET-only; add mutation
  methods and body validation together only when an accepted route contract requires them.
- Declare each endpoint audience as `web-only` or `public`. Web-only endpoints require strict
  same-origin Fetch Metadata; public endpoints require an explicit route-local CORS policy.
- Allow only the required upstream path, method, headers, query fields, and body shape.
- Authenticate every upstream application request with `X-HeyBlog-Web-Token`; never accept that
  header from a browser request or expose its value in a response.
- Forward cookies and authentication headers only when the endpoint requires them. Preserve
  relevant `Set-Cookie` headers from the API response.
- Preserve meaningful upstream status codes and response bodies. Convert connection failures to a
  stable `502` response without exposing internal addresses or stack traces.
- Combine the incoming request cancellation signal with the endpoint timeout for every upstream
  request.
- Never forward an upstream `Location` header directly. Map accepted redirects to an explicit
  same-origin Web destination at the route boundary.
- Do not forward hop-by-hop headers or stale `content-length`, `transfer-encoding`, or
  `content-encoding` values when rebuilding a response.
- Web middleware may provide redirects and user experience guards, but the Go API remains the
  authority for authentication and authorization.
- Validate browser input at the web boundary for usability; the API must validate it again for
  correctness and security.

## Migration Guidance

- Before rebuilding a route, inspect its current requirements, application boundary, API forwarding
  contract, and focused tests together.
- Preserve accepted URLs, status handling, cookies, redirects, and user-visible behavior unless the
  current task or accepted HTTP contract changes them.
- Implement direct or browser-facing backend calls through the current same-origin HTTP boundary.
- Do not introduce Node database access, Fastify assumptions, or imports from database packages.

## Commands and Validation

Run commands from the repository root:

- `task web:dev`: start the Astro development server.
- `task web:check`: run Astro and TypeScript checks.
- `task web:test`: run focused Node tests for Web server infrastructure.
- `task web:lint`: run ESLint and Stylelint.
- `task web:format:check`: check formatting.
- `task web:build`: build the web application.
- `task web:content`: sync generated content from the `heyblog/.github` `main` branch.
- `task web:verify`: run all current offline web checks.
- `task container:build`: build the production container image after Dockerfile changes.

Run `task web:content` after initial checkout or when the remote content changes. Offline checks
consume the existing generated snapshot and do not update it.

Use the current Node test task for server adapters, authentication, proxying, rendering decisions,
or critical browser logic. Do not introduce a Vitest or Playwright setup without explaining why it
is the right choice.
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
