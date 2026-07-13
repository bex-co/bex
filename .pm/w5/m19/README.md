# w5 · m19 — Fix dashboard SSR authentication: forward the real Kratos session cookie

**Worker:** worker5 **Goal:** An authenticated dashboard page's SSR output contains real, correctly-scoped data — not an empty/unauthenticated shell that only self-corrects after client-side hydration. Today `factory.server.ts` sends a Bearer token from a cookie (`bex-dashboard-token`) that nothing in the app has ever set — dead scaffold code from `w5/m1` (2026-07-06), predating real Kratos auth — so every SSR-rendered authenticated GraphQL query gets an unauthenticated response. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                    | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Root-cause confirmation: trace every SSR-rendered route/loader that depends on `getClient()` and confirm they're silently getting unauthenticated responses today | 20m | —          |
| t002 | Fix `factory.server.ts`: forward the incoming request's session cookie (`ory_kratos_session` via the `Cookie` header) to bex-api instead of the dead `bex-dashboard-token` Bearer path | 30m | t001       |
| t003 | Audit SSR-dependent auth-gate logic (e.g. `__root.tsx` loaders) for correctness now that SSR queries actually authenticate — fix any redirect/guard logic that was compensating for the broken path | 30m | t002       |
| t004 | Confirm unauthenticated visitors still get correct public-route SSR behavior (no session ⇒ no `Cookie` forwarded, not a crash)             | 15m | t002       |
| t005 | Acceptance: view-source (JS disabled) an authenticated page and confirm real data is present in the initial SSR HTML, not just post-hydration | 20m | t003, t004 |

## Definition of done

An authenticated dashboard page's SSR output contains real, correctly-scoped data (verified with JS disabled / view-source), not an empty/unauthenticated shell that only self-corrects on hydration; unauthenticated SSR requests behave exactly as before (no crash, correct public-route rendering).

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones to work on` 2026-07-13 — a codebase TODO/FIXME sweep found `dashboard/src/common/apollo/factory.server.ts:33-44`'s own comment ("nothing sets the `bex-dashboard-token` cookie yet"), verified live: bex-api already accepts Kratos session cookies directly (`lego/backend/internal/api/auth.go:149,246` forwards the incoming `Cookie` header to Kratos `whoami`), so the fix is forwarding the real cookie, not new backend work.
- **Goal linkage:** dashboard correctness — the dashboard is bex's primary human surface (`docs/ADR008-vision.md`); this closes a live, silent authentication bug in the SSR path.
- **Expected outcome:** SSR-rendered authenticated pages return real data on first paint instead of an unauthenticated shell that quietly repairs itself after hydration.
- **Why now:** found live during a codebase sweep; the fix is small and well-understood (bex-api already supports the exact cookie the dashboard needs to forward) — the risk grows the longer dead scaffold code masquerades as working auth.
- **Render parity closing task: omitted** — internal dashboard rendering-pipeline fix, no REST/GraphQL/MCP/UI capability comparison applies.
