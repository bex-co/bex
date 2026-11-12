# w4 · m353 — An instance that fails its health check is restarted after about 2 minutes, not Render's 60 seconds: the 60-second rollout drain also runs before every liveness restart, while the instance is already out of rotation

**Worker:** worker4 **Goal:** an instance whose health check keeps failing is restarted 60 seconds after its checks started failing, as on Render. The 60-second drain still protects a healthy instance that is being retired (a deploy, a restart, a scale-in). **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Drain only an instance that is still serving: skip the preStop wait when the instance's own health check fails | 1h | — |
| [t002](t002.md) | Cover TCP-checked services (no Health Check Path), and amend ADR004 | 45m | t001 |
| [t003](t003.md) | Simplify | 15m | t002 |
| [t004](t004.md) | Test coverage | 30m | t002 |
| [t005](t005.md) | Closeout (live replay on production) | 30m | t003, t004 |

## Definition of done

All on production, on a Free web service from `POST /v1/services`: `docker.io/traefik/whoami:v1.10`, `dockerCommand: /whoami -port 10000`, `healthCheckPath: /health`.

1. **The restart comes at Render's 60 s.**
   - After Live, `curl -X POST --data 500 https://<name>.onbex.co/health` (whoami's `/health` then answers 500).
   - `curl` `/` and `/health` every 2 s.
   - `/` stops answering within about 15 s, as today (w8/189). `GET /v1/logs?…&type=app` shows the next `Starting up on port 10000` no later than 75 s after `Update health check status code [500]`. Round 229 measured 114 s.
   - `/` answers 200 again within about 15 s of that line. The Events tab shows `server_failed` (`unhealthy`) and then `server_available`.
2. **The same holds without a Health Check Path.** A service checked by TCP, whose listener stops accepting (for example a busybox `httpd` that is killed while the shell keeps PID 1 alive), is restarted no later than 75 s after its checks start failing.
3. **The drain still protects healthy instances (w8/180, w1/m154).** On a Starter web service with two instances, a 70-second request started just before a Manual Deploy completes with 200, as in w8/180's acceptance. The retired instance's last log line comes about 60 s after the new instance is Ready.
4. **Fixtures.** Every fixture is deleted, and each GET answers 404.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 229, 2026-11-12 20:32–20:37 UTC (`muse.env`, workspace `bex-canary`; pin `33e1b1e6117c`). The probes are under § Finding.
- **Governing docs:**
  - [ADR004](../../../docs/ADR004-app-deployment.md) § the health-check paragraph (line 96). It says a wedged instance "self-heals in about a minute" (w7/m81).
  - § Rollout headroom (line 459), the 60 s `lifecycle.preStop.sleep` drain (w1/m154, raised from 10 s by w8/180).
  - w7/m81's DoD was proven by projection tests. Neither later drain change measured a liveness restart.
- **Goal linkage:** Render parity on a documented timing. [Render's health checks](https://render.com/docs/health-checks): after "15 consecutive seconds of failures … Render temporarily stops routing traffic to it", and after "60 consecutive seconds of failures … Render automatically restarts the instance".
- **Expected outcome:** for a one-instance service, an unhealthy instance's outage shrinks from about 112 s (round 229: 503 from 20:34:20 to 20:36:12) to about 60–65 s. Deploys and restarts keep the 60 s drain that w8/180 needed for long requests.
- **Why now:** major (availability). Every one-instance web or private service that wedges — the case w7/m81 exists for — is down twice as long as it should be. No instance can serve during the extra minute: readiness pulled it at 15 s, so the drain protects nothing.
- **Render parity task omitted:** the change is operator-internal (the pod's preStop hook and the probe binary). No REST, GraphQL, MCP or dashboard shape changes; the events stay `server_failed` and `server_available`.

## Finding (round 229)

Fixture `qa-20261112-r229-hc` (`srv-dbr24ia585js73afgis0`), created at 20:32:10 by `POST /v1/services`: Free, `docker.io/traefik/whoami:v1.10`, `dockerCommand: /whoami -port 10000`, `healthCheckPath: /health`. Live; `curl /` 200 from 20:33:58 (pod `…-58497f976-nqpr2`). Deleted at the end of the round.

- **Probe** (`curl` from outside every 2 s, `/` and `/health`):
  - 20:34:05: `curl -X POST --data 500 …/health` → 200. The app logs `Update health check status code [500]` at 20:34:06.318.
  - 20:34:07: `/` 200, `/health` 500.
  - 20:34:20 (+14 s): `/` 503 `no available server`; then from 20:34:29, 503 `{"error":"service not ready","retryAfter":5}`. Removal at about 15 s matches Render (w8/189).
  - 20:36:00.200: the app logs `Starting up on port 10000`, +114 s after the flip. It is the same pod (Hostname still `…-nqpr2`), so the kubelet restarted the container.
  - 20:36:12: `/` 200 again, `/health` 200 (whoami's health state lives in memory).
  - Events: `server_failed {"reason":{"evicted":false,"unhealthy":"The instance failed its health check"}}` at 20:34:26, and `server_available` at 20:36:25.
- **The timing decomposes exactly into liveness plus drain:**
  - liveness is 10 s period × 6 failures (`deployment_projection.go:145-154`, `:380-385`), so the kill decision comes between 20:34:56 and 20:35:06;
  - the kubelet then runs the container's preStop `sleep` of 60 s (`drainSeconds`, `:490-497`; `applyDrain` `:533-543`) before SIGTERM, so SIGTERM lands between 20:35:56 and 20:36:06;
  - whoami exits on SIGTERM and restarts at 20:36:00.
  - Kubernetes runs preStop for liveness kills: its container hooks documentation says PreStop "is called immediately before a container is terminated due to an API request or management event such as a liveness/startup probe failure". The pod's `terminationGracePeriodSeconds` (drain + `maxShutdownDelaySeconds`, `:551-561`) bounds it.

### Mechanism

- `drainSecondsFor` (`lego/operator/internal/controller/deployment_projection.go:503-508`) gives every non-worker, non-disk pod a 60 s `lifecycle.preStop.sleep`. Its purpose, per its comment and ADR004 line 459, is to keep a pod "marked for deletion" serving "before it gets SIGTERM", while its endpoint leaves Traefik.
- A liveness kill is not a deletion, and the pod has already left the endpoints: readiness (5 s × 3) failed about 45 s earlier. Yet the native `sleep` action runs unconditionally, so the restart waits 60 s more.
- A probe-level `livenessProbe.terminationGracePeriodSeconds` cannot fix this alone. Kubernetes counts preStop inside the grace period, so a shorter grace would cut the sleep, then give SIGTERM only the 2-second extension before SIGKILL. The app would lose its graceful shutdown.

## Unverified

- A TCP-checked service (no Health Check Path) has the same preStop, so it should behave the same. It was not probed; t002 covers it.
- Render's own measured restart time was not captured; its docs are the reference.
- Whether Render waits `maxShutdownDelaySeconds` after SIGTERM on a health restart is not documented. bex keeps that SIGTERM-to-SIGKILL window either way.
- **Corroboration (not a new probe):** [w4/m327](../blocked/m327/README.md)'s round 100 ran the same whoami experiment on 2026-11-11. Its flip at 11:41:42 came back as 200 at 11:43:59, 137 s later — the same liveness-plus-drain shape, recorded there without the timing analysis.
