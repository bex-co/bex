# w4 · m364 — Autoscaling never changes a service's instance count on production: the operator cannot read pod metrics, so an idle service with autoscaling on stays at 3 instances for 10+ minutes and a manual scale while autoscaling is on takes effect

**Worker:** worker4 **Goal:** a service with autoscaling on runs the instance count its CPU and memory targets call for. It scales down to `min` once the 5-minute stabilization window passes on an idle service, scales up under load, and records `autoscaling_started`/`autoscaling_ended`. A manual scale while autoscaling is on is stored, as Render does, but does not change the running count. **Status:** todo (t001–t006 done; t007, the live production replay, pending)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Confirm on production that the operator's metrics read is refused, and grant the controller `get`/`list` on `pods.metrics.k8s.io` — **DONE** | 40m | — |
| [t002](done/t002.md) | Make a failed metrics read visible: log it, and surface a condition instead of holding silently forever — **DONE** | 40m | t001 |
| [t003](done/t003.md) | A manual scale while autoscaling is on stores `numInstances` and leaves the running count to the autoscaler — **DONE** | 45m | t001 |
| [t004](done/t004.md) | Render parity — **DONE** | 20m | t002, t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 40m | t004 |
| [t007](t007.md) | Closeout (live replay on production) | 30m | t005, t006 |

## Definition of done

All on production, workspace `bex-canary`, on a Starter web service from `POST /v1/services` (`docker.io/traefik/whoami:v1.10`, `dockerCommand: /whoami -port 10000`):

1. **Scale-down works.** `POST …/scale {"numInstances":3}`, wait for 3 instances, then `PUT …/autoscaling {"enabled":true,"min":1,"max":3,"criteria":{"cpu":{"enabled":true,"percentage":70},"memory":{"enabled":false,"percentage":70}}}`.
   - Within 7 minutes `GET …/instances` lists 1 instance.
   - `GET …/events` shows `autoscaling_started` then `autoscaling_ended` (from 3 to 1).
   - Round 265 stayed at 3 instances for 10.5 minutes with no autoscaling event.
2. **Scale-up works.** With autoscaling on (min 1, max 2, CPU 50%), load the service: a busy loop in a second command, or sustained requests. Within a few minutes `…/instances` lists 2, and `autoscaling_started` records 1 → 2.
3. **A manual scale while autoscaling is on is ignored at runtime.** With autoscaling on and the service at its autoscaled count, `POST …/scale {"numInstances":3}` answers 202 and stores `serviceDetails.numInstances: 3`. `…/instances` keeps the autoscaled count. After `DELETE …/autoscaling` the service runs 3, its own count.
4. **A refused metrics read is visible.** The operator logs the failure with the App name, and the App carries a condition naming it. Exercise this in a test cluster with the new RBAC rule removed.
5. **Fixtures.** Every fixture is deleted, and each GET answers 404.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 265, 2026-11-13 06:02–06:20 UTC (`muse.env`, workspace `bex-canary`; pin `4c3c68e6b05b`). The probes are under § Finding.
- **Governing docs:**
  - [ADR018](../../../docs/ADR018-render-parity.md) rows "Manual scale (instance count)" (line 99) and "Autoscaling config" (line 100), both ✅ on every surface. The autoscaling row describes the stabilization window and the scale-down rules as shipped behavior.
  - w1/008 (done) built the autoscaler: "a reconciler that adjusts `spec.replicas` from live metrics".
