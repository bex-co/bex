# w6 · m120 — A hibernated free service wakes itself repeatedly with no traffic, cycling every 60–90 seconds

**Worker:** worker6 **Goal:** a service that auto-hibernates stays hibernated until something actually asks for it, so the free tier costs what ADR003 says it costs and the events feed reports wakes that happened **Status:** todo

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | -------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Isolate what re-stamps `last-active` (or otherwise un-parks) with no request       | 45m | —          |
| t002 | Fix the cycle so a hibernated service stays hibernated until a real wake           | 45m | t001       |
| t003 | Stop emitting `service_woken` for a wake nothing requested                         | 30m | t001       |
| t004 | Render parity                                                                      | 20m | t002, t003 |
| t005 | Simplify                                                                           | 20m | t004       |
| t006 | Test coverage                                                                      | 30m | t004       |
| t007 | Closeout                                                                           | 10m | t005, t006 |

## Definition of done

- **A hibernated free service that receives no request stays hibernated.** Repeat this milestone's probe: create a free web service, set `idleTTLSeconds: 60`, never touch its URL, and read `GET /v1/services/{id}/events` after 10 minutes. Today that window contains repeated `service_hibernated`/`service_woken` pairs (capture below); after the fix it contains **one** `service_hibernated` and **no** `service_woken`.
- **A real wake still works, verified in the same run**, so the fix is not "stop waking": from that hibernated state, one `curl` against the `.onbex.co` URL returns `200` within the documented window and produces exactly one `service_woken`. `w6/m116`'s run measured that path at 15 seconds with the honest `503`/`retry-after: 5` interstitial; that must not regress.
- **`service_woken` means a service woke because something asked for it.** It is a push-notification event type (the Notifications page's "Service recovered"/event filters) and a webhook event, so a false one reaches subscribers, not just the timeline.
- `make test` from `lego/operator/` has a test that fails if a parked App un-parks without its `last-active` advancing.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 50th run, 2026-08-27. Fixture `qa-20260827-resume` (`srv-da868cfm2e9c73ft5prg`, free `web_service`, Go, deleted at end of run). Reached `Running`, then `setIdleTimeout(idleTTLSeconds: 60)` at `16:24:53Z`. **Its public URL was never requested at any point in the run** — every probe went to `api.bex.co`.

  `GET /v1/services/srv-da868cfm2e9c73ft5prg/events?startTime=2026-08-27T16:24:00Z`:

  ```
  service_hibernated@16:32:15Z
  service_woken@16:31:32Z
  service_hibernated@16:30:02Z
  service_woken@16:29:15Z
  service_hibernated@16:28:45Z
  service_woken@16:27:45Z
  service_hibernated@16:27:32Z
  service_woken@16:26:32Z
  suspender_removed@16:26:19Z      <- a resumeService call (see below)
  service_hibernated@16:26:02Z
  deploy_ended@16:25:02Z
  idle_timeout_changed@16:24:53Z
  ```

  Counted: **5 `service_hibernated`, 4 `service_woken` in six minutes, with no traffic.** Polling agreed — `phase` alternated `Running`/1 instance ↔ `Hibernated`/0 instances across the same window.

- **What this run got wrong first, and how the artifact corrected it.** The `resumeService` at `16:26:19Z` was followed by `service_woken` 13 seconds later, and a poll 30 seconds after that showed `Running` — which read as "Resume recovers an auto-hibernated service". The cycle then continued with three more wakes and no further calls, so that first wake was one beat of an ambient cycle, not an effect of the mutation. `Resume` is `setSuspended(ctx, name, false)` (`apps/service.go:2933-2935`) and is type-agnostic, so it could not have behaved one way here and another way on the private service of `w6/m119` — the divergence was the observation, not the code. The event feed is the instrument that settles it; polling is not.
- **Root cause: not isolated.** `t001` owns it. The mechanism must explain an un-park with no request, and the candidates worth opening first are:
  - `desiredReplicas` returns 0 only while `shouldAutoHibernate` is true, which needs `time.Since(lastActiveTime(app)) >= TTL` (`app_controller.go:1641-1650`). For the App to come back, `app.bex.co/last-active` must be advancing — but `:2096-2102` re-stamps it **only when the annotation is zero** (`if lastActiveTime(app).IsZero()`). So something is clearing or losing the annotation, and the re-stamp then restarts the whole TTL.
  - `holdHibernateForRouting` (`:1729`) plus `parkKubernetes`'s `routingHold` branch (`:1859-1861`) return the pre-hibernate replica count and requeue after `hibernateRoutingGrace` (10s, `:1942`) — `w6/m94`'s deliberate hold so a live endpoint never disappears before Traefik observes the activator route. Confirm this is a one-shot grace and not re-entrant.
  - The control-plane projector: `w6/m46`'s post-mortem found `applyOwnedSpec` re-stamping a field on every resync and bumping `metadata.generation`. If a resync touches the App's metadata, an annotation-based timer is exactly the kind of state that would be disturbed.
- **Goal linkage:** [ADR003](../../docs/ADR003-control-plane.md):80 — "a sleeping pod occupies nothing, so the cluster **overcommits well beyond Σ** and Free approaches \$0". A free service that restarts a pod every 60–90 seconds occupies a great deal, and does so precisely while advertising that it is asleep. Also [ADR007](../../docs/ADR007-restart-suspend-and-resume.md) (the auto-hibernate/wake design) and [ADR006](../../docs/ADR006-bex-api.md) for the event vocabulary.
- **Expected outcome:** a hibernated free service consumes nothing until a request arrives, and every `service_woken` on the timeline corresponds to a real one.
- **Why now:** it is the load-bearing assumption of the free tier, and it compounds two open items — `w6/m110` (App compute is never metered, so this churn is invisible in usage) and `w6/m118` (nothing caps instance count by plan). It also actively degrades `w6/m119`'s evidence: that milestone's private-service claims were established by polling at 90-second intervals, which this data proves cannot distinguish "never wakes" from "wakes every 60–90 seconds". `w6/m119/t009` was added this run to re-verify those claims against the event feed.
- **Render parity:** included (t004). Render's free tier spins a service down and wakes it on request; a spontaneous wake has no Render counterpart, so this is a bex defect rather than a divergence to record — but the `service_woken` event's semantics are a bex extension worth stating in the ledger once `t003` settles them.
- **Blast radius:** whatever `t001` finds sits in the operator's hibernate path, which every auto-sleep-eligible type shares (`web_service`, and per `w6/m119` `private_service`). `service_woken`/`service_hibernated` are consumed by the Events tab, outbound **webhooks**, and **push notifications** — the Notifications page exposes "Service crashed"/"Service recovered" event filters — so a false event reaches subscribers, and `t003` must check all three consumers rather than the timeline alone.
- **Adjacent classes:** the fix must not make a **genuine** wake slower or less reliable (`w6/m94` fixed two races on that path and its guarantees must hold), and must leave **explicit suspend** untouched — a suspended App is parked for a different reason and must stay parked.
- **Unverified this run:** (1) the mechanism, entirely — `t001` exists because this run observed the cycle without isolating its cause. (2) Whether a `private_service` flaps the same way; `w6/m119/t009` will answer it. (3) Whether the cycle depends on the short 60s TTL used here or also occurs at the dashboard's shortest preset (300s) — the fixture used an API-only value, and a longer TTL may simply lengthen the period or may not reproduce at all. (4) Whether the false `service_woken` events actually reach webhook subscribers — no webhook was subscribed during this run.
