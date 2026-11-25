# w6 · m634 — A failed health check says what the check got back, in Render's words, on every surface

**Worker:** worker6 **Goal:** when an instance fails its health check, at deploy or while live, every surface names what the check got back in Render's words: the status code, the 5-second timeout, `connection refused` or `i/o timeout`. The surfaces are the `server_failed` event's `failureReason.unhealthy` on REST, GraphQL, MCP and webhooks, the deploy's failure reason, and the dashboard. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [ADR004 design: how the probe's last result reaches the App's status](done/t001.md) — **DONE** | 30m | — |
| t002 | [The health-probe helper reports a failure in Render's words](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [The operator records an instance's last health-check result when it fails readiness](done/t003.md) — **DONE** | 1h30m | t002 |
| t004 | [`server_failed` carries the result as `failureReason.unhealthy` on REST, GraphQL, MCP, webhooks and the alert email](done/t004.md) — **DONE** | 1h | t003 |
| t005 | [A deploy that fails its health check ends its failure reason with the last result](done/t005.md) — **DONE** | 45m | t003 |
| t006 | [The dashboard's Events and deploy pages show the result](done/t006.md) — **DONE** | 45m | t004, t005 |
| t007 | [Production check in `bex-canary`: a 503, a timeout and no listener](done/t007.md) — **DONE** | 45m | t006 |
| t008 | [Render parity](done/t008.md) — **DONE** | 30m | t007 |
| t009 | [Simplify](done/t009.md) — **DONE** | 20m | t008 |
| t010 | [Test coverage](done/t010.md) — **DONE** | 45m | t008 |
| t011 | [Closeout](done/t011.md) — **DONE** | 10m | t009, t010 |

## Definition of done

- On production, a disposable web fixture in `bex-canary` that answers its Health Check Path with 503 reads `HTTP health check failed with status code 503` in `failureReason.unhealthy` on REST `GET /v1/events`, GraphQL and MCP for the same `server_failed` event, and on the dashboard's Events page.
- A fixture whose check sleeps past 5 seconds reads `HTTP health check failed (timed out after 5 seconds)`.
- A fixture that stops listening reads a reason containing `connection refused` (or `i/o timeout`).
- A deploy that never passes its health check ends its failure reason with the same last result, on every surface and on the deploy page.
- When no result was observed, every surface keeps today's "The instance failed its health check" and never invents a cause.
- No tenant pod rolls because the helper changed.

## Production check (t007, 2026-11-25)

On pin `cdaa0a248615`, with the QA user's session and a run-minted API key (revoked afterwards), `POST /v1/services` created three Starter web image services in `bex-canary`: `w6-m634-503` (`srv-dc3k5svfct6s73fs032g`), `w6-m634-slow` (`srv-dc3k5tnfct6s73fs034g`) and `w6-m634-stop` (`srv-dc3k5tsg1jgc73dafq5g`), all disposable. Each ran `docker.io/library/python:3.13-alpine` with a Docker Command that serves `PORT=10000`, Health Check Path `/healthz`. `/healthz` answers 200 until a `POST /mode/<m>` switches it: `503` answers 503, `slow` sleeps 7 s, `stop` closes the listener and keeps the process running. A `START_MODE` env var sets the mode a release starts in. Each first deploy went `live` in 12–14 s, and the Deployments probed by exec of the helper's `check`.

**A running instance (`server_failed`).** At 20:21:22Z each fixture was switched to its mode. Within 11–17 s every App read Ready `HealthCheckFailing` with `status.healthCheckFailure` set, and the liveness restart at 60 s brought each back (`server_available` 20:22:56Z–20:23:09Z). One `server_failed` per fixture read the same `unhealthy` on REST (`GET /v1/services/{id}/events` and `GET /v1/events/{id}`, `details.reason.unhealthy`), GraphQL (`serviceEvent(id:)` and `serviceEvents`, `details.unhealthy`, `reasonCode: readiness_failed`), MCP (`get_service_event`, `list_service_events`) and the dashboard's Events page ("A running instance failed its health check: <result>."):

| Fixture | Event | `unhealthy` on every surface |
| --- | --- | --- |
| 503 | `evt-5rav58lpb9gd1m2qi0jn` | `HTTP health check failed with status code 503` |
| slow | `evt-6qvko3n3d3gerkkqlcot` | `HTTP health check failed (timed out after 5 seconds)` |
| stop | `evt-d8rcv7hq26i0iidu9c12` | `HTTP health check failed: dial tcp 10.244.27.146:10000: connect: connection refused` |

**A deploy that never passes (failure reason).** At 20:24:10Z each fixture's `START_MODE` was set to its failing mode and deployed (`POST /v1/services/{id}/deploys`). The new instance never passed, the old one kept serving (the public URL answered 200 throughout and afterwards), and no `server_failed` was recorded while the deploy was open. In flight, `stallReason` and the Events page already ended with the result. Each deploy closed `update_failed` at 20:39:13Z–20:39:22Z, after the 15-minute rollout budget, with the same `failureReason` on REST `GET …/deploys/{id}`, GraphQL `deploy(…)`, MCP `get_deploy`, the `deploy_ended` event's `failureReason` and the dashboard's deploy page: "the container is running but its startup health check never succeeded within the rollout window, so the deploy failed: GET /healthz on port 10000. Check that the path returns a 2xx or 3xx status on the service's own port. Last result: <result>.", where `<result>` read `HTTP health check failed with status code 503`, `HTTP health check failed (timed out after 5 seconds)` and `HTTP health check failed: dial tcp 10.244.26.64:10000: connect: connection refused`.

- **No tenant pod rolled.** The operator and bex-api carrying t002–t005 started at 17:58Z–17:59Z. Of the 354 Deployment ReplicaSets outside the platform namespaces, the only ones created since then are the six of these fixtures.
- **No result recorded.** This was not reproduced live: an instance whose check fails with no readable result needs an errored probe, and `bex-canary` held no readiness-failure event from before the pin. `TestServerFailedUnhealthyAgreesOnEverySurface` (its "no result recorded" case), `TestUnhealthyIsNullOffAFailedHealthCheck`, `TestFailedDeployReasonEndsWithTheLastResult` (no result: byte-identical reason) and the Events route test's fallback cover it.
- **Not checked live.** The alert email and a webhook receiver. A receiver reads the event by id, the `GET /v1/events/{id}` read above, and the email's paragraph has its own unit test.
- **Cleanup.** `DELETE /v1/services/{id}` answered 204 for all three and each GET then 404. By 20:42:37Z no App, Deployment, pod, Service or Ingress of them remained in the namespace. The key was revoked and the QA session logged out.

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
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). It also serves ADR008's AI-native goal: an agent diagnosing a failed deploy over MCP needs the cause.
- **Expected outcome:** a user who follows Render's troubleshooting guide finds the message it describes, and a deploy failure says why the check failed.
- **Why now:** w6 has the health-probe helper in hand (w6/170, w6/176, and m624, which is parked). t002 can fix the helper's output before m624 resumes and changes how the helper is mounted, so the two do not collide.
- **Render parity:** included. REST, GraphQL, MCP, webhooks and the dashboard all change.

## Unblock record

- **2026-11-25 (verified evidence, `/pm unblock w6/m634`; affects t007, t008, t011):** the gate cleared by itself. The newest pin `8b9d780f45` pins platform images to `cdaa0a248615`, and `git merge-base --is-ancestor e3a1842c94 cdaa0a248615` holds; it is the first pin to carry the work (`9cc529d10432` and `7b046987bcd8` do not). On production, `bex-controller-manager` and both `bex-api` pods run `bex-operator@sha256:e97d4f5c…` (the pin's `controller=` digest, started 17:58Z–17:59Z), the dashboard runs the pin's `bex-dashboard@sha256:8f20de75…` (18:00Z), the App CRD carries `status.healthCheckFailure`, and the operator's service account may `list` events but not `get` or `watch` them. No gate remains; next: t007's three fixtures in `bex-canary` (disposable), then t008 and t011.
