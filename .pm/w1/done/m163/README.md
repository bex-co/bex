# w1 · m163 — A cron job or static site whose newest build failed or is still building ignores suspend, resume and schedule edits

**Worker:** worker1 **Goal:** suspend, resume and schedule edits act immediately on cron jobs and static sites even while the newest release has no image, the way they already do for web, private and worker services since `w1/m156`–`m158`. **Status:** done — all eight tasks done; t004 drilled live on production 2026-10-02 (user decision), every DoD bullet observed

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Render evidence: suspend on a cron job or static site acts immediately regardless of the latest build — **DONE** | 20m | — |
| t002 | Extend the hold to cron jobs: converge `suspend` and `schedule` on the CronJob serving the prior image — **DONE** | 50m | t001 |
| t003 | Static sites: suspend and resume over a failed or in-flight build converge the published revision's routing — **DONE** | 40m | t001 |
| t004 | Live: a cron job with a failed newest build stops firing within one schedule tick of suspend — **DONE** | 45m | t002, t003 |
| t005 | Render parity — **DONE** | 20m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 40m | t005 |
| t008 | Closeout — **DONE** | 10m | t007, t004 |

## Definition of done

- **Cron.** After suspend, a cron job whose latest deploy reads `build_failed` (or is still building) records no new run; resume and a schedule edit take effect without a new release; the CronJob never carries the unbuilt release's image. Observed live (t004), not reasoned.
- **Static.** A static site over a failed or in-flight build stops serving on suspend and serves its last published revision on resume.
- **Control (must not regress).** A healthy cron job keeps firing; web/private/worker behavior from `m156`–`m158` is unchanged.

## Root cause

- `Reconcile` returns on `resolveDeployImage`'s halt (`app_controller.go:597-600`) before the runtime dispatch (`:2079` → `reconcileCronJob`, where `spec.suspend` is written at `:3545`).
- `holdPendingArtifact` (`:5019`) deliberately returns `held=false` for cron jobs and static sites via `scalableRuntime` (`:4954`); `m156`–`m158` fixed the class for the scalable types and recorded these two as excluded, not verified.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-09-20, item 3 — traced from code as the last unfixed member of the `w1/m156`–`m158` class.
- **Goal linkage:** `docs/ADR004-app-deployment.md` (a failed deploy leaves the previous revision serving); `docs/ADR018-render-parity.md` (suspend and resume act immediately).
- **Expected outcome:** a tenant can stop a misbehaving cron job or unpublish a static site regardless of build state; no more "suspended but still running" cron jobs.
- **Why now:** the hold mechanism to extend is fresh (`m157`/`m158`) and the exclusion is explicit in code; leaving it is a known, user-facing gap.
- **Render parity:** included — suspend/resume/schedule are exposed on REST, GraphQL, MCP and the dashboard.

## Triage and implementation evidence — 2026-09-21

**Work, with two disproved premises.** Static suspension already bypasses build state in the static-server resolver: it removes the host on refresh (default ten seconds), returns 404, and restores the same published prefix on resume. The new HTTP regression verifies this behavior, including cached bytes. No static hold is necessary. Render’s public docs do not establish the milestone’s immediate-schedule timing assertion; the artifact records that limit.

The cron bug is real, but its pre-fix shape is more precise than the original t004 recipe: a suspended App can reuse the previous image and enter normal cron dispatch, copying the pending command/environment and falsely advancing active revision; resume and schedule edits otherwise stop at the build halt. The fix converges controls and run history using the existing CronJob template, including manual runs, while retaining release identity and build status. Command is release configuration, so t002’s request to apply it to an unbuilt release is deliberately not implemented (`lego/types/v1alpha1/app_identity.go:53`).

**Automated verification:** `GOWORK=off make test` passed (controller package 81.145s); focused failed/queued/running cron matrix passed; static HTTP/resolver tests passed; existing backend cron-surface and suspend/resume tests passed. Simplify’s three reviews completed; its one assertion-diagnostic improvement was applied. These are automated results, not a live scheduling claim.

## Live verification gate — 2026-09-21

**Not completed.** `scripts/dev-env.sh 1 up` failed its preflight because it could not find a kind cluster named `bex`; the current local topology is the CAPD workload cluster behind `bex-mgmt`. The pre-approved `scripts/mock-cluster.sh` run refreshed the existing three-node CAPD baseline (Calico controller placement and local-path configuration), then stalled fetching the pinned kubelet-CSR-approver Helm chart. A separate anonymous Helm pull with a 45-second deadline also timed out. Only this run’s hanging Helm child was terminated; the pre-existing unrelated Helm process and other dev stacks were left alone. No cluster or tenant namespace was deleted.

