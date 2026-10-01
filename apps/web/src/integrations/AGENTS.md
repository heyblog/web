# Web Integrations Layer Guidance

All file references are repository-root-relative; inherit `apps/web/AGENTS.md`.

- Own external-source access and framework/build adapters. Do not depend on application or API
  feature state. Keep presentation sorting, grouping, and page flow in application.
- `apps/web/src/integrations/content/content.server.ts` owns Astro collection/entry reads. Preserve
  collection names, filters, missing-index failures, and reference resolution; route rendering and
  static paths remain with their Astro pages.
- `apps/web/src/integrations/github-avatar` and `apps/web/src/integrations/github-contributors`
  own GitHub fetches, cache lifetime, validation, and in-flight request coalescing. Preserve cache
  keys, fallback behavior, cancellation, and source limits. Optional credentials come from
  `apps/web/src/config.server.ts`, not browser configuration.
- `apps/web/src/integrations/build/build-metadata.ts` reads compile-time environment/Git metadata.
  Preserve its repository-root resolution after moves, version validation, fallback precedence,
  and virtual build constants; do not route build-only inputs through runtime configuration.
- Validation evidence lives in `apps/web/tests/github-avatar.server.test.ts`,
  `apps/web/tests/github-contributors.test.ts`, and `apps/web/tests/build-metadata.test.ts`.
