# w1 · m156 — A hibernated free service whose latest deploy failed never wakes: the failed-release gate halts the reconcile before replicas and routing

**Worker:** worker1 **Goal:** a free service whose newest release failed (its pre-deploy command, or its build) still sleeps and wakes on the release that is actually serving. A request gets the wake response and then the prior release's own reply, and the service never reads Running while its URL can only answer `503 service hibernated`. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                  | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Runtime convergence survives a terminal failed release: a serving prior release still wakes, hibernates, scales and routes while the failed verdict stands | 1h  | —          |
| t002 | Blast radius: every halt before the Deployment write, against every runtime transition that needs that write                                            | 45m | t001       |
| t003 | Live: reproduce and then verify the wake over a failed pre-deploy (and probe the failed-build variant) on production                                     | 40m | t002       |
| t004 | Render parity                                                                                                                                          | 20m | t003       |
| t005 | Simplify                                                                                                                                               | 15m | t004       |
| t006 | Test coverage                                                                                                                                          | 40m | t004       |
| t007 | Closeout                                                                                                                                               | 10m | t006       |

## Definition of done

Repeat on a throwaway free web service (`bex-co/bex` `examples/hello-go`, docker) that is live, returns `200`, and receives no traffic. Only states observed at filing time are listed:

- **It wakes over a failed pre-deploy.**
  1. Set Pre-Deploy Command `echo qa; exit 3`, so the latest deploy reads `pre_deploy_failed`.
  2. Leave the service idle until `service_hibernated`.
  3. Request its URL every second.

  The first requests may get the activator's `503 {"error":"service hibernated","retryAfter":5}`. Within 60 s they get the prior release's own `200`. At filing time 188 of 188 requests over five minutes (07:30:02–07:35:01Z) got that `503`, and so did three more at 07:41:41–50Z, although `service_woken` and `server_available` had been recorded at 07:26:17Z.
- **The phase does not claim Running while the URL cannot serve.** At filing time `GET /v1/services/<srv>` read `phase: Running` and the dashboard header read "Service Running · Latest deploy: Pre-Deploy Failed" throughout those `503`s.
- **The failed release stays unrolled.** After the wake, the pod serves the prior release's response, the failed deploy stays `pre_deploy_failed`, and no new pre-deploy Job runs for that release.
- **Control (must not regress).** A hibernated free service whose latest deploy is live wakes on request. At filing time `qa-20260915-m151ws` answered `503` at 07:30:00 and 07:30:06 and `200` at 07:30:12.

The failed-build variant was traced, not observed. t003 probes it and adds its bullet here if it reproduces.

## Evidence (probes run 2026-09-15, production, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`)

- **Fixture:** `qa-20260915-m149` (`srv-dakeca15v75s738uf84g`), a free web service from `examples/hello-go` (docker) with `MESSAGE=m149-predeploy`. Its first deploy went live at 06:42:54Z. It was created to live-verify w1/m149.

```text
06:44:49Z  PATCH serviceDetails.preDeployCommand "echo qa-pd-marker; exit 3" → dep-dakeio81e15c73bfgpv0 pre_deploy_failed 06:45:11Z
06:44:42–06:51:06Z  URL every 1 s: 234/235 `200 m149-predeploy` (one curl connection failure)
07:07:01Z  service_hibernated (no traffic since 06:51:06Z)
07:22:30Z  production images pinned to c4212ec71 (w1/m146–m155)
07:26:17Z  server_available · service_woken — no later hibernation event
07:30:02–07:35:01Z  URL every ~1.6 s: 188/188 `503 {"error":"service hibernated","retryAfter":5}`
07:30:11Z  PATCH "echo qa-pd-marker2; exit 3" → dep-dakf80pvi8js739uild0 pre_deploy_failed 07:30:26Z, phase Running
07:31:35Z  PATCH "echo qa-pd-marker3; exit 3" → dep-dakf8lpvi8js739uilfg pre_deploy_failed 07:31:57Z, phase Running
07:32Z     dashboard header: "Service Running" · "Latest deploy: Pre-Deploy Failed"
07:41:03Z  GET /v1/services/srv-dakeca15v75s738uf84g → phase Running, numInstances 1
07:41:41Z / 07:41:46Z / 07:41:50Z  curl → 503 {"error":"service hibernated","retryAfter":5}
```

- **Controls in the same window.**
  - `qa-20260915-m151ws` (latest deploy live) hibernated at 07:26:17Z and woke on request: `503`, `503`, then `200` at 07:30:12.
  - `qa-20260915-m151` woke at 07:18:01Z and then served steady traffic.

## Root cause

