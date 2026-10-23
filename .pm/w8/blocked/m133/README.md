# w8 · m133 — A health check that answers 3xx passes, as on Render, instead of following the redirect

**Worker:** worker8 **Goal:** An HTTP health check counts a `2xx` or `3xx` response from the instance as healthy, as on Render ("a `2xx` or `3xx` status code within five seconds"), and never follows the redirect. A service whose health path redirects goes live. **Status:** blocked — t001, t002, t003, t004, t005, t006 done 2026-10-23; the release pipeline builds the new health-probe binary into the bex image and deploys the operator, and a service's next roll adopts the exec probe; QA runs t007 with owned Free echo-server fixtures: a 302→/elsewhere?echo_code=500 health path and a 301→absolute-URL path go live within 2 minutes with no /elsewhere request in the app logs, 404 and 500 paths never go live, /healthz (200) is the control, a path-less service keeps the TCP probe, the probe sends the platform Host; confirm the init container pulls bex's image on the tenant pool; delete fixtures by ID with 404 proofs

## Tasks (in order)

| id   | title                                                                      | est | depends_on                 |
| ---- | -------------------------------------------------------------------------- | --- | -------------------------- |
| t001 — **DONE** | Design + spike: a no-redirect HTTP probe that works for any image          | 45m | —                          |
| t002 — **DONE** | Operator: project the no-redirect probe for startup/readiness/liveness      | 60m | w8/m133/t001               |
| t003 — **DONE** | Rollout and blast radius: existing services, stall/failure copy, timeouts   | 30m | w8/m133/t002               |
| t004 — **DONE** | Render parity: ADR004/ADR018 and the deploy messages agree                 | 15m | w8/m133/t003               |
| t005 — **DONE** | Simplify the changed code                                                  | 15m | w8/m133/t004               |
| t006 — **DONE** | Test coverage: 2xx/3xx/4xx/5xx, relative/absolute redirects, timeout, Host  | 40m | w8/m133/t004               |
| t007 | Close out the milestone                                                    | 30m | w8/m133/t005, w8/m133/t006 |

## Definition of done

Live on production after the release pipeline deploys the operator, with owned Free `docker.io/ealen/echo-server:0.9.2` fixtures (`PORT=80`) deleted by ID with 404 proofs:

