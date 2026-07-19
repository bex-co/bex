# w3 · m19 — Service activity timeline: durable history + observed event fidelity

**Worker:** worker3 **Goal:** Make a service's Events tab a durable, paged account of deploy, availability, scaling, and source-control transitions instead of a one-hour empty view, while preserving the operator's DB-free boundary and the feed's structural redaction guarantee. **Status:** done — **DONE** 2026-07-18

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Fix dashboard event-window semantics and cursor accumulation — **DONE** | 35m | — |
| t002 | Add a typed, idempotent durable source for observed service-event facts — **DONE** | 45m | — |
| t003 | Project deploy and image-pull lifecycle facts — **DONE** | 40m | t002 |
| t004 | Project suspension, failure, recovery, and availability edges — **DONE** | 45m | t002 |
| t005 | Record branch changes and ignored commits — **DONE** | 40m | t002 |
| t006 | Expose autoscaling activity in App status and persist start/end edges — **DONE** | 45m | t002 |
| t007 | Rebuild the Events timeline with Render-style filters and older-history UI — **DONE** | 45m | t001, t003, t004, t005, t006 |
| t008 | Live acceptance: exercise and cross-check every sourceable event family — **DONE** | 40m | t007 |
| t009 | Render parity — surfaces, vocabulary, evidence, and deliberate omissions — **DONE** | 30m | t008 |
| t010 | Simplify the service-event ingestion, projection, and dashboard diff — **DONE** | 25m | t009 |
| t011 | Add meaningful transition, paging, redaction, and UI test coverage — **DONE** | 35m | t009 |
| t012 | Close out the milestone — **DONE** | 15m | t010, t011 |

## Definition of done

Opening `dashboard.bex.co/services/<srv-id>/events` for a quiet but previously deployed store-managed service shows its retained history rather than an empty one-hour slice; the page cursor-loads older results without duplicates or gaps, offers Render-shaped event filtering, and the Metrics timeline asks for its selected range rather than inheriting a hidden one-hour default. REST, GraphQL, MCP, the dashboard feed, metrics markers, and outbound webhooks agree on every newly sourceable transition. A live sequence proves the 19 sourceable labels below at most once per real edge, across API intent and operator-observed state, while retries/resyncs create no duplicates and secret/env values cannot enter event facts. Unsupported labels stay explicitly absent with the reasons below—no synthetic billing, workflow, preview-environment, platform-maintenance, or edge-cache events.

## Investigation and support boundary

### Why the page is empty

- `dashboard/src/features/events/hooks/use-service-events.ts` sends only `{serviceId, limit}`. Its GraphQL document does not declare `startTime` or `endTime` variables.
- `events.Service.List` therefore applies `DefaultWindow = time.Hour` whenever `startTime` is absent. The dashboard labels the result as the service's activity rather than “last hour,” so a successful empty response is indistinguishable from “no history.” Store-off is not this symptom: it returns 503 and the route renders its error state.
- The same shared hook is used by the Metrics event timeline, so a 12-hour/24-hour/custom chart range is also backed by at most one hour of event data.
- The route requests only 20 rows and discards every returned cursor. Render's captured page continues through weeks of events and keeps loading older entries.

### Render labels supplied in the report

| Render UI group | Label | bex disposition after m19 |
| --- | --- | --- |
| Deploy | Deploy Started | **Existing** — `deploy_started` from `deploys.created_at` |
| Deploy | Deploy Ended | **Existing** — `deploy_ended` from the terminal deploy transition |
| Deploy | Image Pull Failed | **Add** — typed failure reason + image from the observed deploy |
| Deploy | Initial Deploy Hook Started/Ended | **Omit** — Render preview-environment initialization; previews are a rejected non-goal |
| Deploy | Pipeline Minutes Exhausted | **Omit** — bex has no pipeline-minute billing/enforcement source |
| Service Status | Resumed / Suspended | **Existing** — user intent as `suspender_removed` / `suspender_added` |
| Service Status | Instance Failed | **Add** — observed Ready/pod failure edge with a bounded reason code |
| Service Status | Server Restarted | **Existing** — `server_restarted` from `apps.Restart` |
| Service Status | Service Resumed / Suspended | **Add** — operator-observed Running/Hibernated convergence, distinct from intent |
| Service Status | Service Recovered | **Add** — observed unhealthy→healthy edge (`server_available`) |
| Scaling | Autoscaling Started / Ended | **Add** — typed autoscaling status edge projected through the App CR |
| Scaling | Autoscaling Config Changed | **Existing** — typed previous/current min/max |
| Scaling | Instance Count Changed | **Existing** — manual replica change with typed previous/current counts |
| Scaling | Branch Changed | **Add** — typed previous/current branch on the source-change write |
| Scaling | Commit Ignored | **Add** — signed push matched the service but skip/build filters rejected the commit |
| Scaling | Instance Type Changed | **Existing** — Render's UI label for `plan_changed` |
| Scaling | Workflow Deploy Started/Ended | **Omit** — Workflows/tasks are an explicit roadmap anti-goal |
| Maintenance | Maintenance Started/Ended | **Omit** — provider platform-maintenance runs are an explicit roadmap anti-goal |
| Maintenance | Maintenance Deploy Started/Finished | **Omit** — same provider-maintenance mechanism, absent from bex |
| Maintenance Mode | Config Changed / URI Updated | **Existing** — `maintenance_mode_enabled` / `maintenance_mode_uri_updated` |
| Edge Cache | Enabled / Disabled / Purged | **Omit** — bex has no mutable edge-cache mode; purge is explicitly a non-goal because static revisions are immutable |

This reaches **19 of the 31 supplied labels** truthfully (10 existing + 9 added). Render's current public `list-events` enum also includes build/pre-deploy, disk, hardware-failure, and auto-deploy events not all visible in this particular dashboard filter; t009 must compare against the current OpenAPI again and document any additional event that becomes free from the same typed sources. It must not expand into persistent disks, preview environments, workflows, maintenance runs, billing, or mutable edge-cache controls forbidden by `.pm/DO_NOT_DO.md`.

## Source + Goal linkage

- **Source:** User report 2026-07-18: `https://dashboard.bex.co/services/srv-d9bj8s3eg85c7390eb9g/events` is always empty; compare with authenticated Render `https://dashboard.render.com/web/srv-caijkqarrk01jalurn90/events` and support the supplied 31-label filter inventory. Evidence also includes `.playwright-mcp/render-events-tab.png`, the saved accessibility snapshot showing weeks of paired intent/observed events, Render's current `GET /v1/services/{serviceId}/events` OpenAPI, and `render.com/docs/webhooks`.
- **Goal linkage:** ADR008 pillars 1 and 4 plus `GOAL.md` basic observability: humans and agents need an honest activity history to explain what the platform did, why a deploy or instance failed, and which state transition followed an accepted action.
- **Expected outcome:** Quiet services retain a useful Events page; all clients see one typed, redacted, cursor-stable feed; the sourceable supplied vocabulary rises from 10/31 to 19/31; outbound notifications inherit the same facts without a parallel emission path.
- **Why now:** The existing UI reports a successful but misleading empty state on the named production-shaped service, and its shared one-hour query also makes newly shipped Metrics markers incomplete. The deploy lifecycle, pre-deploy timestamps, actionable failure reasons, custom autoscaler, signed Git webhook, and control-plane status projector now provide the raw facts that did not exist when w3/m7 deliberately omitted these events.
- **Render parity closing task:** included because the milestone changes REST/GraphQL/MCP semantics, the dashboard feed and metrics markers, and the event vocabulary consumed by outbound webhooks.