- **The pre-deploy gate halts before the runtime writes.** `Reconcile` computes `desiredReplicas` at `lego/operator/internal/controller/app_controller.go:2022`, where a waking App is no longer `autoHibernating`. It then runs the pre-deploy gate only when `!app.Spec.Suspended && !autoHibernating` (`:2051-2055`), and returns on `halt` before the Deployment `CreateOrUpdate` (`:2057`) and `ingressBackend` (`:2118`).
- **A stored failure halts every pass.** The persisted `PreDeployFailed` branch (`:4395-4400`) calls `failPreDeploy`, which returns `halt=true` on every pass of that release generation (`:4515-4527`).
  - Hibernating works because the gate is skipped while `autoHibernating`, so replicas 0 are written.
  - Waking re-enters the gate and halts, so replicas stay 0 and the Ingress stays on the activator alias until a new release generation.
- **It predates w1/m149.** The same gate sat in the same place before it (`778bdd1aa^:app_controller.go:2042-2046`), and its `failPreDeploy` also halted, via `r.fail`. m149 only changed the reported phase from Failed to Running, which makes this state newly misleading.
- **The build path has the analogous gate.** `buildFromSource` halts on `terminalBuildFailureRecorded` (`:755-756`), and `resolveDeployImage`'s halt returns before `dispatchRuntime` (`:591-594`). A failed build over a hibernated serving release should block the wake the same way; this is traced, not probed.
- **Why the tests missed it.**
  - `TestPreDeployFailureOverParkedReleaseStaysHibernated` (`predeploy_prior_release_phase_test.go:115`) and `TestBuildFailureOverParkedReleaseStaysHibernated` (`failed_build_prior_release_phase_test.go:94`) call `failPreDeploy` or `r.fail` directly on a fake client and pin only the phase.
  - `TestWakeRestoresAppServiceAsIngressBackend` (`auto_hibernate_wake_test.go:199`) wakes an App that has no failed release.
  - No test wakes a parked App whose newest release failed.

## Blast radius

- **Who is hit.** Every free web service whose newest pre-deploy command or build failed, from its next sleep until the owner ships a new release. Auto-hibernation is the created default since `w6/m116`. The dashboard meanwhile reports the service as Running.
- **Other transitions.** Any runtime change that needs the Deployment or Ingress write while a failed verdict stands: resume after suspend, manual scale, autoscale, maintenance, custom domains and the IP allow-list. t002 enumerates them.
- **Paid services.** They do not auto-hibernate, so they keep serving; t002 still checks suspend and resume.

## Adjacent classes

- **Phase truth.** The prior-release settle derives Running or Hibernated from the Deployment's desired replicas. When routing is stuck on the activator, neither is true.
- **Registry credentials.** `convergeRegistryCredentials` (`:598-628`) already runs before the build gate for this reason: "a failed newer build deliberately leaves the previous Deployment serving". Replicas and routing need the same treatment.

## Unverified (reasoned, not probed this run)

- **The failed-build variant.**
- **The 07:26:17Z events.** Whether `service_woken` and `server_available` came from a real scale-up or from a projection of the phase after the operator restarted onto `c4212ec71`.
- **The failed rollout deadline.** Whether a service whose newest rollout failed its deadline is affected. Probably not, because `settleFailedRollout` (`:2698`) runs after the Deployment write.

## Dedupe

- **Board search.** `grep -rliE hibernat .pm | xargs grep -liE 'pre-?deploy|failed build|build fail'` finds these, and none covers waking over a failed release:
  - `w4/done/m103`: a first deploy's rollout phase;
  - `w3/done/m19`: the activity timeline;
  - `w6/done/040`: live verification of `m94`'s hibernate routing;
  - `w6/done/m97`: the protected-secret guard;
  - `w6/done/m124`: the build phase rule;
  - `w1/done/074`: RBAC;
  - `w1/101`: a build failure after a first release that never served reads Running.
- **Neighbours.** `w1/m149` covers the phase only; this was found while verifying it live. `w6/m94` is the wake routing hold, and `w2/m82` t006 the terminal build gate.

## Source + Goal linkage

- **Source:** live verification of w1/m146–m155 on production, 2026-09-15 (`/loop-worker w1`), on the w1/m149 fixture.
- **Goal linkage:**
  - `docs/ADR004-app-deployment.md`: a failed deploy leaves the previous revision serving;
  - `docs/ADR018-render-parity.md`: Render keeps the previous deploy live after a failed deploy, and a free service spins up on the next request;
  - `w6/m116`: free-service defaults.
- **Expected outcome:** a free service with a failed newest deploy still wakes and serves its previous release, and its phase matches what its URL does.
- **Why now:** it is silent and permanent. Every free service whose newest pre-deploy or build failed becomes unreachable after its first sleep, while the dashboard says Running, and free is the default plan.
