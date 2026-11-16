# 673 — An environment "Save and deploy" (and an env-group link or edit) on an image-backed service redeploys the saved tag, not the image that is running: after an `imageUrl` / `imgURL` deploy, saving one variable puts the service back on the older image, while Restart keeps the running one

Why: Render's "Save and deploy" "redeploys your service's _existing_ build with the new environment variables" ([render.com/docs/configure-environment-variables](https://render.com/docs/configure-environment-variables)). bex keeps that promise for a Git-backed service (w4/m180: "Save and deploy reuses the serving artifact"). For an image-backed service it reruns the saved image instead.

Since w8/042 and w4/414, a per-deploy `imageUrl` / hook `imgURL` override leaves the saved image alone. So the image a "Save and deploy" reruns can be an older one than the image serving. The user saves one variable and silently ships the previous release.

- **Source / severity / estimate:** live `/qa-find-bugs w4` loop round 399, 2026-11-15 07:23–07:31 UTC (`muse.env`, workspace `bex-canary`, pin `89ac87445bec`). **Major**: an environment save silently rolls the running release back. ~1h (~1h30m with round 400's env-group paths).
- **Fixture:** Free web service `qa-20261115-r399-img` (`srv-dbslrt5dergc73ccgjgg`), image `docker.io/library/busybox:1.36`, `dockerCommand: sh -c 'mkdir -p /w; busybox | head -1 > /w/index.html; exec httpd -f -p 10000 -h /w'`, so the page names the running version. It was deleted afterwards (`GET` 404).
- **Probe (every step through `api.bex.co`, URL read with `curl`):**
  1. D0 went `live` at 07:24:58, and the URL served `BusyBox v1.36.1`.
  2. `POST /v1/services/{id}/deploys {"imageUrl":"docker.io/library/busybox:1.37"}`: the deploy `dep-dbslsitdergc73ccgjkg` went `live` at 07:26:08 (`image.ref …busybox:1.37`, `sha256:a7dbc319b143…`). The URL served `BusyBox v1.37.0`.
     - 30 s later `GET /v1/services/{id}` still read `imagePath: docker.io/library/busybox:1.36`, the saved image (by design, w4/414).
  3. At 07:27:15.9, 68 s after that deploy went live (outside w4/672's window), `PATCH /v1/services/{id}/environment {"envVars":[{"key":"QA_R399","value":"x"}],"saveMode":"deploy"}` answered `200 {"envVarKeys":["QA_R399"],"rolledOut":true,…}`.
     - Its deploy `dep-dbsltktdergc73ccgjl0` (trigger `service_updated`) went `live` with **`image.ref docker.io/library/busybox:1.36`, `sha256:73aaf090f3d8…`**.
     - From 07:28:19 the URL served **`BusyBox v1.36.1`**.
  4. **Control, Restart:** a fresh `imageUrl` 1.37 deploy (`dep-dbslu86oou4c73fdrilg`) went `live` at 07:30:09. At 07:30:40, `POST /v1/services/{id}/restart` (no body) opened `dep-dbslv86oou4c73fdritg`, which went `live` on `docker.io/library/busybox@sha256:a7dbc319…`. The URL kept serving `BusyBox v1.37.0`.
  5. Same flow inside w4/672's window (round 399 fixture `qa-20261115-r399-env`, the env save 0.4 s after the `imageUrl` deploy went live): the env deploy went `live` on 1.36 as well, after 84 s of `no available server` (w4/672).
- **Root cause:**
  - `servedArtifact` (`lego/backend/internal/secrets/batch.go:195-211`) returns `""` whenever `a.Spec.Image != ""`. That is every prebuilt-image service: the comment reads "Empty means nothing to select — a prebuilt image".
  - `selectServedBuild` (`:463-476`) then writes no `Spec.ReleaseConfig`, and the release runs `Spec.Image`, the saved tag.
  - The premise dates from w4/m180, whose README says "Prebuilt-image services already run `spec.image` without a build". That stopped being true when the trigger override began selecting its runtime image per release without touching the saved one (`lego/backend/internal/deploys/service.go:962-966`: "A deploy override selects runtime input without changing the saved image that the projector and subsequent config changes use").
  - Restart does not have the bug, because `restartSelection` (`deploys/rollback_config.go:183`) re-selects the running release, image included (`assertRestartedGeneration` asserts `selected.Image == servingImage`, `restart_test.go:290-296`).
- **Fix (target behavior):**
  - An environment "Save and deploy" on an image-backed service reruns the image the service is running (the live release's resolved image, the digest Restart selects) with the new values.
  - `servedArtifact` therefore returns `live.ResolvedImage` from `Rollout.ServedRelease` for an image-backed service as it does for a Git-backed one. It keeps returning `""` (deploy `Spec.Image`, as today) when there is no live release, so an image service never gets m180's 409 `ENVIRONMENT_DEPLOY_NEEDS_LIVE_RELEASE`.
  - "Save, rebuild, and deploy" on an image service keeps pulling the saved image (there is nothing to build). Say so in the code comment.
  - REST `PATCH …/environment`, GraphQL `patchServiceEnvironment` and the MCP environment tools share `PatchEnvironment`, so one change covers all three. The dashboard needs no change.
- **Blast radius:** `servedArtifact` has 3 callers, all in `secrets/batch.go`:
  - `:141`, the precheck;
  - `:466`, `selectServedBuild`;
  - `:506`, `deferServedBuildDeploy`. With a non-empty image for image services, an image service's "Save and deploy" behind a deploy in flight would also queue under Render's Wait (w4/667's rule, today Git-only). Decide that explicitly and test both policies (Wait, Override w4/653).
  - Regression tests on the Git-backed path (w4/m180, w4/667) must stay green.
- **Round 400: the env-group paths do the same; a Settings edit does not deploy** (2026-11-15 07:43–07:54 UTC, pin `89ac87445bec`).
  - Fixture: Free `qa-20261115-r400-sib` (`srv-dbsm53tdergc73ccgkbg`, same image and command) and env group `qa-20261115-r400-eg` (`evg-dbsm6i5dergc73ccgkg0`, `QA_G=1`). Both were deleted (`GET` 404).
  - Each step started from an `imageUrl` 1.37 deploy that had been `live` for 30 s and served `BusyBox v1.37.0`.
  1. **Linking the group:** `POST /v1/env-groups/{id}/services/{serviceId}` answered `200`. Deploy `dep-dbsm6iddergc73ccgkgg` (`service_updated`) went `live` at 07:47:21 on **`busybox:1.36` (`73aaf090…`)**, and the URL served `BusyBox v1.36.1`.
  2. **Editing the group's variable:** `PUT /v1/env-groups/{id}/env-vars/QA_G {"value":"2"}` answered `200`. Deploy `dep-dbsm7uuoou4c73fdrjkg` (`service_updated`) went `live` at 07:51:19 on **`busybox:1.36`**, and the URL served `BusyBox v1.36.1`.
  3. **A Settings edit:** `PATCH /v1/services/{id}` with a new `dockerCommand` answered `200`. It saved the command and opened no deploy within 30 s, so the URL kept serving `1.37`.
     - GraphQL then read `undeployedChanges: true`, `undeployedChangesReason: "saved"`, `servingImage: …:1.37`, `imagePath: …:1.36`.
     - The next standard deploy would apply the command on the saved image, as w4/414's notice says.
  - **Mechanism of 1 and 2 (code read):** the group's writes go through `rollLinked` (`lego/backend/internal/envgroups/service.go:1185-1203`) and `rollOne` (`:1318-1332`). Both bump `spec.restartedAt` with no `Spec.ReleaseConfig`. The operator then reads the release as `savedImageRelease` (`lego/operator/internal/controller/release_identity.go:655-659`: "a release of the saved prebuilt image … no selected … release configuration") and runs `Spec.Image`.
  - `grep -rn 'Spec.ReleaseConfig = ' lego/backend/internal` finds only deploys (`service.go:965`), rollback/restart (`rollback_config.go:131`) and secrets (`batch.go:473`).
- **Fix, widened:** every configuration-only release of an image-backed service runs the image the service is running, as Restart's `restartSelection` does and as a Git-backed service keeps its built artifact. That covers the service "Save and deploy", and the env-group link, unlink, variable and secret-file writes that roll linked services. One way: a shared helper in `rollout` that selects the served release's resolved image into `Spec.ReleaseConfig` for an image service, called from `selectServedBuild` and from `rollLinked` / `rollOne`.
  - If render.com's `imgURL` deploy turns out to rewrite the saved image instead (w4/414's open question), saving the tag on the override fixes every path at once. The per-path selection above is parity-sufficient either way, because Render's "Save and deploy" names the existing build.
  - Estimate ~1h30m with the env-group paths.
- **Round 407: a Blueprint sync** (2026-11-15 09:28–09:34 UTC, pin `89ac87445bec`).
  - Fixture: an inline `POST /v1/blueprints/deploy {ownerId, bexYaml}` of one Free image web service, `qa-20261115-r407-bp` (`srv-dbsnmdddergc73ccgqcg`, `image.url: docker.io/library/busybox:1.36`, the version-page `dockerCommand`). It was deleted afterwards (`GET` 404), and no Blueprint object remained.
  - Each step started from an `imageUrl` 1.37 deploy that had been `live` for 30 s.
  1. **The unchanged manifest leaves the override alone, as Render does.** `POST /v1/blueprints/validate` planned one `noop`. Re-applying the manifest answered `200` and opened no deploy within 30 s. The URL kept serving `1.37`, and GraphQL read `undeployedChangesReason: "image_override"`.
  2. **A changed manifest rolls the override back, and its plan does not say so.** I added `envVars: [{key: QA_BP, value: plan-vs-apply}]`.
     - The plan read one `update` with `changedFields: [{"path":"envVars"}]`.
     - The apply opened `dep-dbsno2eoou4c73fdrn7g` (trigger `blueprint_sync`) on **`busybox:1.36`**. It went `live` at 09:33:45, and the URL served `BusyBox v1.36.1`. GraphQL `servingImage` turned empty.
     - Deploying the declared image is defensible, since the manifest is the source of truth. **The defect is the plan:** it diffs the manifest against the saved spec (`live := r.services[resource.Name].Spec`, `lego/backend/internal/apps/blueprint_state_plan.go:247-255`), and the per-deploy override lives in `Spec.ReleaseConfig`, outside that diff.
     - **Target:** when a service runs an override (`servingImage` ≠ the declared image), the plan names `image` among the changed fields, with a message that the sync replaces the running `…:1.37` with the declared `…:1.36`. The dashboard's Blueprint review shows that line.
- **Round 418 control: Suspend and Resume keep the running image** (2026-11-15 12:28–12:32 UTC, pin `89ac87445bec`).
  - Fixture: Free `qa-20261115-r418-res` (`srv-dbsqb0nn63pc7382qv40`, deleted afterwards, `GET` 404), on an `imageUrl` 1.37 deploy that had been `live` for 30 s.
  - `POST …/suspend` answered `202`, and the URL answered `503`. `POST …/resume` answered `202`.
  - About 25 s later the URL served `BusyBox v1.37.0`. No new deploy opened: the `imageUrl` row stayed `live`. GraphQL still read `servingImage …:1.37`, `undeployedChangesReason: "image_override"`.
  - Resume, like Restart, is not one of the paths this note fixes.
- **Unverified:**
  - an env-group delete (it also rolls linked services);
  - a mutable saved tag (`:latest`) that moved since the last deploy. Today a "Save and deploy" would pull the new image under it; after the fix it reruns the running digest, which is Render's "existing build".
- **Render:** "Save and deploy: Render redeploys your service's _existing_ build with the new environment variables." Whether Render's `imgURL` deploy also rewrites the saved image URL is not documented (its deploy-hooks page only says an absent `imgURL` uses "whichever tag or digest you've specified in the service's settings"). w4/414's Unverified line already flags it.
- **Dedupe:**
  - w4/m180 (done) pinned the Git-backed case; its DoD never ran an image service.
  - w4/414 (done) relabelled the override state ("a standard deploy returns to it") but did not decide what an environment "Save and deploy" does.
  - w4/667 (done) is the Git-backed deferral.
  - `grep -rli 'servedArtifact' .pm` finds 667 and w4/m234 (a suspended service's save, unrelated). Not a duplicate.