- `--health-check-path "/hc?echo_code=302&echo_header=Location:/elsewhere?echo_code=500"` → the first deploy goes `live` within 2 minutes and the app logs show no `/elsewhere` request from the health checker.
- `--health-check-path "/hc?echo_code=301&echo_header=Location:https://example.invalid/"` → `live`.
- `--health-check-path "/hc?echo_code=404"` and `…500` → never live; the deploy fails with the existing stall/failure reason, and the previous release keeps serving on a later deploy.
- `--health-check-path /healthz` (200) → `live` (control). A service with no health check path still uses the TCP probe.
- The probe still sends the `Host` header Render sends (sweep 1294: the service's platform hostname, or its verified custom domain).

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep 9c1e, 2026-10-23 UTC: bex v0.3.3, pin Render CLI v2.27.0 `a764810a7682`, `https://api.bex.co/v1/`, `bex-canary` (`tea-daif693dqjvc73e7as3g`), human device login. Evidence below.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). [render.com/docs/health-checks](https://render.com/docs/health-checks): "Render considers a check successful if the instance responds with a `2xx` or `3xx` status code within five seconds." ADR004:146 already promises "2xx/3xx is ready", and bex's own stall reason says "Check that the path returns a 2xx or 3xx status".
- **Expected outcome:** apps whose health path redirects with a relative `Location` deploy as they do on Render. Common cases are a `/` → `/login` redirect, a trailing-slash redirect to an auth-guarded page, and framework locale redirects. Bex's message stops contradicting its behavior.
- **Why now:** major. The failure is total (the first deploy never goes live) and silent: the stall reason tells the user the 3xx they return is acceptable. Render parity task included: the deploy stall/failure text is shown on REST/GraphQL/MCP and the dashboard.

## Evidence (sweep 9c1e)

```text
$ bex services create --type web_service --name qa-20261022-9c1e-hc302 \
    --image docker.io/ealen/echo-server:0.9.2 --plan free --env-var PORT=80 \
    --health-check-path "/hc?echo_code=302&echo_header=Location:/elsewhere?echo_code=500" --confirm -o json
exit 0 → srv-dbdcdi28ohec73f9fvdg, serviceDetails.healthCheckPath = "/hc?echo_code=302&echo_header=Location:/elsewhere?echo_code=500"
```

`bex deploys list` polled every 20 s from 02:31:19 to 02:39:04 UTC: `update_in_progress` throughout (deploy `dep-dbdcdi28ohec73f9fve0`, created 02:30:32). `GET /v1/services/srv-dbdcdi28ohec73f9fvdg/deploys?limit=1` → 200:

```json
[{"deploy":{"id":"dep-dbdcdi28ohec73f9fve0","serviceId":"srv-dbdcdi28ohec73f9fvdg","status":"update_in_progress","trigger":"other","bexTrigger":"create","image":{"ref":"docker.io/ealen/echo-server:0.9.2"},"createdAt":"2026-10-23T02:30:32.727062Z","updatedAt":"2026-10-23T02:30:46.712023Z","startedAt":"2026-10-23T02:30:46.712023Z","stallReason":"the container is running but its startup health check has not succeeded, so the rollout is waiting: GET /hc?echo_code=302&echo_header=Location:/elsewhere?echo_code=500 on port 80. Check that the path returns a 2xx or 3xx status on the service's own port — an unset Health Check Path falls back to a TCP connect, which only asks whether the process is listening."},"cursor":"dep-dbdcdi28ohec73f9fve0"}]
```

`bex logs -r srv-dbdcdi28ohec73f9fvdg --type app --limit 40 -o json`: 40 entries, all `user-agent: kube-probe/1.34`, 20 for `/hc?echo_code=302…` and 20 for `/elsewhere?echo_code=500`. The prober followed every redirect and scored the `500`. The public URL answered 503 throughout (no ready instance).

At 02:45:49 UTC (15 m 17 s after creation, matching Render's 15-minute window) the deploy ended `update_failed` with `failureReason`:

```text
the container is running but its startup health check never succeeded within the rollout window, so the deploy failed: GET /hc?echo_code=302&echo_header=Location:/elsewhere?echo_code=500 on port 80. Check that the path returns a 2xx or 3xx status on the service's own port — an unset Health Check Path falls back to a TCP connect, which only asks whether the process is listening.
```

On Render the same instance answers `302` and is healthy within seconds.

## Root cause

- `lego/operator/internal/controller/deployment_projection.go:364-375` `healthCheckHandler` projects a Kubernetes `HTTPGetAction` (with the `Host` header) for the startup, readiness and liveness probes.
- The kubelet's HTTP prober follows redirects whose target has the same hostname as the probe URL (the pod IP, so every relative `Location`), up to 10 hops, and scores the final response. It treats a non-local redirect as success without following it. That is Kubernetes `pkg/probe/http` `RedirectChecker(followNonLocalRedirects=false)` (cited from upstream; not vendored here). `HTTPGetAction` has no knob to stop following.

## Fix (target, t001 decides)

1. **Recommended: an exec probe with an injected static helper.** An init container copies a tiny static `bex-probe` binary (built in this repo, no libc) into a shared `emptyDir`. Startup/readiness/liveness run `bex-probe --port <p> --path <path> --host <Host> --timeout 5s`, which makes one GET, never follows redirects, and exits 0 on 200–399. It works for any image, keeps probing the pod itself, and keeps the Host header. Cost: one exec per probe period; confirm on the pinned containerd/kubelet that the period stays within budget.
2. Alternative: a platform-side checker (operator or activator) driving a pod `readinessGate`. Larger, and liveness/restart semantics would need reimplementing.
3. Rejected: rewriting the user's path, or proxying through a sidecar port (changes what the app sees).

## Blast radius

Every web and private service with a Health Check Path, in every workspace. Services without a path keep the TCP probe. Changing the pod template rolls every such service once. t003 sequences this through the normal operator rollout and checks the m121/m120 held-release rules. Liveness restarts (Render: restart after 60 s of failures) must keep their timing.

## Unverified

- Whether Render follows redirects at all (its docs only say 3xx counts). The target follows none.
- Exec-probe overhead at fleet scale (t001 measures).

## Dedupe

Searched `.pm` open and done, ADR004/ADR018 and DO_NOT_DO for health + redirect/3xx/follow: no item. w4/m190 and w4/388 own stall-reason wording and port detection, not redirect scoring.

## Partial live evidence (2026-10-23 11:13–11:18 UTC, platform pin `29d63e1f7bea`, bex v0.3.3, `bex-canary`, `/qa-find-bugs-cli` sweep d4e8)

Fresh free echo-server web `srv-dbdk2l1f0c2s738fsdqg` with `--health-check-path "/hc?echo_code=302&echo_header=Location:/elsewhere?echo_code=500"` reached `live` within ~3 minutes of create (sweep 9c1e's identical fixture never went live and failed at 15 m 17 s). Not covered: the absolute-redirect, 404/500 and no-path controls, app-log check for `/elsewhere`. DoD (t007) still open.

Additional (sweep e9a1, same pin): a scratch-based image with no shell (`traefik/whoami:v1.10.3`, `--health-check-path /health`, `srv-dbdkk09f0c2s738fse80`) went `live` and served 200; a TCP-probe control (no path) went `live`; `--health-check-path "/hc?echo_code=404"` stayed `update_in_progress` with the stall reason naming the path. Fixtures deleted, 404.
