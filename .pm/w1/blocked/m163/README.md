# w1 · m163 — A cron job or static site whose newest build failed or is still building ignores suspend, resume and schedule edits

**Worker:** worker1 **Goal:** suspend, resume and schedule edits act immediately on cron jobs and static sites even while the newest release has no image, the way they already do for web, private and worker services since `w1/m156`–`m158`. **Status:** blocked — t001, t002, t003, t005, t006, t007 done; t004 requires a healthy local CAPD control plane for the live drill, then t008 closeout

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Render evidence: suspend on a cron job or static site acts immediately regardless of the latest build — **DONE** | 20m | —          |
| t002 | Extend the hold to cron jobs: converge `suspend` and `schedule` on the CronJob serving the prior image — **DONE** | 50m | t001       |
| t003 | Static sites: suspend and resume over a failed or in-flight build converge the published revision's routing — **DONE** | 40m | t001       |
| t004 | Live: a cron job with a failed newest build stops firing within one schedule tick of suspend              | 45m | t002, t003 |
| t005 | Render parity — **DONE** | 20m | t004       |
| t006 | Simplify — **DONE** | 15m | t005       |
| t007 | Test coverage — **DONE** | 40m | t005       |
| t008 | Closeout                                                                                                  | 10m | t007       |

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

**Gate:** the operator must restore a healthy local CAPD API/scheduler (without disrupting the other dev stacks), or explicitly authorize a production drill. Then run t004 against the pre-fix and fixed operators with a healthy control cron, verify static HTTP suspend/resume, and complete t008. Automated coverage is green and shippable; the milestone remains blocked rather than done.

**Mutation proof:** on an isolated `f47178f1d` checkout, all three cron matrix cases (failed, running and queued builds) fail because suspension changes the prior pod template; the same cases pass with this fix.
