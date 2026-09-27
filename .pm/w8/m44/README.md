# w8 · m44 — An unpullable image hides behind a 10–18 minute timeout and a "check the logs" message that has no logs behind it

**Worker:** worker8 **Goal:** when a deploy's image cannot be pulled, the deploy fails promptly with the kubelet's pull error on every path (pre-deploy and rollout, first release or over a prior release), and a failed image deploy does not leave the service configured to re-roll the broken image on its next config change. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                   | est | depends_on       |
| ---- | ------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Pre-deploy: detect a Waiting `ImagePullBackOff`/`ErrImagePull` on the Job pod, fail fast, and name it  | 45m | —                |
| t002 | Rollout over a prior release: keep the stuck-pod diagnosis (image pull) as the deploy's failure reason   | 45m | —                |
| t003 | Decide and implement the configured image after a failed `imageUrl` deploy (row image vs. last live)     | 40m | —                |
| t004 | Render parity                                                                                           | 20m | t001, t002, t003 |
| t005 | Simplify                                                                                                | 15m | t004             |
| t006 | Test coverage                                                                                           | 30m | t005             |
| t007 | Closeout                                                                                                | 15m | t006             |

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
