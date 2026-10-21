# w5 · m151 — Render edge caching: `serviceDetails.cache.profile`

**Worker:** worker5 **Goal:** a web service's edge cache profile works as on Render, replacing the `—` row w5/m146 recorded without user sign-off. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Accept and read back serviceDetails.cache.profile on every surface — **DONE** | 1h30m | — |
| t002 | Design the edge cache: profile semantics, scope and purge — **DONE** | 1h | — |
| t003 | Implement the edge cache behind the profile — **DONE** | 2h | t002 |
| t004 | Dashboard control and the ADR018 row — **DONE** | 45m | t001, t003 |
| t005 | Render parity — **DONE** | 30m | t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

Total: **8 tasks, about 7h**.

## Definition of done

- An `origin-controlled` service serves its second request for a `Cache-Control: max-age` asset from the edge, a deploy purges it, and a `no-cache` service is never cached.
- The profile round-trips on REST, GraphQL, MCP and the dashboard.

## Source + Goal linkage

- **Source:** fifth `/pm-brainstorm for w5` (2026-10-20), all five items chosen by the user the same day after `2163b27c7` made Render parity the top principle (a divergence stands only with ADR018-recorded user sign-off; never park parity on an optional decision); w5/m146 refused `serviceDetails.cache` and added an ADR018 `—` row without sign-off.
- **Goal linkage:** ADR008 Render parity first; ADR006/ADR018.
- **Expected outcome:** tenants get Render's edge caching, and the unsigned divergence is gone.
- **Why now:** the unsigned `—` row breaks the new rule today.
- **Render parity:** included: new field on REST/GraphQL/MCP and the dashboard.

## Closeout (2026-10-21)

- **The cache.** `lego/operator/internal/edgecache` implements Render's web-service edge caching as documented (render.com/docs/web-service-caching). It has 13 tests under `-race`.
  - Profiles: common static files or all files. TTLs: the status defaults, `CDN-Cache-Control` over `Cache-Control`, `s-maxage` over `max-age`.
  - Responses with `no-store`/`private`/`no-cache` or `Set-Cookie` bypass. `CF-Cache-Status` is set, and concurrent misses collapse into one request.
  - Hardening from the review: only `GET` fills; validators are stripped and only Render's statuses are stored; `Accept-Encoding` is pinned to the key; every `Cache-Control`/`Vary` line counts; event streams pass through and release waiters; buffering fills are bounded.
- **Operator.** A paid web service with a caching profile gets an `edge-cache` sidecar (bex's own image, `/edge-cache`), and the Service targets it, falling back while old pods lack it. The app keeps its own probes, and the sidecar drains with the app. A purge annotation rolls the pods. Envtests cover the paid service, purge, turning caching off and the free plan.
- **API.** `PATCH serviceDetails.cache` and the web-service read view (create omits it, as Render's spec does). `POST /v1/services/{id}/cache/purge` answers 202 with no body. GraphQL has `cacheProfile`, `setCacheProfile` and `purgeServiceCache`. Free plans and non-web types are refused; a free plan reads as `no-cache` and refuses purge. API guard tables are updated (verb counts, scope matrix, route inventory, events excused, release-identity class).
- **Dashboard.** A Settings **Edge Caching** section with the three options and **Purge cache**, plus the free-plan message, i18n and tests. Lint passes, and 5414 tests pass.
- **Docs.** ADR018 replaces both unsigned `—` rows (edge caching, cache purge) with ✅ rows, and ADR004 § Edge caching records the per-pod mechanism.
- **Deviations from the DoD:**
  - MCP is `—` because Render's MCP has no cache tool, so bex matches it.
  - The live "second request served from the edge" check ran as proxy tests and envtest projection, not on dev-5. dev-5 reuses the shared operator, and the sidecar binary ships in the image with this change.
  - The dashboard skeleton was sized by measuring the ready section in headless Chromium at 390 and 1440px; no side-by-side pending-vs-ready check was run.
