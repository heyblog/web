# Web Module Guidance

This file refines the repository-level `AGENTS.md` for `apps/web`.

## Scope and Sources of Truth

- `apps/web` is the browser-facing Astro and Svelte application.
- The Web application inherits the project release version from
  `mise.toml#vars.project_version`; its `package.json` does not declare an independent version.
- Treat `package.json`, `astro.config.ts`, `src/site.config.ts`, `svelte.config.ts`, and
  `mise.toml` as the current dependency, runtime, site metadata, and command truth.
- Treat `apps/web/contents` as a generated snapshot of the `contents/` directory from the
  `heyblog/.github` repository. `mise run //apps/web:prepare` resolves the remote `main` branch to a commit,
  requires every source declared in `content-sources.mjs` and consumed by `content.config.ts` to be
  non-empty, downloads the files from that immutable commit, and records the resolved source in
  `.source-revision`.
- Browser requests that require backend data enter through this application, which communicates
  with the Go API over HTTP.
- Treat current task requirements, accepted HTTP contracts, current routes, and tests as
  migration behavior truth.
- `apps/web/Dockerfile` uses the repository root as its Docker build context and invokes the module
  mise task; Web module commands execute relative to `apps/web`. The production image copies the
  default Astro `dist` output without workspace-wide development dependencies.
- Footer build provenance is compile-time metadata. `WEB_BUILD_COMMIT`, `WEB_BUILD_REF`,
  `WEB_BUILD_COMMIT_TIME`, `WEB_BUILD_REPOSITORY_URL`, and `WEB_BUILD_TIME` are accepted only by the
  Docker builder stage; `mise run container:build` derives them from the host checkout because `.git`
  is excluded from the Docker context. They do not change `src/config.server.ts` ownership of Web
  runtime environment configuration and must not be retained in the runner image.
- `WEB_BUILD_VERSION` is internal compile-time metadata derived from
  `mise.toml#vars.project_version`; callers do not set it independently.

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

Authentication forms and OAuth callbacks enter through the same-origin `/auth/[...path]` Astro
gateway. Forward only the shared Web token, the incoming Cookie header, and API Set-Cookie headers;
forward the single validated `X-Real-IP` value supplied by the trusted production reverse proxy as
both `X-Real-IP` and `X-Forwarded-For`, and never extend a browser-supplied forwarding chain. Never
expose API secrets or authentication tokens to browser JavaScript. Protected pages read
`/auth/me` during SSR and redirect unauthenticated users to `/login` with a sanitized local `next`
path. Management pages additionally enforce the API-provided role and permission snapshot.

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
- Keep public content pages prerendered when only a small personalized region needs request state.
  Render that region as a Server Island with a layout-stable fallback, and mark personalized island
  responses `private, no-store`.
- Browser mutations, authenticated reads, and live refreshes go through same-origin Astro routes or
  actions. Do not expose the internal API base URL to browser code.
- Site lifecycle forms live under `src/components/site-submission`, with browser payload mapping and
  the purpose-built same-origin proxy under `src/application/site-submission`. Public submission
  routes use `/api/site-submissions/*`; management review pages use the authenticated server API
  boundary and preserve the requested, drift, reviewer-correction, and conflict diff views.
- Keep the API base URL in server-only configuration. Never place internal URLs or secrets in
  `PUBLIC_*` variables.
- Define cache behavior with each data path. Authentication and mutations default to `no-store`;
  cache public reads only with a clear invalidation strategy.
- For live or partial updates, refresh the affected data region instead of reloading the full page.

API credential management uses a client list with a modal detail sheet and same-origin endpoints
under `/management/api-keys`. `src/application/api-keys` owns typed response parsing, request
failures, permission options, key status derivation, and per-island operation state. Internal and
external clients default to `example.call`; only internal clients may select `data_import.write`.
Enabled clients without an active key can issue a new key; otherwise use rotation. Keep one-time
secrets in the result view's memory only and preserve them if a subsequent list refresh fails.
Metadata updates and rotation responses do not contain complete key history; refresh the list
without replacing existing history with those partial results.

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

Site share images use the public SSR `/og/site/[identifier].png` route. Identifiers follow the
site-page short-ID/UUID policy; UUID requests redirect to the short-ID image, and custom IDs have
no image lookup route. `src/application/site-og` owns metadata, wrapping, rendering, and responses.
Cards use the visual language of `public/og-default.svg` and include the site's two-level category,
cached site icon (or first-grapheme fallback), and a QR code for the canonical short-ID page.
The small HeyBlog mark is a footer signature. The icon adapter calls only the authenticated API
`/sites/id/{shortId}/icon` endpoint; it checks the PNG bounds and the profile's `iconHash` against
the icon ETag. Transient icon failures still render a fallback card with `no-store`.
The server-only renderer embeds `@resvg/resvg-wasm` and the Noto Sans SC fonts from
`src/assets/site-og`; their license ships in `public/licenses/noto-sans-sc.txt`. Keep these assets
out of browser bundles and preserve dist-only deployment. Bump the template version in
`site-og.model.ts` whenever rendering, layout, or fonts change; visible content also versions image
URLs, including the icon hash and QR destination. Successful images allow five minutes of shared caching and one minute of stale revalidation;
errors and redirects are not cached. `mise run //apps/web:smoke` includes isolated dist-only HTML/PNG checks.

Run commands from the repository root:

- `mise run //apps/web:dev`: start the Astro development server.
- `mise run //apps/web:typecheck`: run Astro and TypeScript checks.
- `mise run //apps/web:check`: run Web formatting, lint, and type checks.
- `mise run //apps/web:test`: run focused Node tests for Web server infrastructure.
- `mise run //apps/web:lint`: run ESLint and Stylelint.
- `mise run //apps/web:format:check`: check formatting.
- `mise run //apps/web:build`: invoke the Web build from the repository root; the module command runs in
  `apps/web`.
- `mise run //apps/web:smoke`: start the built standalone server on an ephemeral port and verify it serves a
  request successfully.
- `mise run //apps/web:prepare`: sync generated content from the `heyblog/.github` `main` branch.
- `mise run //apps/web:verify`: run all current offline web checks.
- `mise run container:build`: build the production container image after Dockerfile changes.

Run `mise run //apps/web:prepare` after initial checkout or when the remote content changes. Offline checks
consume the existing generated snapshot and do not update it.

Use the current Node test task for server adapters, authentication, proxying, rendering decisions,
or critical browser logic. Do not introduce a Vitest or Playwright setup without explaining why it
is the right choice.
Use browser-based end-to-end tests only when requested or when a critical interaction cannot be
verified reliably at a lower level.

## Background Development Server

Run a long-lived Astro development server in background mode:

```bash
mise --cd apps/web -E development exec -- pnpm exec astro dev --background
```

Manage it from the repository root:

- `mise --cd apps/web exec -- pnpm exec astro dev status`
- `mise --cd apps/web exec -- pnpm exec astro dev logs`
- `mise --cd apps/web exec -- pnpm exec astro dev stop`

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
- Run `mise run //apps/web:verify` and any focused tests introduced by the change.
