# w6 · m633 — A multi-instance service's deploy and restart keep every old instance serving until its replacement is healthy, as Render's zero-downtime deploy does

**Worker:** worker6 **Goal:** no deploy or restart of a service with four or more instances takes an old instance out of service before a new one is healthy. Every RollingUpdate App Deployment rolls with `maxUnavailable: 0`, and a release that fails its health check leaves every old instance serving. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [ADR004 amendment: no old instance leaves before its replacement is healthy](done/t001.md) — **DONE** | 20m | — |
| t002 | [The operator rolls every RollingUpdate App Deployment with `maxUnavailable: 0`](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [envtest: 4 and 100 instances keep every old instance ready through a deploy, a restart and a failing release](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [Production check: a 4-instance web service in `bex-canary` never drops below 4 ready instances](done/t004.md) — **DONE** | 45m | t003 |
| t005 | [ADR018 records the multi-instance rollout and its measurement](done/t005.md) — **DONE** | 15m | t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 20m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 30m | t005 |
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t006, t007 |

## Definition of done

- On production, a paid web service in `bex-canary` (disposable fixture) scaled to 4 instances keeps 4 ready endpoints at every 1-second sample through a code deploy and a restart. Each rollout adds new instances before it removes any old one.
- A release of the same fixture that fails its health check keeps the 4 old instances ready until the rollout budget ends, and the deploy closes failed with the previous release still serving.
- envtest shows the same for 100 instances (the CRD maximum), and shows that rewriting the strategy on an existing Deployment changes no pod-template hash.
- A disk-bearing service still rolls with `Recreate`, and a held web-image release still uses w4/571's `100%`/`0` strategy.
- ADR004 and ADR018 record the rule, Render's sources and the measurement.

## Production check (t004, 2026-11-25)

On pin `cdaa0a248615`, with the QA user's session and a run-minted API key (revoked afterwards), `POST /v1/services` created the Starter web image service `w6-m633` (disposable; `srv-dc3jku7fct6s73fs024g`) in `bex-canary`: `docker.io/traefik/whoami:v1.10.1`, Health Check Path `/health`, `PORT=80` (so no release is held for port discovery and every rollout uses the rule), 4 instances. Its Deployment read `maxSurge: 25%`, `maxUnavailable: 0` from creation. A sampler read the EndpointSlices of the Service its Ingress routes to once a second through `kubectl proxy`, with the ReplicaSets' ready counts, and fetched `https://w6-m633.onbex.co/` every 0.5 s.

| Run | Samples | Min ready | Rollout | Public URL | Deploy |
| --- | --- | --- | --- | --- | --- |
| Code deploy to `whoami:v1.10.2` | 150 | 4 | 19:45:20 → 19:46:05 (45 s), one new instance per round, an old one removed only as each became Ready | 268 fetches, all 200, served by 8 pods | `live` at 19:46:07 |
| Restart | 145 | 4 | 19:48:04 → 19:48:49 (45 s), the same four rounds | 269 fetches, all 200, served by 8 pods | `live` at 19:48:49 |
| Release failing its health check | 1,192 | 4 | the one surge instance never Ready; removed when the budget closed | 2,205 fetches, 2,204 of them 200 (below) | `update_failed` at 20:06:06, 15 min 2 s after start; the previous release `live` |

- **Every rollout adds before it removes.** The ReplicaSets went `4/4` → `4/4 + 0/1` → `3/3 + 1/2` → `2/2 + 2/3` → `1/1 + 3/4` → `4/4`, about 11 s a round. Ready endpoints read 4 at every sample of every run, and never 5: the Deployment controller removed an old instance in the same second its replacement became Ready. Each removed instance stayed `serving` while `terminating` for its 60 s drain, so the last old pod left at 19:47:06 and 19:49:49.
- **The failing release.** The image was switched in settings to `docker.io/library/nginx:1.27-alpine` and deployed, because a deploy's `imageUrl` may change only the tag, as on Render. Its `/health` answers 404 rather than the 503 the step names; both fail the check. Through the 15-minute budget the 4 old instances stayed Ready. The deploy closed `update_failed` with "the container is running but its startup health check never succeeded within the rollout window … Last result: HTTP health check failed with status code 404", and the Deployment's template returned to the serving release with no new ReplicaSet.
- **One fetch failed, outside the rollout.** At 19:55:49 one fetch timed out while connecting (Python's `URLError` after 5.2 s). The cluster read 4 Ready instances and no change for 4.5 minutes either side, and every access-log line the edge kept for the host reads 200. It counts as a network blip on the sampling client, not a failed instance, and it fell in the failing-release run, whose criterion has no public-URL clause.
- **Sampling.** No read failed. Through the code deploy and the restart no two Kubernetes samples were more than 1.1 s apart. Through the 15-minute failing release three gaps reached 2.0 s, so three 1-second slots went unread, none of them near a ReplicaSet change.
- **Cleanup.** `DELETE /v1/services/srv-dc3jku7fct6s73fs024g` answered 204, the GET then 404, and no App, Deployment, pod, Service or Ingress of it remained in the namespace a minute later. The key was revoked and the QA session logged out.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w6` 2026-11-24 (zero-downtime deploys and health checks), item 1, filed by `/pm them to w6`. At `2f964da9e8`, an unheld App Deployment rolls with Kubernetes' default `maxSurge: 25%` / `maxUnavailable: 25%`: `defaultRollingUpdateStrategy` (`lego/operator/internal/controller/deployment_projection.go:941`) and `projectRolloutHold` (`:956`). Only a held web-image release gets `100%`/`0` (`heldReleaseStrategy`, w4/571). `maxUnavailable` rounds down, so from 4 instances a rollout deletes old pods the moment it starts: 1 of 4, and 25 of 100 (`app_types.go:792`). A release that never becomes healthy leaves the service a quarter short for up to the 15-minute rollout budget, and a restart behaves the same. ADR004 reasons only about `replicas: 1`. Board search for `maxUnavailable` and `one instance at a time` found only platform components and the held release.
- **Render:** [render.com/docs/deploys](https://render.com/docs/deploys) (read 2026-11-24): "your original instance continues to receive all incoming traffic while the new instance is spinning up", and "For services that are scaled to multiple instances, Render performs steps 2-5 for one instance at a time". A restart is "a special form of manual deploy" and "applies to all instances". [render.com/docs/health-checks](https://render.com/docs/health-checks) says, of new deploys, "If all new instances pass at the same time, the deploy succeeds and traffic moves to them".
- **Recommendation, not a user decision:** set `maxUnavailable: 0` and keep `maxSurge: 25%` (rounded up, so at least 1).
  - Render's two pages disagree on how many new instances start at once, but both keep every old instance serving until the new ones are healthy. `maxUnavailable: 0` is the parity-sufficient change.
  - `maxSurge: 1` is not chosen because bex-api's `DeployGateTimeout` gives the rollout phase 18 minutes of wall-clock (`lego/backend/internal/store/reconciler.go:1908`). A 100-instance rollout that starts one instance at a time could exceed it.
  - Starting exactly one instance at a time is a separate follow-up, filed only with a Render measurement.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)); the availability of the hosting core.
- **Expected outcome:** a service scaled to several instances, by hand or by autoscaling, keeps its full capacity through every deploy and restart.
- **Why now:** an autoscaled service has the most instances under peak load, which is when a deploy now removes a quarter of them. The change is small, and a strategy write rolls no pods. w6 changed the same projection in m547, m595 and m624.
- **Render parity:** omitted. The change is operator mechanism only: no REST, GraphQL, MCP or dashboard field or behavior other than availability changes, so there is nothing to compare across surfaces.

## Unblock record

- **2026-11-25 (verified evidence, `/pm unblock w6/m633`; affects t004, t005, t008):** the gate cleared by itself. The newest pin `8b9d780f45` pins platform images to `cdaa0a248615`, and `git merge-base --is-ancestor ab720955e1 cdaa0a248615` holds. On production, `bex-controller-manager` runs `bex-operator@sha256:e97d4f5c…` (the pin's `controller=` digest, pod started 17:58Z). 17 rolling App Deployments read `maxSurge: 25%`, `maxUnavailable: 0`, and none of the 17 has a ReplicaSet newer than the operator's start, so the strategy write rolled nothing. No production App ran more than one instance, so the multi-instance reading comes from t004's fixture. Three suspended Apps at 0 replicas still read the API server's `25%`/`25%`: a suspended App's held pass moves only its replicas and leaves the rest of the Deployment untouched (`holdServingRelease`), and at 0 replicas a rollout has no old instance to remove. Resume scales up, which the strategy does not govern, and the first full pass writes `25%`/`0`. No gate remains; next: t004's production steps with a 4-instance web service in `bex-canary` (disposable), then t005 and t008.