The local API alternated between a successful `/readyz` response and EOF / 15–20 second request timeouts. The pod inventory showed the Kubernetes scheduler and controller-manager at 0/1 with repeated restarts; CNPG was crash-looping. Two attempts to seed a throwaway cron App using a compiled **pre-fix** reconciler both failed during API discovery (`GET /api?timeout=20s`: EOF), before App creation. Consequently there is no live pre/post scheduling or static curl result to claim, and no App/CronJob fixture was created by the drill. The temporary driver was removed from the working tree.

**Gate (resolved 2026-10-02 — production drill authorized; see § Live drill below):** the operator must restore a healthy local CAPD API/scheduler (without disrupting the other dev stacks), or explicitly authorize a production drill. Then run t004 against the pre-fix and fixed operators with a healthy control cron, verify static HTTP suspend/resume, and complete t008. Automated coverage is green and shippable; the milestone remains blocked rather than done.

**Mutation proof:** on an isolated `f47178f1d` checkout, all three cron matrix cases (failed, running and queued builds) fail because suspension changes the prior pod template; the same cases pass with this fix.

## Live drill — production 2026-10-02

**User decision:** run t004 on production instead of locally. The drill used the QA user through `api.bex.co`, workspace `tea-d98210cbbpdc73dcrkvg`. The prod cluster was read only (`get` only) to observe the CronJob `spec.suspend`, `spec.schedule` and image, and the Jobs. All times are UTC.

**Fix is deployed.** Prod `bex-controller-manager` runs `ghcr.io/bex-co/bex-operator@sha256:e8b8fc78…`, pinned by `eca8e4274` ("pin platform images to db66f833ff3b", 2026-10-02 04:13Z) in `deploy/gitops/base/bex.yaml`. `git merge-base --is-ancestor c29c8fe70 db66f833f` succeeds, so the fix (2026-09-21) is in the running operator. **A pre-fix comparison is not possible on prod** because no pre-fix operator runs there. The pre-fix half rests on the recorded mutation proof above: on `f47178f1d`, all three cron matrix cases fail.

**Fixtures** (all `qa-20261002-`, `autoDeploy: no`, repo `bex-co/bex@main`):

| fixture | id | shape |
| --- | --- | --- |
| `cron-ctl2` (control) | `srv-davke1ptf7js73907leg` | `examples/cron-demo`, runtime `go`, `go build -o cron-demo .` / `./cron-demo`, `*/1 * * * *` |
| `cron-sub2` (subject) | `srv-davke2fibfms73f3us90` | same as the control |
| `static` | `srv-davkb138dvus73e89p10` | `examples/static-site`, publish path `.`, `https://qa-20261002-static.onbex.co` |
| `cron-control`/`subject` | `srv-davkavvdrk5s73fbjng0` / `srv-davkb0j8dvus73e89ovg` | first attempt, `runtime: docker`; never deployed (see finding below), deleted at 05:54 |

**Finding outside this milestone: new Docker services cannot deploy in the `bex` workspace.** Both Docker-runtime crons reached `update_failed` at 05:49:37. The failure was `cronjobs.batch … is forbidden: ValidatingAdmissionPolicy 'bex-operator-workloads' … operator-authored containers outside the execution boundary must be restricted (no added capabilities …)`. Here is the cause. `BEX_IMAGE_COMPATIBILITY_WORKSPACES=tea-d98210cbbpdc73dcrkvg` (`lego/operator/config/prod/kustomization.yaml:107`) gives every new Docker or image service in that workspace `containerPolicy: image-v1`. That policy adds `CHOWN, DAC_OVERRIDE, SETUID, SETGID` (`lego/operator/internal/controller/container_policy.go:29`). But `deploy/gitops/base/operator-workload-admission.yaml:146-159` admits added capabilities only for disk snapshot/restore Jobs. The policy covers Deployments, StatefulSets, Jobs and CronJobs, so web, worker and private services are presumably affected as well (not drilled). ADR089 (`c4f7f035c`) shipped without the matching admission change. The drill switched to the native Go runtime, which takes the `strict` policy. This needs its own milestone.

**Cron, suspend over an in-flight build, then resume:**

