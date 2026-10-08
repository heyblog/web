# Web Module Guidance

All file references are repository-root-relative. This file refines `AGENTS.md` for `apps/web`.

## Sources and Ownership

- Dependencies and compiler configuration: `apps/web/package.json`, `apps/web/svelte.config.ts`,
  `apps/web/astro.config.ts`, and `apps/web/tsconfig.json`.
- Commands: `apps/web/mise.toml`; release version: root `mise.toml#vars.project_version`.
- Site metadata: `apps/web/src/site.config.ts`; content schemas: `apps/web/src/content.config.ts`.
- SEO policy is shared in `src/shared/indexing.ts`; runtime sitemap composition/cache belongs to
  `src/application/seo`, API sitemap DTOs/reads to `src/api/sitemap`, and Astro content reads remain
  in integrations. Sitemap uses only public data, 60-second snapshots, bounded cursor reads and
  complete XML responses; no incoming credentials enter its cache. `robots.txt` and `llms.txt`
  are build outputs from `src/shared/crawlers.ts`. Content update times are optional source dates;
  never substitute build/file timestamps or edit the generated content snapshot.
- Announcement transport is owned by `src/api/announcements`; editor state, inline-markup
  projection, draft field validation, banner dismissal and carousel playback by `src/application/announcements`. `marked` parses only
  inline source into escaped Svelte nodes; never render announcement source as raw HTML or MDX.
  `scripts/sync-web-content.mjs` owns the generated `apps/web/contents` snapshot. Offline checks
  consume that snapshot rather than refresh it.
- Own routing, rendering, browser state, presentation, and the same-origin HTTP boundary. Domain
  rules, authoritative authorization, database schemas, and connection pools belong to `apps/api`.
  Communicate with that application only over HTTP, using DTOs rather than persistence models.
- Runtime environment reads belong to `apps/web/src/config.server.ts`, including private upstream
  origin, shared Web token, and optional GitHub token. Browser code must not import this module.
- Build provenance belongs to `apps/web/src/integrations/build/build-metadata.ts` and
  `apps/web/Dockerfile`. Build-time inputs are not runtime environment configuration. Preserve
  repository-root discovery and the dist-only Node standalone deployment.
- Development loads `qrcode` through Node's CommonJS interop; production keeps it bundled for
  dist-only deployment. Do not enable SSR dependency prebundling: Vite version queries can split
  Astro's development environment registry. `apps/web/tests/dev-server.test.ts` covers fresh dev
  rendering of prerendered/SSR headers and decodable QR images.

## Directory Boundaries

| Directory | Responsibility |
| --- | --- |
| `apps/web/src/pages` | Astro routes, route-specific HTTP policy, and thin composition |
| `apps/web/src/layouts`, `apps/web/src/components` | Page shells and presentation components |
| `apps/web/src/api` | HTTP transport, protocol parsing, request serialization, and API DTOs |
| `apps/web/src/application` | Page flows, form/state transitions, display projections, and Web guards |
| `apps/web/src/integrations` | External GitHub access, Astro content reads, and build integration |
| `apps/web/src/shared` | Genuinely cross-feature helpers, not a second API or integration layer |
| `apps/web/src/styles` | Theme registration and global element defaults |

Application may call API and integrations; neither layer imports application. These are ownership
partitions within Web, not independent workspace modules. Only these three layers have local
guidance: `apps/web/src/api/AGENTS.md`, `apps/web/src/application/AGENTS.md`, and
`apps/web/src/integrations/AGENTS.md`. Read the relevant layer guidance when editing or changing
its callers; feature directories use source contracts and focused tests rather than more AGENTS
files. Use `.server.ts` and `.browser.ts` for runtime-specific modules; shared
modules must not implicitly depend on either runtime. Browser imports must not reach Node modules,
private configuration, or server assets. `apps/web/tests/architecture.test.ts` checks the TypeScript
layer and browser-module boundaries.

## Rendering and Same-Origin Safety