- **Goal linkage:** Render parity on a paid feature. [Render: scaling](https://render.com/docs/scaling) — autoscaling adjusts instance count to the CPU and memory targets. [Render: Scale a service](https://api-docs.render.com/reference/scale-service), read 2026-11-13: "Render ignores this value as long as autoscaling is enabled for the service."
- **Expected outcome:**
  - Autoscaling saves money on idle services and keeps busy services up, as its settings page promises.
  - A Render client's manual scale cannot pin extra billed instances under an autoscaling policy.
- **Why now:**
  - Major. Every autoscaling service on production runs at whatever count it had when autoscaling was turned on: no scale-up under load (availability) and no scale-down (cost).
  - The dashboard and API report the policy as active, and nothing tells the user it does nothing.
  - No `.pm` item records the autoscaler ever acting on production (`grep -rln "autoscaling_started\|autoscaling_ended" .pm` finds only design and event-shape items).
- **Render parity task included (t004):** the REST `scale` and `autoscaling` verbs, GraphQL, MCP and the Scaling page are touched.

## Finding (round 265)

**Fixture:** Starter web service `qa-20261113-r265-scale` (`srv-dbrag9gv8soc73b22la0`), created 06:03:18 by `POST /v1/services`, live 06:05:13. Deleted at 06:19:55 (`DELETE` 204, then `GET` 404). Requests went through `curl` with this run's own session jar; the Playwright browser was held by another session.

1. **Manual scale works (control).**
   - `POST …/scale {"numInstances":2}` at 06:05:28 → `202` (with the service body; Render documents no body).
   - `…/instances` listed 2 at 06:05:35. A `curl` every ~0.5 s answered `200` all 38 times, spread over both pods (12 and 26).
   - Events: `instance_count_changed {"fromInstances":1,"toInstances":2}`.
2. **Autoscaling turned on.** `PUT …/autoscaling` with min 1, max 3, CPU 70% at 06:06:25 → `200`. Events: `autoscaling_config_changed`.
3. **Manual scale while autoscaling is on takes effect.**
   - `POST …/scale {"numInstances":3}` at 06:06:28 → `202`, `numInstances: 3`.
   - `…/instances` listed 3 at 06:06:40. Events: `instance_count_changed {2→3}`.
   - Render ignores that value while autoscaling is on.
4. **The autoscaler never acts.**
   - Polled every ~30 s from 06:09:15 to 06:17:03: `instances=3`, `numInstances=3`, `replicas=3`, throughout.
   - That is 10.5 minutes after autoscaling was turned on, against a 5-minute scale-down window (`scaleDownStabilizationWindow`, `autoscale.go:47`) on an idle `whoami` (CPU near 0, target 70%, min 1).
   - No `autoscaling_started` or `autoscaling_ended` event was recorded.

### Mechanism (from code; cluster state not read)

- `applyAutoscaling` (`lego/operator/internal/controller/autoscale.go:284`) reads pod usage through `r.MetricsReader` (`:306`). On any error it returns `current` with a requeue — "Metrics unavailable — hold steady" (`:308`) — and logs nothing.
- The reader is `NewMetricsServerReader` (`:83`, wired at `lego/operator/cmd/manager/main.go:503`). It `GET`s `/apis/metrics.k8s.io/v1beta1/namespaces/<ns>/pods` (`autoscale.go:86`) with the controller-manager's ServiceAccount.
- **No rule grants that ServiceAccount `metrics.k8s.io`.**
  - The controller's generated ClusterRole `lego/operator/config/rbac/role.yaml` has no `metrics.k8s.io` rule, and no `+kubebuilder:rbac:groups=metrics.k8s.io` marker exists in `lego/operator`. `app_controller.go:491` grants core `pods` only.
  - `manager-rolebinding` binds `controller-manager` to `manager-role` alone.
  - The only grants are to bex-api (`deploy/gitops/base/bex-api-apps-rbac.yaml:42`) and the human day-to-day operator role (`operator-daytoday-rbac.yaml:88`).
  - metrics-server's chart aggregates its reader role into view/edit/admin, none of which the controller holds.
- So every read is refused, `applyAutoscaling` never reaches `settleAutoscaler` (`:359`), and `annotAutoscaleReplicas` is never written.
  - The replica seed therefore stays `configured`, i.e. `spec.replicas` (`app_controller.go:3158-3172`).
  - That is also why the manual scale in step 3 changed the running count: `Scale` writes `spec.replicas` (`lego/backend/internal/apps/service.go:4569`) with no regard for autoscaling.
- Unit tests drive `autoscaleDesired` and the reconciler with a fake `MetricsReader` (`autoscale_test.go`), so the real reader's permission was never exercised.

## Unverified

- **The 403 itself.** No production cluster access this round. t001 reads the operator log or runs `kubectl auth can-i list pods.metrics.k8s.io --as=system:serviceaccount:<ns>:controller-manager`. If the read is permitted, the cause is elsewhere in `applyAutoscaling` (metrics-server returning no items for the selector, or `tierLimits("starter")` evaluating nothing), and t001 must name it before t002.
- **Scale-up under load** was not probed. The metrics read precedes every decision, so it is expected to fail the same way.
- **The memory criterion's percentage.** The PUT sent `memory {enabled:false, percentage:70}` and read back `percentage: 0`. Render's schema requires `percentage` without saying whether a disabled criterion keeps it; that is the same open question as [w4/598](../blocked/598.md). Not part of this milestone.

## Dedupe

- w1/008 (done) built the autoscaler. w5/109, w5/120 and w5/130 (done) refined its window and re-enable rules. w4/m335 (blocked) covers the `autoscaling_config_changed` event shape. w4/596 and w4/597 (done) cover the autoscaling read and its off/on memory. None verifies that the autoscaler acts on production, or covers a manual scale under autoscaling.
- Not deploy lag: `git log 4c3c68e6b05b..origin/main -- lego/operator/internal/controller/autoscale.go lego/operator/config/rbac lego/operator/cmd/manager/main.go` is empty.

## Decisions and follow-ups (2026-11-14, t001–t006)

- **Cause confirmed (t001):** the operator ServiceAccount could not list `pods.metrics.k8s.io` (`can-i` = `no`, metrics-server Available). Fixed by the generated `manager-role` rule.
- **Event decision (t003):** with autoscaling on, a manual scale stores `numInstances` and records no `instance_count_changed`, because no instance count changed and Render documents no event for a scale it ignores. Trade-off: that stored-count change leaves no audit row.
- **Rollout order:** the ClusterRole and the operator Deployment both render from `lego/operator/config/prod` in the `bex-operator` Argo Application. The new operator works before the grant syncs (it shows the paused condition until then), and the grant is harmless to the old operator, so their order does not matter.
- **Follow-ups to file:**
  - Render's `POST …/scale` 202 has no body; bex returns the service (pre-existing drift, recorded in ADR018's Manual scale row).
  - An unwired `MetricsReader` (the manager could not build the metrics client) still skips autoscaling silently; production always wires it.
  - Blueprint applies and deploy-time scale paths (`deploy.go`, `blueprint_setting_events.go`) still record `instance_count_changed` for a count change under autoscaling, which runs nothing either.
