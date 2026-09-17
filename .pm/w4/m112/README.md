# w4 · m112 — Health-gated rollouts stall silently: no UI signal names the failing probe

**Worker:** worker4 **Goal:** when a rollout is gated on pods that never become ready because their health-check probe fails, the deploy detail page and events feed say so — naming the probe path and the observed failure — instead of a bare "In Progress" for up to 15 minutes. **Status:** todo

## Tasks (in order)

| id   | title                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Operator diagnoses probe-failing (Running-but-unready) pods in the stall message       | 1h  | —          |
| t002 | Project the stall reason onto the in-progress deploy/events for UI consumption         | 1h  | t001       |
| t003 | Dashboard renders the stall reason on the deploy detail page while In Progress         | 45m | t002       |
| t004 | Render parity                                                                          | 20m | t003       |
| t005 | Simplify                                                                               | 15m | t004       |
| t006 | Test coverage                                                                          | 45m | t004       |
| t007 | Closeout                                                                               | 10m | t006       |

## Definition of done

Each bullet is a click the next person can repeat on production and watch succeed.

- **The stall names the probe.** On a throwaway web service: set Health Check Path to a path that 404s. Within ~2 minutes of the new pod logging `listening` while the deploy sits `update_in_progress`, the deploy detail page states the rollout is waiting on the health check, names the path, and gives the observed probe result — instead of a bare "In Progress" with a log stream that just stops.
- **The events feed carries the same reason.** The same window shows a reason on the in-progress deploy's event rows (a `reasonCode`-family value rendered through the existing `services.eventsReason.*` locale keys), so the feed and the detail page agree.
- **Existing stall signals unchanged.** CrashLoopBackOff / ImagePullBackOff stalls keep their current messages (the `stuckPodMessage` Waiting-container cases), and the `RolloutSettling` exclusion still suppresses phantom outage pages (w3/m78 control).
- **The settle itself is untouched.** At `progressDeadlineSeconds` the rollout still settles exactly as today (Running over the prior release when one served, Failed with reason otherwise) — this milestone adds diagnosis, not new settle behavior.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass on `https://dashboard.bex.co`, 2026-09-17 (w4-targeted run, `muse.env` credentials). Fixture: `qa-20260917-p7-web` (`srv-dam2aih2dbts73fi25tg`); health path `/qa-bogus-health` (404s); `config_change` deploy `dep-dam2bvjs0ils73bgove0` sat `update_in_progress` from 10:40 with its new pod listening-but-unready while every surface showed only "In Progress"; canceled at 10:43; phase sat `Deploying` with zero in-flight work until flipping to `Running` at 10:58:40 — exactly the 900s `progressDeadlineSeconds` (`deployment_projection.go:131`) via `settleFailedRollout` → `settlePriorRelease` (`app_controller.go:2849-2852,4541-4546`). Public URL served 200 throughout (prior pod kept routing).
- **Goal linkage:** product truthfulness on the deploy journey ([docs/ADR004-app-deployment.md](../../../docs/ADR004-app-deployment.md)) and the parity ledger ([docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md)). A user watching a health-gated stall — including the post-cancel window this hunt's m152 extension covers — currently cannot distinguish "still pulling the image" from "your probe path 404s" without waiting out the full 15-minute budget.
- **Expected outcome:** health-gated stalls become self-diagnosing on the deploy page within ~2 minutes, in both locales, without changing any settle behavior.
- **Why now:** the fix composes entirely from existing machinery — `stuckPodMessage` already diagnoses Waiting-container stalls (`app_controller.go:4176`), the `readiness_failed` reason vocabulary and its dashboard renderer already exist (`event_facts.go:105`, `services.$serviceId.events.tsx:472-474`) — the probe-failing case is the one stall class that falls through every layer. Render parity is included because the fix touches the shared deploy/events surface (operator condition → backend projection → dashboard).
- **Explicitly out:** the cancel half of this hunt's observation (cancel re-dispatches the old image against current spec, shipping the canceled probe) is w1/m152's filed mechanism, extended with live evidence as w1/m152/t010 — not re-filed here. The `m110/t001` config-rebuild taxonomy question (this health-path save rebuilt the image while env-only saves reuse) stays with m110.