- Database backup transport lives in `src/api/database-backup`; file selection, confirmation and
  safe messages in `src/application/database-backup`. `/management/database-backup` is SYS_ADMIN-only.
  Purpose-built POST forwarding streams attachments and multipart files without parsing backup data
  in Web. Keep Cookie/Web-token authorization, same-origin metadata, private no-store, 512 MiB file
  bounds, 513 MiB multipart bounds, 30-minute cancellation and refresh cookies. Inspection contains
  counts and administrator mapping only. Never retry restoration automatically; an uncertain result
  requires fresh inspection. Browser state holds selected files only until navigation.

- Site homepage navigation uses `application/site-outbound` and `/site/out/{shortId}`; external
  resources and management snapshots use the same link attributes without click tracking. Links
  isolate the opener, disclose only the origin and add the canonical public hostname as UTM source,
  including development. `api/site-metrics` owns bounded same-origin impression POSTs and private
  outbound resolution. PublicLayout observes qualified displays with document-local deduplication;
  no persistent browser identity is used. Random manual navigation stops the automatic timer.

- Tag concepts share one slug across admin-confirmed names. Selections carry concept `id` and
  optional `label_id`, preserving chosen names on content. Selector rows use label IDs, search expands
  confirmed synonym groups, and duplicate concepts cannot be chosen as tertiary tags. Directory URL
  label parameters restore presentation only; canonical slug filters determine results.
- Tag dictionaries use global tag identity; classification roles belong to paths and selections, so
  PRIMARY and SECONDARY may share an ID. Batch slug jobs retain editable previews in component memory,
  recover persisted tasks through the API, and cancel polling on unmount. Candidate edits and applies
  use task revisions; only selected successful items are submitted. Conflicting candidates remain
  `needs_confirmation` until an administrator maps synonyms or supplies an independent slug.
- Tag management and system settings use the purpose-built authenticated boundary in
  `src/api/taxonomy`. Payload validators reject arbitrary prompts, models, and endpoints; mutations
  require matching Origin and same-origin fetch metadata. Only SYS_ADMIN changes global AI settings.
  Provider credentials stay entirely in the API. Generation and model requests have a 25s browser
  deadline and 20s Web deadline; throttling headers survive forwarding. User-facing failures map
  stable codes through `src/application/taxonomy`, never raw upstream diagnostics.
  Review and review-draft forwarding allows 40s for backend automatic slug preparation. Batch
  candidate inputs are limited to 128 characters and become editable after generation completes;
  browser polling and its timers stop when the component unmounts.

- Astro owns filesystem routing, SSR, prerendering, middleware, and island placement. Keep server
  output and the Node standalone adapter coherent with build and deployment tasks.
- Preserve explicit client prebundling of Tabler icons, `marked`, and graph Worker imports
  (`d3-force-3d`, `graphology`, `graphology-shortest-path/unweighted.js`). Worker imports are not
  covered by the initial page scan; late optimization can invalidate Astro's dev toolbar module.
  `apps/web/tests/dev-server.test.ts` verifies runtime scripts after those imports load.
- Svelte may render without hydration. Add a client directive only when browser state or
  interaction needs it; preserve SSR-only composition and stable island fallbacks.
- Decide SSR, prerendering, and caching per route. Personalized server islands retain
  `private, no-store`; preserve existing public-read cache policy rather than applying one policy
  to every route.
- Browser backend calls enter through purpose-built same-origin routes. Do not expose the internal
  API origin or secrets through `PUBLIC_*` values or add an unrestricted proxy.
- Friend-link graphs use `/api/site-graph` and `/api/site-graph/{identifier}`. Graph DTOs belong to
  `src/api/sites`; selection, layout Worker, pathfinding, and renderer lifecycle belong to
  `src/application/site-graph`. Three.js renders the global graph; d3-force-3d and graphology
  run layout and shortest paths in the Worker. Details lazy-load the one-hop SVG graph.
  Dispose Workers and GPU resources when leaving; graph data must not enter persistent storage.
  Graph query and layout Workers are independent. Global visibility filters leave the complete
  search/path topology intact; isolates use a separate stable layout. The fullscreen workspace
  preserves its canvas and view state across separate webpage and native fullscreen modes. Native
  fullscreen failures keep the current mode and offer webpage fullscreen. SVG and WebGL share
  pointer gestures, anchored zoom and CSS-pixel pan. Relative zoom ranges from 0.25 to 1024. Both renderers share
  viewport-aware name/domain labels, density hysteresis, and collision placement. Inline
  touch interaction defaults to page scrolling and has an explicit graph-operation mode.