- 05:58:00 and 05:59:00: the subject runs successfully, with CronJob image `sha256:34463b99383b…` and command `/bin/sh -c ./cron-demo`.
- 05:59:43: `PATCH serviceDetails.envSpecificDetails.buildCommand = "echo m163-intentional-failure && exit 1"` returns 200. It opens deploy `dep-davkgjptf7js73907lk0` (trigger `config_change`), and by 05:59:53 that deploy reads `build_in_progress`.
- 05:59:53: `POST /suspend` returns 202. By 05:59:55 (1.5 s) the CronJob reads `spec.suspend=true`, with the same image and command and the deploy still `build_in_progress`.
- The build stayed in progress until 06:06:33, then read **`build_failed`** ("build failed — check the build logs …").
- 05:59:55 to 06:06:29: six minute ticks passed while suspended. The subject recorded **zero** new Jobs and **zero** new API runs (`lastScheduleTime` stayed at 05:59:00). The control recorded seven runs, one per minute.
- 06:06:29: `POST /resume` returns 202. By 06:06:31 (2.0 s) the CronJob reads `spec.suspend=false`, with the prior image. The Kubernetes CronJob controller then started one catch-up run at 06:06:29 for the missed 06:06:00 tick. Regular ticks followed at 06:07:00, with no new deploy: the deploy list was still `dep-davkgjptf7js73907lk0` (now `build_failed`) over `dep-davke2fibfms73f3us9g` (`live`).

**Schedule edit over `build_failed`:**

- 06:07:10: `PATCH serviceDetails.schedule = "*/2 * * * *"` returns 200. By 06:07:11 (1.1 s) the CronJob reads `*/2 * * * *`, with the **prior image and prior command**. The deploy list was unchanged and no release was created.
- 06:08:00 and 06:10:00: the subject runs on the new schedule.

**Cron, suspend over `build_failed`, then resume:**

- 06:10:05: `POST /suspend` returns 202. By 06:10:06 (1.4 s) the CronJob reads `spec.suspend=true` and the image is `sha256:34463b99383b…`. The latest deploy reads `build_failed`.
- 06:10:06 to 06:14:19: the 06:12 and 06:14 ticks passed with **zero** new Jobs or API runs (`lastScheduleTime` stayed at 06:10:00).
- 06:14:20: `POST /resume` returns 202. By 06:14:21 the CronJob reads `spec.suspend=false`, with the prior image. A catch-up run started at 06:14:20, then the regular 06:16:00 run. The deploy list was unchanged.
- The full subject run history was 05:58:00, 05:59:00, 06:06:29 (catch-up), 06:07:00, 06:08:00, 06:10:00, 06:14:20 (catch-up) and 06:16:00. Nothing was recorded inside either suspended window.

**Control:** the control ran successfully every minute from 05:59:00 through 06:16:00, with no gaps, through both subject suspend windows and the schedule edit.

**Static** (latest build in flight, then `build_failed`; last published revision `rev-1`):

- 05:54:40: the baseline returns 200 with body sha1 `9e3333334bec`.
- 05:54:41: `PATCH serviceDetails.buildCommand` to a failing command opens `dep-davke8fibfms73f3usb0`, which reads `build_in_progress` at 05:54:53.
- In flight: suspend at 05:54:54 serves **404 within 5.8 s**, and 404 again at 05:55:04. Resume at 05:55:05 serves **200 with `9e3333334bec` within 4.3 s**.
- 05:55:22: the deploy reads `build_failed`. Suspend at 05:55:23 serves **404 within 5.6 s**. Resume at 05:55:34 serves **200 with `9e3333334bec` within 5.4 s**, which is the last published revision.

**Cleanup:** at 06:17:01–06:17:12 every fixture returned `DELETE` 204, then `GET` 404. The two Docker attempts had already been deleted at 05:54 (204, then 404). At 06:17:32 the static URL returns 404. By 06:17:43 no `qa-20261002-` App, CronJob or Job remained in the namespace. The QA session was logged out and its state file deleted.

**DoD verdict:**

- **Cron: pass.** Observed live over both an in-flight build and a `build_failed` build. Resume and the schedule edit converged in ≤2 s without a release, and the CronJob never left the prior image or command.
- **Static: pass.**
- **Control: pass.** The healthy cron fired every minute. The web, private and worker paths were not re-drilled; this change does not touch them, and their `m156`–`m158` regressions pass in the full operator suite above.

**Observation, not a defect here:** on resume, the Kubernetes CronJob controller immediately runs the most recent tick missed during suspension, because no `startingDeadlineSeconds` is set. Render does not document whether resume backfills a missed run, so this is a parity question for a follow-up, not part of this DoD.
