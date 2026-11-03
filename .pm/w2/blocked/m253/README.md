# w2 · m253 — A service that crash-loops for 24 hours is suspended with `stuck_crashlooping`, as on Render

**Worker:** worker2 **Goal:** a service that repeatedly fails to start and become healthy over 24 hours is suspended with Render's `stuck_crashlooping` suspender, its owners are told, and a user resume brings it back. **Status:** blocked — t001–t004, t006–t008 done; t005 (dev-2 walk with a short window) next, then t009

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Decide and record the detection rule — **DONE** | 45m | — |
| t002 | Operator: detect and suspend with a recorded reason — **DONE** | 1.5h | t001 |
| t003 | Surface stuck_crashlooping on REST, GraphQL, MCP, events, webhooks and the dashboard — **DONE** | 1h | t002 |
| t004 | Suspension notice by email and push — **DONE** | 30m | t003, w2/m252/t005 |
| t005 | dev-2 walk with a shortened window | 45m | t002 |
| t006 | Render parity — **DONE** | 15m | t004, t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- A service with no Ready instance for 24h while restarts continue is suspended; REST/GraphQL/MCP report `suspended: true`, `suspenders: ["stuck_crashlooping"]`, and the dashboard explains why.
- A service that becomes Ready at any point in the window is not suspended; services that are building, deploying, hibernated or already suspended are excluded.
- A user resume clears the suspender and the service starts again.
- Owners receive the suspension notice by email (w2/m252 path) and push.
- An envtest proves the 24h boundary; dev-2 proves the flow with a shortened window override.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-11-02 #3. render.com/docs/notifications: "Render suspends a service that repeatedly fails to start and become healthy over a 24-hour period … often caused by a service configuration error, a bug in application code, or an undersized compute plan." Render's pinned `suspenderType` enum (`render-public-api-1.json`) includes `stuck_crashlooping`; bex has zero references to it. eden-cms-v2 (2026-08-20) was liveness-killed 33×/16h.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); dense bin-pack economics (crash loops waste node capacity).
- **Expected outcome:** forgotten crash loops stop consuming capacity, and owners learn why their service stopped.
- **Why now:** it is an unimplemented value of a pinned Render enum; w2/m252 provides the email path for the notice.
- **Render parity included:** `suspenders`, events, webhooks and the dashboard banner are tenant-facing.
- **Open design point:** Render does not publish the exact detection rule; t001 records bex's rule in an ADR.

## Work log — 2026-11-03

- **t001:** [ADR096](../../../docs/ADR096-crashloop-suspension.md) (ADR094/095 were taken by concurrent sessions). Rule: Ready `False` on `CrashLoopBackOff`/`HealthCheckFailing` for 24h, measured from the later of Ready's transition and the sweeper's first sighting at the current spec generation (`app.bex.co/crashloop-observed`); excluded: suspended (owner/Free hours/crash loop), Building/Deploying, cron jobs, static sites.
- **t002 (placement changed):** detection runs in bex-api (`internal/crashloop.Sweeper`, every 5 min), not the operator — the same shape as the Free-hours suspension, because suspension is a `spec.suspended` write plus an annotation that bex-api owns and the operator never writes spec. `BEX_CRASHLOOP_SUSPEND_AFTER` overrides the window (env inventory updated).
- **Review fixes:** the first draft measured from Ready's transition alone, so a resumed (still broken) service would have been re-suspended within minutes with no new email — the sighting-per-generation rule fixes it (`TestNewGenerationRestartsTheWindow`); the owners are now notified before the suspension is written, so a failed notification suspends nothing (`TestUnnotifiedServiceIsNotSuspended`).
- **t003/t006:** `suspenders: ["stuck_crashlooping"]` on REST, GraphQL and MCP via `core.SuspendersFor`; a manual resume clears it; the dashboard Resume card explains the cause (en/zh); `CrashLoopBackOff`/`HealthCheckFailing` promoted to `lego/types` reason constants. ADR018 Suspend/Resume and Notifications rows updated.
- **t004:** `CrashLoopNotifier` enqueues a `service_suspended` failure alert (migration 0178) through w2/m252's queue; push and webhooks get the observed `service_suspended` event as before.
- **t008:** `internal/crashloop` (window boundary, exclusions, sighting, generation reset, recovery, notify-first), `apps/crash_loop_test.go` (suspenders + resume), notifier unit test, PG test for the cursor-less enqueue and the new event value, dashboard mapping test; full dashboard suite (5749) green.

## Blocked (2026-11-03)

All implementation is done (t001–t004 and t006–t008 done; t005 (dev-2 walk with a short crash-loop window) blocked). The remaining walk needs a gate this session cannot clear: user: choose how the local CAPD cluster runs the egress nft rule (drop it locally, or rework it to bind in nested kind nodes) so dev-2 comes up for the short-window crash-loop walk. Run `/unblock w2` to decide. The milestone returns to the queue once the gate clears.
