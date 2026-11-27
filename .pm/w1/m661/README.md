# w1 · m661 — A multi-instance deploy's page shows "Scaling: N of M instances", as on Render

**Worker:** worker1 **Goal:** the deploy header of a service that runs more than one instance, or autoscales, shows Render's **Scaling** fact, how many instances run this deploy out of how many it needs, live while the deploy rolls out. **Status:** todo (t001–t004 and t006–t008 done; t005 walked DoD 1–3 on production, and its fix for the autoscaled header's frame waits for a pin, then t005's re-check and t009)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's Scaling rule in `deploy-detail-page.md` — **DONE** | 30m | — |
| [t002](done/t002.md) | GraphQL `ServiceInstance` answers the deploy each instance runs and its status — **DONE** | 2h | t001 |
| [t003](done/t003.md) | The deploy header's Scaling fact, in all eight locales — **DONE** | 1h30m | t002 |
| [t004](done/t004.md) | The deploy page's pending frame still matches its ready state at 1440 and 390 px — **DONE** | 20m | t003 |
| [t005](t005.md) | Walk a rolling deploy on dev-1 and on production | 45m | t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t006 |
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

## Unblock (2026-11-27)

- **Affects:** t005 (its production walk), then t009.
- **Source:** verified evidence. The newest `chore(deploy): pin platform images to 6ba91ca5b72f` on `origin/main` (`9c76d4ae72`) pins a commit that carries `19d2b556ac` (`git merge-base --is-ancestor 19d2b556ac 6ba91ca5b72f` exits 0), so the gate `pin must carry 19d2b556ac` holds. The page-scroller fix `83a2ff4630` (w1/m657) is not on that pin, so a window scroll while a frame is held is that fix's to clear, not this milestone's.
- **Remaining gates:** none. No user action is needed.
- **Next:** as the QA user in `bex-canary` on disposable services, walk t005's production steps (DoD 1–3) at 1440 and 390 px, and check the deploy page's pending frame against its ready state in all eight locales, after a hover and with its reads held (DoD 5). Then delete the fixtures, log the session out and run t009.

## Production walk and the autoscaled header (2026-11-27)

t005 walked DoD 1–3 on production pin `6ba91ca5b72f` as the QA user in `bex-canary` on disposable services, and each held ([t005](t005.md#production-walk-2026-11-27-pin-6ba91ca5b72f)): a 2-instance restart read "0 of 2", "1 of 2" and "2 of 2 instances" without a reload at 1440 and 390 px, a single-instance service and a deactivated deploy showed no Scaling, and an autoscaled service counted against its minimum of 2. DoD 5 did not hold for that autoscaled service:

- **What failed.** Its instance count is 1, so only its autoscaling says the header counts instances, and the page read the autoscaling after mounting. At 390 px, where the facts sit two to a row, the fact then added a row and pushed the log down 48 px, hovered or held, in all eight locales. At 1440 px it took the third place of an existing row and nothing moved. By the same count, a repo-built service with a pre-deploy step would move at 1440 px too, its seventh fact opening a third row (not walked).
- **Decision (worker, 2026-11-27; basis: the root `AGENTS.md` rule and `dashboard/AGENTS.md` § Navigation pending states, as w1/m659 decided for the Notices).**
  - The deploy routes' loader reads the service's autoscaling, cache-first, beside the Notices' reads, whenever the header can carry the fact (a web service, private service or background worker on a live, queued or in-progress deploy). The page mounts knowing whether the fact shows.
  - The instances are not read in the loader. The fact's value shows a bar in its own 20 px line until they land, which changes no geometry, and the loader adds no third round trip to a navigation.
- **Not chosen.** Reading the instances in the loader too, which draws the count at mount at the cost of a third sequential read on every scaled-out service's deploy page. Reserving the fact in the ready header whenever the service supports scaling, which would show a Scaling fact Render does not show on single-instance services.
- **dev-1.** With the dashboard from this tree, an autoscaled service (disposable) with an instance count of 1 mounted with the fact and never moved, hovered and held, in all eight locales at 390 px and at 1440 px. With the read switched off it moved 48 px again.
- **Filed:** [w1/179](../179.md), a digest-pinned image reference wrapping the Image fact past its frame at 390 px. It predates this milestone.
