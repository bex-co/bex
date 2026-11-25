# w6 · m634 — A failed health check says what the check got back, in Render's words, on every surface

**Worker:** worker6 **Goal:** when an instance fails its health check, at deploy or while live, every surface names what the check got back in Render's words: the status code, the 5-second timeout, `connection refused` or `i/o timeout`. The surfaces are the `server_failed` event's `failureReason.unhealthy` on REST, GraphQL, MCP and webhooks, the deploy's failure reason, and the dashboard. **Status:** blocked (t001–t006, t009 and t010 done; t007 waits for a production pin carrying `e3a1842c94`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [ADR004 design: how the probe's last result reaches the App's status](done/t001.md) — **DONE** | 30m | — |
| t002 | [The health-probe helper reports a failure in Render's words](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [The operator records an instance's last health-check result when it fails readiness](done/t003.md) — **DONE** | 1h30m | t002 |
| t004 | [`server_failed` carries the result as `failureReason.unhealthy` on REST, GraphQL, MCP, webhooks and the alert email](done/t004.md) — **DONE** | 1h | t003 |
| t005 | [A deploy that fails its health check ends its failure reason with the last result](done/t005.md) — **DONE** | 45m | t003 |
| t006 | [The dashboard's Events and deploy pages show the result](done/t006.md) — **DONE** | 45m | t004, t005 |
| t007 | [Production check in `bex-canary`: a 503, a timeout and no listener](t007.md) | 45m | t006 |
| t008 | [Render parity](t008.md) | 30m | t007 |
| t009 | [Simplify](done/t009.md) — **DONE** | 20m | t008 |
| t010 | [Test coverage](done/t010.md) — **DONE** | 45m | t008 |
| t011 | [Closeout](t011.md) | 10m | t009, t010 |

## Definition of done

- On production, a disposable web fixture in `bex-canary` that answers its Health Check Path with 503 reads `HTTP health check failed with status code 503` in `failureReason.unhealthy` on REST `GET /v1/events`, GraphQL and MCP for the same `server_failed` event, and on the dashboard's Events page.
- A fixture whose check sleeps past 5 seconds reads `HTTP health check failed (timed out after 5 seconds)`.
- A fixture that stops listening reads a reason containing `connection refused` (or `i/o timeout`).
- A deploy that never passes its health check ends its failure reason with the same last result, on every surface and on the deploy page.
- When no result was observed, every surface keeps today's "The instance failed its health check" and never invents a cause.
- No tenant pod rolls because the helper changed.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w6` 2026-11-24 (zero-downtime deploys and health checks), item 2, filed by `/pm them to w6`. At `2f964da9e8`:
  - `server_failed`'s `failureReason.unhealthy` is always the fixed `unhealthyMessage`, "The instance failed its health check" (`lego/backend/internal/events/rest.go:212-214, 235-236`; its comment: "the fact records which check, not its output").
  - A deploy's probe-stall reason names what was checked (`GET {path} on port {port}`), not what it got (`lego/types/v1alpha1/deploy_reasons.go:182-188`).
  - The helper already fails with `health check answered %d` (`lego/operator/internal/healthprobe/healthprobe.go:70`), and the kubelet puts an exec probe's output in the pod's `Unhealthy` event. The operator can only create, patch and update events (`lego/operator/config/rbac/role.yaml`), so nothing reads it.
  - w4/m327 changed only the copy. Board search for `health check failed with status`, `timed out after 5 seconds`, `unhealthyMessage`, `connection refused` and `i/o timeout` found no owner. w4/664 is a different bug.
- **Render:** [render.com/docs/troubleshooting-deploys](https://render.com/docs/troubleshooting-deploys) (read 2026-11-24): "A custom HTTP health check can fail during startup or while running, emitting an event with one of the following errors:" `HTTP health check failed (timed out after 5 seconds)` and `HTTP health check failed with status code <N>`; "If your service restarts due to health check failures, the event can include `connection refused` or `i/o timeout` errors."
- **Recommendation, not a user decision:**
  - When the operator sees a readiness failure, it lists that pod's `Unhealthy` events on demand. That needs namespaced `get`/`list` on events and no watch. It records the newest message, trimmed to Render's wording, on the App's status.
  - The alternative is a file the helper writes for a node-side reader. It needs no RBAC but adds a second channel, and the kubelet's TCP probe never writes it.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). It also serves ADR008's AI-native goal: an agent diagnosing a failed deploy over MCP needs the cause.
- **Expected outcome:** a user who follows Render's troubleshooting guide finds the message it describes, and a deploy failure says why the check failed.
- **Why now:** w6 has the health-probe helper in hand (w6/170, w6/176, and m624, which is parked). t002 can fix the helper's output before m624 resumes and changes how the helper is mounted, so the two do not collide.
- **Render parity:** included. REST, GraphQL, MCP, webhooks and the dashboard all change.
