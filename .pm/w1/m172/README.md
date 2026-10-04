# w1 · m172 — A free service that sleeps through a failed rollout wakes onto the broken release: `503 service hibernated` while the API reads Running

**Worker:** worker1 **Goal:** a free service whose newest rollout failed over a prior release wakes on, and keeps serving, the release that last succeeded; its phase never reads Running while its URL can only answer the activator's 503. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Failed rollout over a prior release: restore the served release's pod template so wake, resume and scale run the prior release | 1h  | —          |
| t002 | Phase truth: never report Running from desired scale while no replica of the serving template can become ready                 | 30m | t001       |
| t003 | Live: reproduce on production, then verify wake after a failed rollout (both orderings)                                        | 40m | t002       |
| t004 | Render parity                                                                                                                  | 20m | t003       |
| t005 | Simplify                                                                                                                       | 15m | t004       |
| t006 | Test coverage                                                                                                                  | 40m | t004       |
| t007 | Closeout                                                                                                                       | 10m | t006       |

## Definition of done

Run on a disposable free image web service (`docker.io/mendhak/http-https-echo:35`, `HTTP_PORT=3000`) that is live and returns `200`:

- **It wakes after a failed rollout.** `bex deploys create <srv> --image docker.io/library/memcached:1.6-alpine` closes `update_failed` with the TCP-probe line. Leave the service idle until `service_hibernated`, then request its URL every second: within 60 s the requests get the prior release's own `200`. At filing time every request for 6+ minutes after `service_woken` (20+ samples, 07:08:31–07:15Z on 2026-10-03) got `503 {"error":"service hibernated","retryAfter":5}` while REST read `phase: Running`, `replicas: 1`.
- **Same when the failed rollout starts while the service is hibernated** (the observed ordering): after the row closes, a request wakes the service and it serves the prior release within 60 s.
- **Phase truth.** `GET /v1/services/<srv>` reads Running only while the URL serves the prior release. The failed deploy stays `update_failed` and no new rollout of the failed image starts on wake.
- **Control (no regression).** A hibernated free service whose latest deploy is live still wakes on request (w6/m94 shape); w1/m156's failed-pre-deploy wake and w1/m157's failed-build wake still hold.

## Evidence

`.pm/w8/done/m44/README.md` § "Live closeout 2026-10-02/03", adjacent observation 1. Production pin `1263d12ae3fa`, `bex v0.2.1`, workspace `bex-canary`, fixture `srv-db09v90ehcmc739j12gg` (deleted). The service auto-hibernated at 06:45:36Z during an earlier rollout; the failing deploy `dep-db0acnqtm2ss7389qoc0` started while it was hibernated and closed at 07:08:31Z (`deploy_ended` + `service_woken`). Unverified at filing: the Deployment template and ReplicaSet state (no cluster access that run).

## Mechanism (current `main`)

- **Template stays on the failed release.** `settleFailedRolloutMessage` (`lego/operator/internal/controller/app_controller.go:3026-3036`) over a prior release only stamps `ConditionRollout` and calls `settleFailureOverPriorRelease`; nothing restores the Deployment's pod template. `reconcileKubernetes` keeps applying the failed `image` on every pass (`:2229` `applyServingDeployment`). `holdUnpassedRelease` (`:2207`, `:5404`, w1/m156) holds only a pre-deploy step that has not passed — nothing holds a failed rollout.
- **Waking scales the broken ReplicaSet (inferred).** Parked, both ReplicaSets are at 0; scaling the Deployment 0 → 1 gives the replica to the newest ReplicaSet (the failed template), so `ReadyReplicas` stays 0.
- **Route held on the activator.** `ingressBackend` (`:2629`) keeps the public route on the activator while nothing is ready, so the activator keeps answering `503 service hibernated` (`lego/operator/cmd/activator/main.go:218`).
- **Phase from desired scale.** `settleFailureOverPriorRelease` / `settlePriorRelease` (`:5108-5150`) derive Running from `dep.Spec.Replicas == 1`, so the API says Running while the URL is down — on every sleep/wake cycle until a new successful release.
- w1/m156 § Unverified ("The failed rollout deadline … probably not affected because `settleFailedRollout` runs after the Deployment write") guessed this path safe; this run shows it is not.

## Fix direction

- **t001.** When a rollout settles failed over a served release, re-apply that release's recorded pod template: reuse w1/m152's `servedPodTemplateForCancel` (`lego/operator/internal/controller/release_config_snapshot.go:463-484`, the `ReleaseRecordName` Secret), generalised from cancel to failed rollout, so later passes scale and route the prior release (m156's `convergeServingRuntime` invariant). If the record is missing (GC'd or pre-m152), fall back to scaling the prior ReplicaSet or report Failed honestly — never Running.
- **t002.** `settlePriorRelease` must not report Running when the template it scales is not the served one.

## Related

- [w6/m147](../../w6/m147/README.md) — the same failed-rollout-meets-auto-hibernate family: a park that lands **mid-rollout** erases the stall diagnosis (generic health-gate failure reason) and the dashboard header disagrees with the phase. Its t002 (settle or defer the park of an unsettled rollout) must leave a verdict that this milestone's t001 can restore from; coordinate, do not duplicate.

## Source + Goal linkage

- **Source:** w8/m44 live closeout (2026-10-02/03, `/qa-find-bugs-cli`), adjacent observation 1, filed 2026-10-03; sibling of w1/m156 (failed pre-deploy variant, fixed in `1cb1f2d27`) and w1/m157 (failed build variant).
- **Goal linkage:** free-tier sleep/wake must be invisible to users (ADR008 dense free tier; w6/m116 made auto-hibernate the default); Render keeps running the most recent successful deploy after a failed deploy (ADR018, ADR004 § "A held release does not freeze the serving one").
- **Expected outcome:** one bad deploy no longer takes a free service offline at its next sleep, and the API/dashboard stop saying Running for a URL that only returns 503.
- **Why now:** every free web service with a failed rollout over a live release is down from its next wake until a new successful deploy; w8/042 per-deploy `imageUrl` makes bad-image rollouts a routine CI outcome. m156 and m157 closed the two sibling paths, leaving this one.
- **Render parity included:** it changes the service phase on REST/GraphQL/MCP and the dashboard header.
