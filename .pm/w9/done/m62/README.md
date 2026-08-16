# w9 · m62 — Polling hygiene + hydration fetch-policy: stop paying for tabs nobody is looking at

**Worker:** worker9 **Goal:** close the dashboard's remaining ungated pollers (metrics, deploy detail, usage/billing) with the existing `skipPollWhenHidden` pattern, unwind the deploy-detail fetch waterfall, and stop loader/SSR-primed queries from immediately refetching on hydration — so background tabs go quiet and the m68 prefetch investment isn't thrown away on mount **Status:** done

## Results (2026-08-16)

- **Poll gating (t001/t003):** added `skipPollAttempt: skipPollWhenHidden` to the metrics stack (`use-metrics`, `use-datastore-metrics`, `use-month-to-date-bandwidth`), the usage/billing hooks (`use-resource-limits`, `use-usage`, `use-billing-onboarding`), and the `use-database-insights` `parameterOverrides` leg its four siblings already gated. A backgrounded Metrics/usage tab now fires zero polls.
- **Payment poll stop (t003):** the payment-required dialog polls readiness at 2s only while `open && !paymentMethodReady`, stopping the moment a method binds and re-arming on reopen — via adjust-state-during-render (no `setState`-in-effect), the codebase's sanctioned prop-change pattern.
- **Deploy-detail waterfall (t002):** header, timeline, and log panel now mount in parallel, each with its own skeleton, instead of the page blank-waiting on the header `deploy` query; the timeline/log queries `skip` until the deploy's window is known (a `?r=` range lets the log panel query immediately). The three deploy pollers are visibility-gated; terminal settling unchanged. Note: the window-dependent child queries still fire _after_ the header resolves — a genuine data dependency (they need `deploy.createdAt`), not artificial serialization — so the win is per-region skeletons + no page blank + the ranged-log parallel case.
- **Cache-first primed mount (t004):** new `PRIMED_FETCH_POLICY` (`common/lib/fetch-policy.ts`, `cache-first`) applied to the overview list hooks (`useServices`/`useDatabases`/`useKeyValues`/`useProjects`, all of which poll for freshness); the service/static detail primary (`use-server`) was already cache-first. An SSR/prefetch-primed navigation renders from the warm cache with no duplicate mount fetch; the m68 loaders will land on this seam.
- **Coverage (t006):** `polling.test.ts` (skipPollWhenHidden hidden/visible + `PRIMED_FETCH_POLICY` = cache-first) + wiring guards on `useMetrics` (gate) and `useServices` (primed policy). **2,136 dashboard tests green, ESLint + tsc clean.**

## Tasks (in order)

| id   | title                                                                | est | depends_on       |
| ---- | -------------------------------------------------------------------- | --- | ---------------- |
| t001 | Visibility-gate the metrics pollers — **DONE**                       | 30m | —                |
| t002 | Deploy detail: gate pollers + unwind the 2-hop waterfall — **DONE**  | 1h  | —                |
| t003 | Usage/billing pollers: gate + stop the 2s poll once satisfied — **DONE** | 30m | —            |
| t004 | Hydration fetch-policy: loader-primed queries mount cache-first — **DONE** | 1h | t001, t002, t003 |
| t005 | Simplify — **DONE** (clean diff; shared helper; lint 0)             | 20m | t004             |
| t006 | Test coverage — **DONE**                                            | 40m | t004             |
| t007 | Closeout — **DONE**                                                 | 10m | t006             |

## Definition of done

- With the tab hidden, an open Metrics tab, an in-flight deploy detail page, and the usage/billing surfaces fire **zero** poll requests (verified in devtools network with the tab backgrounded); polling resumes on visibility.
- The deploy detail page starts its timeline + log queries in parallel with the header `deploy` query (each region shows its own skeleton) instead of blank-waiting on the header round trip; the payment-required dialog's 2s poll stops once `paymentMethodReady` flips.
- A query primed by a route loader / SSR dehydration renders from cache on mount **without** an immediate duplicate network request (verified in devtools: hover-prefetch or SSR → navigate → no refetch), while background freshness still arrives via the existing poll intervals.
- The one inconsistent `database-insights` leg (`parameterOverrides`) is gated like its four siblings.
- All dashboard tests green; rendered results identical to before (only when/whether requests fire changes).

## Source + Goal linkage

- **Source:** perf sweep 2026-08-16 (data-fetching leg of the w5/m67–m69 follow-on, handed to w9 by user direction). Evidence: the shared resource hooks already pass `skipPollAttempt: skipPollWhenHidden` (`dashboard/src/common/lib/polling.ts:16`) but the metrics stack does not (`use-metrics.ts:150` — ~10 queries × 30s per open Metrics tab; `use-datastore-metrics.ts:57`; `use-month-to-date-bandwidth.ts:24`), nor do the deploy-detail hooks (`use-deploy.ts:54` 3s, `use-deploy-timeline.ts:49` 3s, `use-deploy-logs.ts:74` 5s ×3 — ~5 req/3–5s from a hidden tab during a build) nor the usage hooks (`payment-required.tsx:56` polls at **2s**; `use-resource-limits.ts:45`; `use-usage.ts:101`; `use-billing-onboarding.ts:90`; `use-database-insights.ts:78` one leg of five). Separately, feature hooks default to `fetchPolicy: "cache-and-network"`, which always refires on mount — partially defeating the m68 loader-prefetch and the SSR dehydration in `router.tsx:35-44`. Deploy detail additionally gates its child queries on the header query (`deploy-detail-page.tsx:76`), a real 2-hop waterfall.
- **Goal linkage:** ADR008 vision — the dashboard is the human surface; this is the request-count/latency complement to w9/m68 (which controls when data loads) and directly protects m68's outcome (a prefetched cache that is then ignored on mount is wasted work). Fewer background requests also cut bex-api/Kratos load (every session request pays an uncached whoami).
- **Expected outcome:** background tabs go network-silent; deploy pages render all regions in parallel; loader/SSR-primed navigations render warm with zero duplicate fetches.
- **Why now:** w9/m68 (relocated here) is about to move queries into loaders — t004 must land with/after it or its DoD ("renders with data on mount") is undermined by the unconditional refetch; the polling edits are independent, tiny, and use an existing in-repo pattern.
- **Render parity task:** **omitted** — changes when/whether requests fire, not the rendered result, error shapes, or any REST/GraphQL/MCP/UI data surface; no render.com behavior to compare (same rationale as w9/m68).
