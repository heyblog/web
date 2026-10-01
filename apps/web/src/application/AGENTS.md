# Web Application Layer Guidance

All file references are repository-root-relative; inherit `apps/web/AGENTS.md`.

- Own Web page flows and display models, not backend domain rules or raw HTTP adapters. Call
  `apps/web/src/api` and `apps/web/src/integrations`; do not move their protocols into this layer.
- `apps/web/src/application/auth` owns SSR session/page guards, safe local redirects, and prompt
  mapping. `apps/web/src/application/management` owns navigation. Neither replaces API permission
  checks.
- `apps/web/src/application/site-directory` interprets page URL state, default filters, and seed
  lifecycle. API query serialization belongs to `apps/web/src/api/sites`. Keep daily seeds and
  shuffle behavior unchanged; see `apps/web/tests/site-directory.shared.test.ts`.
- `apps/web/src/application/home` selects homepage content;
  `apps/web/src/application/site-profile` projects canonical paths and presentation dates.
  Reusable site DTOs do not belong to home.
- `apps/web/src/application/static-content` owns sorting, editor links, member grouping, and
  historical contributor projection. Astro collection access belongs to integrations. Sources and
  tests: `apps/web/src/application/static-content/static-content.models.ts`,
  `apps/web/tests/static-content.test.ts`, and `apps/web/tests/home-project-blogs.test.ts`.
- `apps/web/src/application/site-notice` owns notice display behavior.
- Preserve existing form state sequencing, error messages, URL behavior, and sensitive-result
  lifetime. Keep mutable SSR request state out of shared module scope.

## State and Lifecycle Boundaries

- API-key flows preserve busy/refreshing guards, dirty-form discard confirmation and failure
  sequencing. Keep issued secrets only in result-view memory, retain them after list-refresh
  failure and clear them when leaving the result. Partial update/rotation responses must not
  replace complete key history. Permission options, expiry/status and issue-vs-rotate rules live
  in `apps/web/src/application/api-keys` and `apps/web/tests/api-keys.model.test.ts`;
  state transitions are owned by `apps/web/src/application/api-keys/api-keys.controller.svelte.ts`.
- Submission flows own editable drafts, action-specific payload mapping, draft identities, step
  validation, availability cancellation and tag/snapshot selection. Preserve requested/drift/
  reviewer-correction/conflict views; map drafts before calling API and never expose credentials
  beyond the accepted flow. Evidence: `apps/web/tests/site-submission.browser.test.ts`,
  `apps/web/tests/site-submission.snapshot.browser.test.ts`, and
  `apps/web/tests/site-submission.validation.test.ts`.
- Random discovery uses exact classification display names and keeps preview Web-only with no
  navigation/countdown. Preserve normal-mode SSR links, timer, recovery/copy behavior and
  concurrent selection/options loading with operation-specific fallback. Generated links use the
  configured public origin, not the private API origin. `/site/go` and its selection endpoint
  retain no-store and random links remain non-prefetched/non-indexed; the legacy `/random` is a
  query-preserving 301 without its own explicit no-store header. Sources/tests:
  `apps/web/src/pages/random.ts`, `apps/web/src/application/site-go/site-go.server.ts`, and
  `apps/web/tests/site-go.test.ts`.
- OG flows own content projection, wrapping, rendering, QR and HTTP output; keep Node hashing
  separate from the content model and renderer/fonts server-only for dist-only deployment.
  Preserve UUID-to-short-ID redirect, canonical QR destination, icon/fallback rendering, error/
  redirect no-store and transient-icon-failure no-store versus normal shared caching. Rendering,
  layout or font changes require a template-version change in
  `apps/web/src/application/site-og/site-og.metadata.server.ts`; version inputs include icon hash
  and QR destination. Assets/template/cache details remain grounded in
  `apps/web/tests/site-og.test.ts`, `apps/web/tests/site-og.renderer.test.ts`,
  `apps/web/tests/site-og.icon.test.ts`, and `apps/web/standalone-og.test.mjs`.
