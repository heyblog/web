# Web API Layer Guidance

All file references are repository-root-relative; inherit `apps/web/AGENTS.md`.

- Own API DTOs, protocol parsing, HTTP calls, and request serialization. Do not import application
  state, drafts, presentation helpers, or integrations. Application maps editable state into a DTO
  before calling this layer.
- Shared site-card DTOs live in `apps/web/src/api/sites/site-card.types.ts`; homepage-specific
  fields live in `apps/web/src/api/home/home.types.ts`. Presentation dates and canonical page paths
  belong to application, not these DTO modules.
- `apps/web/src/api/transport/client.server.ts` handles typed server reads;
  `apps/web/src/api/transport/endpoint.server.ts` provides the declarative GET-only forwarding
  adapter. Preserve policy whitelists, audience/CORS checks, timeout plus incoming cancellation,
  request IDs, explicit response headers, and `no-store` behavior. Do not widen it for mutations.
- Web-only GETs use Fetch Metadata when present. If all Fetch Metadata headers are absent (as on
  plain HTTP LAN origins), require JSON Accept, a Referer matching the request URL's origin, and
  no conflicting Origin. Reject direct navigation, missing source evidence, and cross-origin
  requests. Do not special-case an IP address or replace these checks with public CORS.
- `apps/web/src/api/transport/client-ip.server.ts` forwards one validated trusted `X-Real-IP` as
  both address headers; do not extend a browser-supplied forwarding chain.
- Auth forwarding in `apps/web/src/api/auth/auth.server.ts` retains the Web token, incoming Cookie,
  manual redirects, and API Set-Cookie protocol. Safe local redirect destinations belong to the
  calling Web route/application flow. Never blindly relay upstream Location or transport headers.
- Browser adapters use only accepted same-origin routes. Preserve method, query ordering, body,
  signal, timeout, error code, and request count. No retries may be added to mutations during a
  structural refactor.
- Managed-user adapters in `apps/web/src/api/managed-users` parse account display DTOs and
  preserve form-encoded authorization POSTs with JSON response negotiation. The role and permission
  mutations remain separate PATCH calls upstream; never retry a mutation automatically. Tests:
  `apps/web/tests/managed-users.transport.test.ts`.
- Credential adapters own response parsing and raw failures, not labels or result-view state.
  Preserve same-origin credentials, no-store, bounded timeout, malformed-success rejection and
  partial-history response semantics. Validate management payloads/mutation metadata before auth
  forwarding. Never cache, log or persist presented key secrets; see
  `apps/web/src/api/api-keys` and `apps/web/tests/api-keys.validation.test.ts`.
- Submission adapters receive mapped DTOs, not editable drafts. Preserve the purpose-built GET/POST
  proxy's same-origin checks, JSON/body bounds, query forwarding, Web token, trusted client address,
  manual redirects, cancellation, request IDs and status/body/no-store semantics in
  `apps/web/src/api/site-submission/proxy.server.ts`. Management review uses authenticated auth
  forwarding, not the anonymous submission proxy. Keep lookup credentials in their accepted flow.
- Icon transport reads only the authenticated cached site-icon endpoint, validates bounded PNGs
  and profile-hash/ETag agreement, and distinguishes missing from unavailable. Do not forward
  browser credentials or fetch external icon URLs; see `apps/web/src/api/sites/site-icon.server.ts`.
- Sources/tests: `apps/web/tests/client.server.test.ts`, `apps/web/tests/endpoint.server.test.ts`,
  `apps/web/tests/client-ip.server.test.ts`, `apps/web/tests/auth.server.test.ts`, and feature tests.
  Mutation/proxy coverage: `apps/web/tests/api-keys.browser.test.ts`,
  `apps/web/tests/site-submission-proxy.server.test.ts`, and
  `apps/web/tests/site-submission.transport.test.ts`.
