# w1 · m661 — A multi-instance deploy's page shows "Scaling: N of M instances", as on Render

**Worker:** worker1 **Goal:** the deploy header of a service that runs more than one instance, or autoscales, shows Render's **Scaling** fact, how many instances run this deploy out of how many it needs, live while the deploy rolls out. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's Scaling rule in `deploy-detail-page.md` | 30m | — |
| [t002](t002.md) | GraphQL `ServiceInstance` answers the deploy each instance runs and its status | 2h | t001 |
| [t003](t003.md) | The deploy header's Scaling fact, in all eight locales | 1h30m | t002 |
| [t004](t004.md) | The deploy page's pending frame still matches its ready state at 1440 and 390 px | 20m | t003 |
| [t005](t005.md) | Walk a rolling deploy on dev-1 and on production | 45m | t004 |
| [t006](t006.md) | Render parity | 20m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 30m | t006 |
| [t009](t009.md) | Closeout | 15m | t007, t008 |

## Definition of done

1. On production, as the QA user, a disposable paid web service with 2 instances: during a deploy its page shows Scaling "0 of 2 instances", then "1 of 2", then "2 of 2" once live, without a reload.
2. A single-instance service without autoscaling shows no Scaling fact, as on Render. An autoscaled service counts against its minimum, and never shows fewer total than deployed (Render's `max(deployed, total)`).
3. The fact shows only on a live, queued or in-progress deploy (or one that was in progress when the page opened), as on Render.
4. GraphQL `ServiceInstance` carries the deploy id and status; REST `GET /v1/services/{id}/instances` and MCP keep Render's pinned `serviceInstance` (`id`, `createdAt`).
5. The deploy page's pending frame matches its ready state at 1440 and 390 px, in all eight locales.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w1` (2026-11-26), verified the same day before filing. Render's dashboard bundle (entry `index-D5V4ZgbE.js`): `DeployDetailsHeader` renders a **Scaling** fact, "`<deployed>` of `<total>` instances", and `index`'s `P$` computes it: `deployed` counts the service's instances whose `deployID` is this deploy and whose status is succeeded or running; `total` is the autoscaling minimum when autoscaling is on, else `numInstances`; shown only when autoscaling is on or the service runs more than one instance, and only on a live, queued or in-progress deploy (`useDeployDetails`). No flag gates it (the step timeline's `instance_overview` flag gates only the trace queries, see [w1/155](../blocked/155.md)).
- **bex today (verified 2026-11-26):** the deploy header puts Pre-deploy "where Render's Scaling sits" (`dashboard/src/features/deploys/components/deploy-header.tsx:67`). GraphQL `serviceInstances(id)` answers `ServiceInstance { id createdAt }` (`lego/backend/internal/api/testdata/schema.graphql:2832,3514`; `apps.Service.ListInstances`, `lego/backend/internal/apps/service.go:1778`), with no deploy or status. Render's public `serviceInstance` is also only `{id, createdAt}` (pinned `render-public-api-1.json`), so REST and MCP already match and stay as they are; the gap is GraphQL and the dashboard. Pods carry `app.bex.co/revision` (`lego/types/v1alpha1/labels.go:33`), and deploy rows carry their release generation, so an instance can be tied to its deploy.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** a user watching a deploy of a multi-instance or autoscaled service (w1/m20) sees it roll out one instance at a time, as on Render, instead of one status until it is done.
- **Why now:** bex's header already holds a placeholder where Render's fact sits, and the instance-to-deploy link is the one fact missing; zero-downtime rollouts roll one instance at a time (ADR018 § Deploys: Zero-downtime rollout), so the progress exists and is not shown.
- **Not duplicated (checked 2026-11-26 across every workstream):** w4/664 (blocked) is about scale-up events, w4/m173 (blocked) about instance metering; neither shows rollout progress.
- **Render parity task:** included (t006), because the milestone adds GraphQL fields and dashboard UI.
