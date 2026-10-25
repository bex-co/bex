# w1 · m185 — A serving App with zero running instances records server_failed

**Worker:** worker1 **Goal:** when the active revision has no ready pod — even while a new deploy builds or rolls out — bex-api records `server_failed`, so the Events feed, webhooks and Critical push say the service is down. **Status:** done (2026-10-25)

## Tasks (in order)

| id   | title                                                                  | est | depends_on |
| ---- | ---------------------------------------------------------------------- | --- | ---------- |
| t001 | Operator keeps the Serving condition current through the build phase — **DONE**   | 40m | —          |
| t002 | bex-api concludes unhealthy from Serving=False on the active revision — **DONE**  | 40m | t001       |
| t003 | Simplify — **DONE**                                                               | 15m | t002       |
| t004 | Test coverage — **DONE**                                                          | 30m | t002       |
| t005 | Closeout — **DONE**                                                               | 10m | t004       |

## Definition of done

- While an App builds (or waits for build capacity) on top of a live revision, the operator re-observes `Serving` every build poll and persists a change even when phase and Ready are unchanged.
- `observedServiceStateFor` concludes `unhealthy` (readiness_failed) from a current-generation `Serving=False` / `ServingRevisionUnavailable` on an App with an active revision, open deploy or not; disk-backed (Recreate) services keep today's behavior.
- Regression tests reproduce the 2026-10-24 shape: live pod gone, replacement never Ready, a deploy building throughout.

## Source + Goal linkage

- **Source:** [`.pm/w1/done/140.md`](../done/140.md) (promoted 2026-10-25 by `/loopx w1`; split from `w1/138`).
- **Goal linkage:** honest status on every surface (Render's `server_failed` event and notifications).
- **Expected outcome:** an outage like beancount.io's (5h, zero events) emits `server_failed` within the debounce window and `server_available` on recovery.
- **Why now:** the outage just showed the gap; a builder retrying for hours hides any down service.
- **Render parity task omitted:** no API/UI shape change — existing `server_failed`/`server_available` events simply fire when they should.

## Closeout (2026-10-25)

- t001: `setBuildingPhase` (build waiting/building polls) re-runs `observeServingRevision` via `refreshServingWhileBuilding` and persists a Serving change through `setPhaseWith` even when phase and Ready are unchanged.
- t002: `servingRevisionDown` in `lego/backend/internal/store/reconciler.go`: Building + active revision + no disk + current-generation `Serving=False`/`ServingRevisionUnavailable` concludes unhealthy/readiness_failed before the open-deploy exemption; the existing two-pass debounce still applies.
- t003: reviewed; reuses `observeServingRevision` and `setPhase`.
- t004: incident shape → unhealthy with Serving's transition time; no outage for a serving live revision, an unobserved or stale verdict, a disk-backed service, or mid-rollout; operator refresh reports a change once and leaves first deploys / missing Deployments alone. `make test`, `make lint` green.
- Scope note: the rule is limited to the build phase, where the Deployment still runs only the active revision. During a rollout the new revision's pods replace it before `ActiveRevision` moves, so Serving=False there is not an outage.
