# w5 · m369 — A deploy's verdict, a Suspend→Resume and a rollout's start reach every surface within seconds, not on the next 30 s pass

**Worker:** worker5 **Goal:** bex-api records an App's deploy transitions within seconds of the operator writing them, a quick Suspend→Resume always records its `service_suspended`/`service_resumed` pair, and a deploy with no build starts at the operator's own dispatch time. REST, GraphQL, MCP, webhooks, push and the dashboard move together because they all read the rows this pass writes. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Measure a reconcile pass first: a pass-duration histogram, a kick counter, and the `bex_controlplane_` family kept by Prometheus | 40m | — |
| [t002](t002.md) | Watch App status in bex-api and kick the reconciler on the transitions a deploy row reads; the 30 s resync stays as the backstop | 1h45m | t001 |
| [t003](t003.md) | A Suspend→Resume records its pair however quickly the two follow each other (w4/604) | 50m | — |
| [t004](t004.md) | A deploy with no build window starts at the operator's dispatch of its release (w4/609) | 40m | — |
| [t005](t005.md) | Render parity | 20m | t002, t003, t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 40m | t005, t006 |
| [t008](t008.md) | Closeout (live replay on production) | 30m | t007 |

Total: **8 tasks, about 5h20m**.

## Definition of done

All on production, workspace `bex-canary`, with the QA account, after the deploy that carries this milestone:

1. **A failed build reads Failed at once.** A Free Go web service from `https://github.com/bex-co/bex`, root `examples/hello-go`, `buildCommand: go build -o app ./does-not-exist` (w4/624's fixture): REST `GET …/deploys/{id}` reads `build_failed` within 10 s of the build log's `exited with code 1` (round 250 measured ~46 s), and a workspace webhook subscribed to the Deploy events receives `build_ended` and `deploy_ended` within 15 s of it.
2. **A successful deploy reads Live at once.** A good deploy of the same service reads `live` within 10 s of its instance turning Ready (today up to ~30 s).
3. **A quick Suspend→Resume is never lost.** Three runs of `POST …/suspend` then `POST …/resume` 5 s apart each record one `service_suspended` and one `service_resumed` (resume listed above), and the webhook receives both (round 189 lost runs 1 and 3 of 3).
4. **An image deploy starts when it started.** A prebuilt-image deploy's `startedAt` is no later than its new instance's `createdAt`, and `==> Deploying image` prints before the app's first log line (round 207: 7–10 s late, after the app's lines).
5. **The pass is measured.** The pass-duration histogram is on obs.bex.co (ADR088) with a production baseline recorded in § Findings, and the 30 s resync still converges a change when the watch is down (covered by a test).
6. Every fixture is deleted and each GET answers 404.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` round 9, item 1 (2026-11-12), filed by `/pmx 1,2,3,4` with w5 chosen. Re-checked at filing against `origin/main` `98b3587a8`.
- **Absorbs three w4 notes** (moved to `w4/done/` as source evidence, not fixed by the move): [w4/624](../../w4/done/624.md) (failed build reads in progress ~45 s; the root cause, a 30 s resync with no reaction to cluster status), [w4/604](../../w4/done/604.md) (a Suspend undone within ~5 s records no `service_*` pair, so no push or webhook) and [w4/609](../../w4/done/609.md) (an image deploy's `startedAt` is the observing pass, 7–10 s late). Their evidence, root causes and test lists are the specification of t002–t004.
- **Mechanism (bex today):** the operator has a build's verdict within ~5 s (`build.Ensure`, `lego/operator/internal/build/build.go:785-805`; re-observed every 5 s, `app_controller.go:1059`). bex-api reads App status only in `ReconcileOnce` (`lego/backend/internal/store/reconciler.go`), on a 30 s timer (`defaultResyncPeriod`, `:83`; `Run`, `:469`; `core.PollWake`) or an API-write `Kick` (`:461`). Nothing kicks on a cluster-side status change, and the client is uncached (`lego/backend/cmd/api/main.go:248`), so there is no informer to hook. bex-api already holds `watch` on `apps.app.bex.co` cluster-wide (`lego/operator/config/api/rbac.yaml:21-23`). The dashboard already polls deploys every 3 s (`dashboard/src/features/deploys/hooks/use-deploy.ts:16`), so the server is the slow part.
- **Also found at filing:** bex-api's Prometheus keep-list (`deploy/gitops/base/prometheus.yaml`, the `bex_(billing|agent_session|…|usage)_.*` regex) has no `controlplane`, so the existing `bex_controlplane_observed_state_rejections_total` (`store/reconciler_metrics.go:34`) is never stored. t001 adds the family.
- **Goal linkage:** Render parity first (ADR008): on Render a failed build reads Failed as its error prints, and both lifecycle notifications of a quick suspend/resume go out (the rule w4/527 set). GOAL item 2, basic observability for operation.
- **Expected outcome:** CI jobs polling a deploy, people watching it, webhooks, push and email learn its verdict ~40 s sooner, and no lifecycle notification is lost to sampling.
- **Why now:** every deploy pays this latency, and the QA loops keep filing its symptoms one at a time (w4/604, w4/609, w4/624; w8/287 is a likely sibling).
- **Render parity:** included. The rows feed REST, GraphQL, MCP, webhooks and the dashboard.