- Authentication transport lives in `apps/web/src/api/auth`; session checks, safe local `next`
  paths, and page redirects live in `apps/web/src/application/auth`. Web guards improve experience;
  the Go API remains the authorization authority.
- `/dashboard` owns the user workbench, with separate sites, claims, submissions, and security
  routes. `apps/web/src/api/site-management` owns account/claim DTOs and the bounded, authenticated
  route whitelist for `/api/account` and `/api/site-management`; never widen it to arbitrary paths.
- Preserve accepted HTTP methods, URLs, statuses, Cookie/Set-Cookie handling, cancellation,
  timeouts, redirects, request counts, and failure mappings. Consult the API layer rules and
  route's focused tests before changing forwarding behavior.

## Styling and Skill Routing

- Use existing Tailwind CSS v4 utilities and Aria tokens. Do not use `@apply` or add semantic
  component selectors solely to reference a third-party snippet. Prefer canonical utilities and
  custom-property shorthand; use arbitrary values only when needed.
- Reusable class strings use a `Class` or `Classes` suffix. ESLint owns Tailwind correctness and
  class order; Prettier owns layout. Web composes Astro/Svelte tooling and Stylelint over the
  framework-neutral `packages/node/configs` exports.
- UI, styling, layout, accessibility, and motion work must use `aria-design-system` from
  `skills/aria-design-system`. Logic-only, API, and pure content-data changes do not activate it.
- Use `astro-project` from `skills/astro-project` for Astro routing, SSR, prerendering, islands,
  content, and integration work. Use `svelte-project` from `skills/svelte-project` for Svelte
  components/modules, runes, reactivity, context, or hydration behavior.
- `transitions-dev` activates on explicit invocation or clear requests to add/select animation.
  `transitions-polish` activates on explicit invocation or clear tuning of existing animation.
  Significant animation intent includes motion, transition, entrance/exit, expansion/collapse,
  easing, spring, stagger, loader animation, animation performance, or reduced-motion behavior.
  Use dev then polish when both creation and tuning are requested. Generic UI polish, CSS edits,
  or hover styling alone do not activate either.
- Every animation task also loads Aria. Aria has final priority for tokens, accessibility, visual
  rules, and reduced motion. Transition skills supply patterns only: do not copy `_root.css`,
  create parallel global tokens, or introduce snippet-only selectors. Map patterns to existing
  state, Aria tokens, and Tailwind; preserve or add `prefers-reduced-motion` handling.
- Use `brandkit` or `imagegen-frontend-web` only on explicit requests for branding or design
  reference images. Use `playwright-cli` only for requested browser automation or when lower-level
  verification cannot establish correctness. Do not implicitly install tools or MCP dependencies.

## Validation

Run from the repository root using `apps/web/mise.toml`:

- `mise run //apps/web:test`: focused Node tests for transport and critical browser/server logic.
  Test files run serially because development-server tests share Astro's root `.astro` directory.
- `mise run //apps/web:check`: formatting, ESLint/Stylelint, and Astro/TypeScript checks.
- `mise run //apps/web:verify`: tests, checks, and build.
- `mise run //apps/web:dev` or `mise run //apps/web:preview`: task-managed development or build
  preview when needed.
- `mise run //apps/web:prepare`: explicit generated-content refresh, not part of offline validation.

Use existing test tooling. For structure-only changes preserve templates, classes, events,
hydration directives, text, assets, and motion; compare affected browser states when tests alone
cannot prove equivalence. Check the route's rendering/cache choice, same-origin forwarding,
browser import graph, and relevant feature tests before completion.

OG image generation reads `/sites/id/{identifier}/metadata` so image requests do not increment public display metrics.
