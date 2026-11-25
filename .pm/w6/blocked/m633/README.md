# w6 · m633 — A multi-instance service's deploy and restart keep every old instance serving until its replacement is healthy, as Render's zero-downtime deploy does

**Worker:** worker6 **Goal:** no deploy or restart of a service with four or more instances takes an old instance out of service before a new one is healthy. Every RollingUpdate App Deployment rolls with `maxUnavailable: 0`, and a release that fails its health check leaves every old instance serving. **Status:** blocked (t001–t003, t006 and t007 done; t004 waits for a production pin carrying `ab720955e1`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [ADR004 amendment: no old instance leaves before its replacement is healthy](done/t001.md) — **DONE** | 20m | — |
| t002 | [The operator rolls every RollingUpdate App Deployment with `maxUnavailable: 0`](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [envtest: 4 and 100 instances keep every old instance ready through a deploy, a restart and a failing release](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [Production check: a 4-instance web service in `bex-canary` never drops below 4 ready instances](t004.md) | 45m | t003 |
| t005 | [ADR018 records the multi-instance rollout and its measurement](t005.md) | 15m | t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 20m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 30m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007 |

## Definition of done

- On production, a paid web service in `bex-canary` (disposable fixture) scaled to 4 instances keeps 4 ready endpoints at every 1-second sample through a code deploy and a restart. Each rollout adds new instances before it removes any old one.
- A release of the same fixture that fails its health check keeps the 4 old instances ready until the rollout budget ends, and the deploy closes failed with the previous release still serving.
- envtest shows the same for 100 instances (the CRD maximum), and shows that rewriting the strategy on an existing Deployment changes no pod-template hash.
- A disk-bearing service still rolls with `Recreate`, and a held web-image release still uses w4/571's `100%`/`0` strategy.
- ADR004 and ADR018 record the rule, Render's sources and the measurement.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w6` 2026-11-24 (zero-downtime deploys and health checks), item 1, filed by `/pm them to w6`. At `2f964da9e8`, an unheld App Deployment rolls with Kubernetes' default `maxSurge: 25%` / `maxUnavailable: 25%`: `defaultRollingUpdateStrategy` (`lego/operator/internal/controller/deployment_projection.go:941`) and `projectRolloutHold` (`:956`). Only a held web-image release gets `100%`/`0` (`heldReleaseStrategy`, w4/571). `maxUnavailable` rounds down, so from 4 instances a rollout deletes old pods the moment it starts: 1 of 4, and 25 of 100 (`app_types.go:792`). A release that never becomes healthy leaves the service a quarter short for up to the 15-minute rollout budget, and a restart behaves the same. ADR004 reasons only about `replicas: 1`. Board search for `maxUnavailable` and `one instance at a time` found only platform components and the held release.
- **Render:** [render.com/docs/deploys](https://render.com/docs/deploys) (read 2026-11-24): "your original instance continues to receive all incoming traffic while the new instance is spinning up", and "For services that are scaled to multiple instances, Render performs steps 2-5 for one instance at a time". A restart is "a special form of manual deploy" and "applies to all instances". [render.com/docs/health-checks](https://render.com/docs/health-checks) says, of new deploys, "If all new instances pass at the same time, the deploy succeeds and traffic moves to them".
- **Recommendation, not a user decision:** set `maxUnavailable: 0` and keep `maxSurge: 25%` (rounded up, so at least 1).
  - Render's two pages disagree on how many new instances start at once, but both keep every old instance serving until the new ones are healthy. `maxUnavailable: 0` is the parity-sufficient change.
  - `maxSurge: 1` is not chosen because bex-api's `DeployGateTimeout` gives the rollout phase 18 minutes of wall-clock (`lego/backend/internal/store/reconciler.go:1908`). A 100-instance rollout that starts one instance at a time could exceed it.
  - Starting exactly one instance at a time is a separate follow-up, filed only with a Render measurement.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)); the availability of the hosting core.
- **Expected outcome:** a service scaled to several instances, by hand or by autoscaling, keeps its full capacity through every deploy and restart.
- **Why now:** an autoscaled service has the most instances under peak load, which is when a deploy now removes a quarter of them. The change is small, and a strategy write rolls no pods. w6 changed the same projection in m547, m595 and m624.
- **Render parity:** omitted. The change is operator mechanism only: no REST, GraphQL, MCP or dashboard field or behavior other than availability changes, so there is nothing to compare across surfaces.
