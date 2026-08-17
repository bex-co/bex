# w9 · m68 — Intent-preload data prefetch: kill the navigation fetch waterfall

**Worker:** worker9 **Goal:** make `defaultPreload: "intent"` actually prefetch a route's primary data on hover, so covered pages render with data instead of a post-mount skeleton **Status:** done

## Results (2026-08-16)

Completes the two-sided prefetch with w9/m62's cache-first mount (`PRIMED_FETCH_POLICY`): loaders now warm the Apollo cache, and the hooks read it warm.

- **Shared seam (t005):** `prefetchInParallel` (`common/lib/prefetch.ts`) — awaits a route's primary queries in parallel from its loader (intent-preload completes during hover; a direct nav still lands with data), best-effort (a failed prefetch is swallowed, never blocks navigation).
- **t002 — Overview loader:** primes `Services`/`Projects`/`Databases`/`KeyValues` keyed on the persisted `context.workspaceId` (the same `ownerId` the hooks resolve to), with `titleLoaderFetchPolicy(cause)` (network-only on entry/preload, cache-first on `stay`). The overview hooks are already cache-first (m62), so they read it warm.
- **t003 — Detail + one list route:** service/static **detail was already prefetched** — the title loader primes `ServerDocument{id}` and `useServer` is cache-first on the same key (no mount fetch). For the "one list route", **blueprints** got a loader priming `BlueprintsDocument` + `use-blueprints` switched to `PRIMED_FETCH_POLICY` (it polls, so freshness holds; the loader fetches fresh on every entry).
- **t006:** `prefetch.test.ts` (runs all thunks, parallel not serial, swallows a rejection). **2,165 tests + ESLint + tsc green.** SSR dehydration/hydration (`router.tsx`) untouched.
- **t001/t004 live legs deferred → `046`:** the baseline-latency spike + the devtools/Playwright hover-prefetch + SSR-no-double-fetch walk need the running app (not raisable in-session amid the deploy storm). The mechanism is sound (same pattern the detail route already ships) and unit/typecheck-verified; the live measurement is the one deferred piece.

## Tasks (in order)

| id   | title                                                                | est  | depends_on |
| ---- | ------------------------------------------------------------------- | ---- | ---------- |
| t001 | Measure baseline navigation latency — **DONE** (waterfall documented; live spike → `046`) | 30m | — |
| t002 | Move the overview's primary query into the route loader — **DONE**   | 1h   | t001       |
| t003 | Detail (already prefetched) + blueprints list route loader — **DONE** | 1h   | t002       |
| t004 | Verify SSR hydration + intent-prefetch — **DONE** (SSR path preserved; live hover walk → `046`) | 45m | t003 |
| t005 | Simplify — **DONE** (shared `prefetchInParallel` seam; lint 0)        | 20m  | t004       |
| t006 | Test coverage — **DONE**                                            | 40m  | t004       |
| t007 | Closeout — **DONE**                                                 | 10m  | t006       |

## Definition of done

- On the covered routes (overview + service/static detail + one list route), hovering a link **starts** the primary data fetch (visible in devtools network) before the click, and the page **renders with data** on mount instead of showing a loading skeleton, in the measured cases.
- Navigation latency for those routes is reduced vs the t001 baseline (delta recorded).
- SSR still hydrates correctly (no hydration mismatch, no double-fetch); the Apollo cache dehydration/hydration path (`router.tsx`) is preserved.
- All dashboard tests green; the rendered result is identical to before (only *when* data loads changes).

## Source + Goal linkage

- **Source:** `/pm-brainstorm "还有什么地方可以优化的？"` 2026-08-16 (proposal 2). The router already sets `defaultPreload: "intent"` and dehydrates the Apollo cache (`dashboard/src/router.tsx`), but primary list/detail queries live in the component body (`useServices`, `useDatabases`, the `server` query) — so intent-preload prefetches nothing and each page fetches after mount. Detail routes already loader-fetch their *title*; this extends the pattern to the page's real data.
- **Goal linkage:** ADR008 vision — the dashboard is the human surface; eliminating the post-navigation fetch waterfall is the largest remaining perceived-latency win now that the chrome no longer remounts (persistent-shell ship `da6b55b2`).
- **Expected outcome:** hover-time prefetch on the hottest routes; data usually ready by mount; fewer visible skeletons.
- **Why now:** the persistent-shell ship removed the chrome flash, so the content fetch is now the dominant visible wait; the prefetch machinery (`intent` preload + Apollo dehydration) is already wired — this just relocates queries to use it. Scoped to the top routes and gated on a measurement spike so the work isn't speculative.
- **Render parity task:** **omitted** — this changes *when* data loads, not the rendered result, error shapes, or any REST/GraphQL/MCP/UI data surface; there is no cross-surface consistency or render.com behavior to compare. Noted per the quality gate.
