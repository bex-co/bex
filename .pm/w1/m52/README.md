# w1 · m52 — Zero-error deploy rolls: bex-api drain + dashboard roll resilience

**Worker:** worker1 **Goal:** a routine `deploy.yml` roll of bex-api + the dashboard is invisible to users — no failed API calls, no "Couldn't load resources", no SSR error page, no stranded error state that needs a manual reload. **Status:** todo

## The incident (evidence)

2026-07-18 06:54 UTC: the deploy run for `2c9a7143` finished at 06:54:12 and replaced both bex-api pods (RS `55c5498c6c` → `5b6db6f948`, pod starts 06:54:46 / 06:55:00), with a `FailedScheduling` stall in between (anti-affinity across only 3 platform nodes). The user was moving a service in/out of project `prj-d9dgeo0bd9nc73a0vh1g` at exactly that moment — the audit log shows three `projects.SetServices` calls at 06:54:43 / 06:54:49 / 06:55:09, **all allowed and committed** — yet the dashboard showed move-error toasts and `/project/prj-…` stranded on "Couldn't load resources" until a manual reload, because the route loader's failed query is dehydrated as a persistent `state: "error"` (`dashboard/src/common/lib/document-head/index.ts` `loadRouteResource`). Five deploy runs rolled prod between 06:22 and 07:03 that morning — every roll is currently a user-visible error window. The dashboard half (post-roll SSR error page, stale error `<title>`) was already filed as inbox `036` at the m51 closeout; folded in here as t002.

## Tasks (in order)

| id   | title                                                                        | est | depends_on       |
| ---- | ---------------------------------------------------------------------------- | --- | ---------------- |
| t001 | bex-api pre-shutdown drain window + rollout strategy hardening                | 45m | —                |
| t002 | Dashboard roll: readiness gated on upstream reachability + post-roll SSR error page (folds `036`) | 60m | —                |
| t003 | Dashboard client resilience: retry transient reads, re-run SSR-errored loaders on hydration | 45m | —                |
| t004 | Controlled-roll end-to-end verification under synthetic load                  | 45m | t001, t002, t003 |
| t005 | Render parity check                                                           | 20m | t004             |
| t006 | Simplify                                                                      | 30m | t005             |
| t007 | Test coverage                                                                 | 45m | t005             |
| t008 | Closeout                                                                      | 15m | t007             |

## Definition of done

Two consecutive controlled rolls (bex-api and the dashboard, e.g. `kubectl rollout restart`) under continuous synthetic load — an API request loop against api.bex.co and repeated full-page dashboard loads — complete with **zero** non-2xx responses and **zero** served error pages. A dashboard page held open through a roll self-recovers to data without a manual reload (no stranded "Couldn't load resources"), and the post-roll first hit renders normally (no "Something went wrong" SSR page, no lingering stale error `<title>`) — the `036` symptom is gone. Evidence (timestamps, request-loop counts, pod transitions) recorded at closeout.

## Source + Goal linkage

- **Source:** 2026-07-18 prod incident (user report: move-to-project errors + "Couldn't load resources" on `/project/prj-d9dgeo0bd9nc73a0vh1g`; root-caused same day to the 06:54 UTC roll collision — audit log + k8s events above) + inbox note `036` (post-roll SSR error page, observed twice on 2026-07-18, filed at the m51 closeout).
- **Goal linkage:** w1's charter is "de-risk the live system" — production reliability of the Render-alternative core (docs/ADR008-vision.md). Render's own dashboard never surfaces their deploys to users; bex currently does, several times a day. Completes the arc m30 started (SIGTERM in-process graceful shutdown drains in-flight requests) — what remains is the window where Traefik still routes *new* requests to a terminating pod, and the dashboard's blind readiness/dehydrated-error behavior around it.
- **Expected outcome:** `deploy.yml`'s many daily rolls stop being user-visible error windows; mid-roll user actions and page loads succeed or transparently recover.
- **Why now:** deploy cadence is the risk multiplier — five prod rolls in 40 minutes the same morning; the incident reproduced twice in one day (m50, m51 rolls) and will keep reproducing on every deploy until fixed.
- **Render parity included** (t005): the fix touches the dashboard UI's user-visible error/recovery behavior; REST/GraphQL/MCP contracts are expected to be untouched, and t005 verifies exactly that plus compares the roll-window UX against Render's dashboard.
