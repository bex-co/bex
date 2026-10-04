# w8 · m44 — An unpullable image hides behind a 10–18 minute timeout and a "check the logs" message that has no logs behind it

**Worker:** worker8 **Goal:** when a deploy's image cannot be pulled, the deploy fails promptly with the kubelet's pull error on every path (pre-deploy and rollout, first release or over a prior release), and a failed image deploy does not leave the service configured to re-roll the broken image on its next config change. **Status:** done (2026-10-03)

## Tasks (in order)

| id   | title                                                                                                   | est | depends_on       |
| ---- | ------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Pre-deploy: detect a Waiting `ImagePullBackOff`/`ErrImagePull` on the Job pod, fail fast, and name it — **DONE**  | 45m | —                |
| t002 | Rollout over a prior release: keep the stuck-pod diagnosis (image pull) as the deploy's failure reason — **DONE**   | 45m | —                |
| t003 | Decide and implement the configured image after a failed `imageUrl` deploy (row image vs. last live) — **DONE**     | 40m | —                |
| t004 | Render parity — **DONE**                                                                                           | 20m | t001, t002, t003 |
| t005 | Simplify — **DONE**                                                                                                | 15m | t004             |
| t006 | Test coverage — **DONE**                                                                                           | 30m | t005             |
| t007 | Closeout — **DONE**                                                                                     | 15m | t006             |

## Definition of done

- On an image-backed service with a pre-deploy command, `bex deploys create <srv> --image <registry>/<repo>:<nonexistent-tag> --wait` exits non-zero within about 2 minutes (not 10), and the deploy's failure line contains `image pull is failing` plus the kubelet's reason. It does not say "the pre-deploy command did not finish" or "check the pre-deploy logs".
- The same command on a service **without** a pre-deploy command, whose previous release is live, fails with the image-pull reason, not `the deploy did not become healthy within the health-gate window; check the service logs` after 18 minutes. The previous release keeps serving throughout (already true today).
- After such a failure, an unrelated config change (for example clearing the pre-deploy command) does **not** open a deploy of the unpullable image, or, if t003 decides the row keeps the requested image, the service's reads make that explicit and the decision is recorded with Render evidence.
- Over a prior release, a rollout stuck on a failing **health check** ends with the probe diagnosis (w4/m112's `GET <path> on port <n>` line) as its failure reason, not the generic health-gate line. The same `settleFailedRollout` branch as the image-pull case; see t002 § Wider than image pull.

## Evidence (2026-09-26, `/qa-find-bugs-cli` sweep 13, production `726042a28`)

Released `bex v0.2.1` (pin v2.27.0), human device login, workspace `bex-canary`. Fixture: image-backed free web service `srv-darmspgd0qnc73d79r1g` (`docker.io/mendhak/http-https-echo:35` live, since deleted).

```text
# 1. pre-deploy command set ("echo pre-$(date +%s)")
$ bex deploys create <srv> --image docker.io/mendhak/http-https-echo:does-not-exist-999 --wait --confirm -o json
→ exit 1 after 611 s · status pre_deploy_failed (dep-darmtm1smc7s73cq5fr0, 07:06:00 → 07:16:09)
  ==> Pre-deploy failed: the pre-deploy command did not finish within its 10-minute window; check the pre-deploy logs

# 2. pre-deploy cleared → that config change ALONE opened a deploy of the unpullable image
$ bex services update <srv> --pre-deploy-command "" --confirm
→ dep-darn2nhsmc7s73cq5gig · trigger config_change · image …:does-not-exist-999 · 07:16:46 → 07:35:09 (18 min) · update_failed
  ==> Deploy failed: the deploy did not become healthy within the health-gate window; check the service logs
# throughout both: the previous release (:35) kept answering HTTP 200; service reads imagePath …:does-not-exist-999
```

The command never ran and no container ever started, so neither log the messages point to exists. The real cause (`ImagePullBackOff`) was known to the operator the whole time.

## Mechanism

- **Pre-deploy:** `lego/operator/internal/predeploy/predeploy.go:224-229` `FailureMessage` returns the deadline-exceeded line first (`predeployTimeout = 10 * time.Minute`, `:71`). `terminatedContainer` (`:244-257`) reads only `State.Terminated`, never `Waiting`, so an image-pull wait is invisible, and the Job simply runs out its 10-minute `activeDeadlineSeconds`. `failure_message_test.go:100` currently pins `ImagePullBackOff` to the generic message.
- **Rollout over a prior release:** `lego/operator/internal/controller/app_controller.go:2845-2852` `settleFailedRollout` calls `settleFailureOverPriorRelease(…, "the latest rollout failed")` when `releaseHasServed(app)`, **before** consulting `stuckPodMessage`. `stuckPodMessage` (`:4238`, `:4266-4267`) would return `"image pull is failing: " + kubelet message`. The backend then closes the row with the generic `timedOutDeployReason` (`lego/backend/internal/store/reconciler.go:1446-1455`) after the rollout budget (`rolloutBudgetSeconds = 900`) plus the deploy-gate margin. The first-release path (`releaseHasServed == false`) already keeps the diagnosis.
- **Row image after a failed imageUrl deploy:** `a3fae64cd` (w8/022) now writes the requested image to the store row before the CR patch. That is correct for success, but nothing restores the last live image when the deploy fails, so the projector keeps `spec.image` on the broken tag and the next config change or restart re-rolls it (observed in step 2).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs-cli` sweep 13 (w8), 2026-09-26, while live-verifying the w8/022 fix.
- **Goal linkage:** honest deploy failure reasons (the w7/m79 and w4/m103 line of work that removed the vague health-gate message for other failure classes; image pull was left out on these two paths) and Render-compatible deploys (ADR006).
- **Expected outcome:** a mistyped or missing image tag is diagnosed in about a minute with the registry's own error, and one bad tag does not poison later config changes.
- **Why now:** w8/022 just made `deploys create --image` and deploy-hook `?imgURL=` work, so CI pipelines will start sending tags that don't exist yet (race with a registry push) and will hit this directly.
- **Render parity included:** it changes a user-facing deploy-failure surface (REST/GraphQL/MCP deploy rows and dashboard deploy log).

## Unverified

- Render's exact pull-failure timing and wording, and whether Render's service image setting follows a failed `imageUrl` deploy (t003/t004 must establish this before choosing).
- The first-release path's timing with a pre-deploy command (not exercised).
- The deploy-hook `?imgURL=` path (same backend path; not exercised live).

## Blocked (2026-09-26)

t001, t002, t004–t006 are done: the pre-deploy gate fails an unpullable image in about 95s with `image pull is failing: …`, and a rollout that fails over a prior release closes when the operator settles, with its diagnosis (image pull, crash loop, health check, quota) via `ConditionRollout`. Open:

1. **t003: user decision + Render evidence.** What the service's configured image is after a failed `imageUrl` deploy. Render's docs don't say, and restoring the row alone is unsafe (see t003). Pick (a) keep, (b) auto-rollback release, or (c) don't write the row for `imageUrl`.
2. **t007: live closeout** after deploy (`blocked/m42`) with a logged-in `bex` CLI.

## Live re-verification (2026-09-27, `/qa-find-bugs-cli` sweep 52)

Over a prior release (no pre-deploy command), production `4a0422577`, `bex v0.2.1`, human device login, workspace `bex-canary`, free `mendhak/http-https-echo` fixtures (deleted). `bex deploys create <srv> --image …:qa52-nonexistent-tag --wait`: the open row carried `stallReason: image pull is failing: Back-off pulling image … ErrImagePull … NotFound` from the first minutes, and closed `update_failed` with that same line as `failureReason` (**reason ✔**, no generic health-gate text). The prior release kept serving (200). **Timing is still not prompt:** exit 1 after **922 s** (~15 min, down from ~18). The rollout path waits for the Kubernetes progress deadline, while the pre-deploy path has a 90 s pull-failure grace. The goal's 'fails promptly … on every path' needs the same `PullFailure`-style grace on the rollout branch (a stuck-pod image-pull verdict after ~90 s closes the row without waiting for `ProgressDeadlineExceeded`). The service's `imagePath` stayed on the failed tag (t003, parked).

> Overlap (2026-09-27): `w4/162` (a parallel QA pass 239) independently files the same 15-minute wait for a permanent `NotFound` pull on a service with a live release. Land one fix and close the other against it.

## Live re-verification 2 (2026-09-28 ~09:20Z, production `6b6d99ea8`)

**Timing now holds.** On the over-prior-release path, which includes `87c93899d` "bound permanent rollout image pull failures", `bex deploys create <srv> --image …:qa74-nonexistent-tag --wait` exits 1 after **100 s** (was 922 s), with `failureReason: image pull is failing: … ErrImagePull: rpc error: code = NotFound`. The prior release kept serving (200). Only t003 (the configured image after a failed `imageUrl` deploy) remains for this milestone. `w4/162` is satisfied by the same change.

## Unblocking work (2026-10-02)

The user accepted per-deploy image overrides with saved settings unchanged. [Render's image webhook documentation](https://render.com/docs/deploying-an-image#deploy-via-webhook) explicitly states subsequent deploys use the tag/digest in service settings. The REST reference documents `imageUrl` and its repository restriction but does not explicitly state persistence; applying the webhook policy across REST/GraphQL/MCP is a recorded consistency decision, not an authenticated Render experiment.

The local implementation uses the existing release selection from w5/m107: `imageUrl`/`imgURL` sets `ReleaseConfig` for one release; the saved row and `spec.image` stay unchanged, and the deploy row records that call's image. Projection preserves the selection. Ordinary/configuration deploys return to the saved image; restart retains the actual live release. The prior failure-diagnosis checks remain valid.

**Remaining:** ship, then verify a failed override followed by a configuration change on a disposable live service. The old row-restore alternatives and missing-evidence blocker are superseded by this decision.


## Verification and remaining release gate (2026-10-02)

Full backend suite against isolated Postgres/OpenFGA/OpenBao, operator `make test`, full CLI suite, backend/CLI lint, targeted Go race tests, workflow guards and ci-red-streak fixtures passed. The final affected backend packages passed again after review changes. Overlay mutation checks confirmed the image, workspace and rename regressions fail with their fixes removed. Markdown was formatted; QA fixtures and the isolated credentials were cleaned up.

Implementation and review are complete locally. Repository `AGENTS.md` requires an explicit `$ship` before commit/push. After ship, observe CI/production and complete this milestone's remaining live closeout; m45 also needs the updated CLI released. The earlier policy/sign-off/login blockers are resolved.

## Live closeout 2026-10-02/03 — DONE

Production deploy pin `1263d12ae3fa`, which contains `faaaba0ce` (per-deploy `imageUrl` via `ReleaseConfig`, saved image unchanged) and `9e70249c9`/`87c93899d`. The client was the released `bex v0.2.1` (pin v2.27.0) with human device login in an isolated config, workspace `bex-canary`. The fixture was free image web service `qa-20261002-aed4ca-img2` (`srv-db09v90ehcmc739j12gg`, `docker.io/mendhak/http-https-echo:35`, `HTTP_PORT=3000`). It was deleted afterwards and `GET` returns 404. A first fixture `srv-db09segehcmc739j125g` lacked `HTTP_PORT`, never passed its TCP probe, and was deleted before any m44 step.

```text
# DoD 2 — no pre-deploy command, prior release live
$ bex deploys create <srv> --image docker.io/mendhak/http-https-echo:qa-…-nonexistent --wait --confirm -o json
→ exit 1 after 100 s · dep-db09vjatm2ss7389qna0 update_failed
  failureReason: image pull is failing: …:qa-…-nonexistent: Back-off pulling image … ErrImagePull: rpc error: code = NotFound …
  prior release answered HTTP 200 on every 15 s sample (6/6); service imagePath stayed …:35

# DoD 3 — t003 decision: the failed override does not poison the next config change
$ bex services update <srv> --pre-deploy-command "echo qa-predeploy" --confirm
→ dep-db0a0fqtm2ss7389qncg · trigger config_change · image …:35 (the SAVED image) · live in 30 s · HTTP 200

# DoD 1 — pre-deploy command set
$ bex deploys create <srv> --image …:qa-…-nopre --wait --confirm -o json
→ exit 1 after 110 s · dep-db0a0tqtm2ss7389qndg pre_deploy_failed
  failureReason: image pull is failing: Back-off pulling image "…:qa-…-nopre": ErrImagePull … not found; the pre-deploy command never ran
  HTTP 200 after; imagePath still …:35

# DoD 4 — health check failing over a prior release (memcached listens on 11211, never on 3000)
$ bex deploys create <srv> --image docker.io/library/memcached:1.6-alpine --confirm   (traffic every 60 s)
→ dep-db0acnqtm2ss7389qoc0 · rollout phase from 06:53:36Z · open-row stallReason = TCP-probe line
→ closed update_failed at 07:08:31Z (~15 min, the operator's settle, not the 18-min backend gate)
  failureReason: the container is running but its startup health check has not succeeded, so the rollout is waiting: a TCP connect to port 3000. …
```

Every DoD line holds. The m44 implementation needs no follow-up.

**Adjacent observations from this run (outside m44's DoD, not filed here; recommend a `/pm` filing as a sibling of `w1/m156`):**

1. **A failed rollout on a hibernated free service leaves it down.** The service auto-hibernated at 06:45:36Z during an earlier rollout, and the DoD-4 deploy started while it was hibernated. After the row closed (`deploy_ended` + `service_woken` at 07:08:31Z), every request through 07:15Z, 20+ samples over 6+ minutes, got the activator's `503 {"error":"service hibernated","retryAfter":5}`. Meanwhile REST read `phase: Running`, `replicas: 1`, and saved `imagePath …:35`. Presumed mechanism: the Deployment template is the failed release and the prior ReplicaSet was at zero, so waking scales only broken pods. w1/m156 fixed the `pre_deploy_failed` variant of the same "never wakes" symptom; this is the `update_failed` rollout variant. Unverified: the Deployment template and RS state were not read, because there is no cluster access.
2. **Hibernation during a rollout masks the diagnosis.** The first DoD-4 attempt (`dep-db0a302tm2ss7389qnmg`, no traffic) was preempted by `service_hibernated` at 06:45:36Z, before the 15-minute progress deadline. It closed at the 18-minute backend gate with the generic `the deploy did not become healthy within the health-gate window; check the service logs`, even though its open row had carried the TCP-probe stall reason.
