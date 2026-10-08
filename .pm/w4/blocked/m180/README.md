# w4 · m180 — Save and deploy reuses the serving artifact

**Worker:** worker4 **Goal:** an environment save requesting `deploy` applies the new configuration once using the serving artifact, without a source build. **Status:** blocked

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Select the serving artifact for a deploying environment batch — **DONE** | 60m | — |
| t002 | Audit shared rollout callers, source kinds and missing-artifact policy — **DONE** | 45m | t001 |
| t003 | Render parity — **DONE** | 25m | t002 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Test coverage — **DONE** | 40m | t003, t004 |
| t006 | Closeout | 10m | t005 |

## Definition of done

Repeat these **observed production journeys** on a fresh owned Free Docker web service from `https://github.com/bex-co/bex`, branch main, root `examples/hello-go`, port 3000 and auto-deploy off. Use synthetic MESSAGE values and remove all owned resources afterwards.

- After the first deploy is Live, edit MESSAGE and choose **Save and deploy**. The wire request still uses `saveMode: "deploy"`; the response reports `rolledOut: true`; exactly one new `config_change` release reaches Live with the new value at its `.onbex.co` URL. Its selected image is the previously serving immutable artifact, its commit remains that artifact's commit, and no BuildKit source build runs. Reload and repeat. Also remove a service-owned MESSAGE over a linked group: the new group's MESSAGE reaches the process without rebuilding the source.
- Choose **Save only** with another MESSAGE. API readback contains the saved value, `rolledOut` is false, the public URL keeps the prior served value, and no new deploy appears. Cancel an unsaved edit and confirm it changes neither saved nor running configuration.
- Choose **Save, rebuild, and deploy**. One new source build/deploy runs, with actual build output, reaches Live, and serves the saved MESSAGE. No intermediate environment-only rollout is opened.
- Choose **Restart service** and confirm it. One release selects the previously serving image/configuration and reaches Live; this existing no-build control remains intact. Image-bearing deploy history stays honest about build lifecycle (coordinate with `w4/blocked/m156`, already fixed on main; do not duplicate its timestamp work).
- The regression audit in [finding.md](finding.md) carries **every clause** of w5/m44's original DoD. Complete its unchecked compatibility work in t002/t005 before closeout; inferred sibling behavior is not claimed as a live failure from this hunt.

## Source + Goal linkage

- **Source:** user-requested infinite `qa-find-bugs` loop using `muse.env`, w4, 2026-10-07 cycle 1. [finding.md](finding.md) contains exact request/complete response, repeat logs, source trace, controls and retained evidence paths.
- **Goal linkage:** ADR008 pillar 1 / Render-compatible hosting, ADR006 shared API contract, ADR013 saved/served secrets, ADR004 immutable serving artifact and generation-scoped release selection, ADR018 parity. Restore the explicit w5/m44 three-choice contract.
- **Expected outcome:** environment-only saves avoid an unnecessary source build and its failure risk while still deploying newly saved values once. The distinct rebuild choice retains its advertised effect.
- **Why now:** two fresh-page `deploy` requests ran real `go build` jobs (~32 s compilation each), while Restart already demonstrated current-artifact reuse. Every environment save currently pays for a build it did not request.
- **Scope:** filing only. Product fixes are pending. t002 owns the blast radius and unprobed families; no broad rewrite of artifact fingerprints or accepted env-group auto-deploy policy.
- **Standing closing tasks:** parity, simplify, coverage and closeout are included because this touches UI plus the REST/GraphQL/MCP environment-save semantics.

## Progress (2026-10-08)

t001–t005 done. `PatchEnvironment` with `saveMode: deploy` on a source-built, non-static App writes an image-only `spec.releaseConfig` inside the locked `rollApp` patch: `{generation: metadata.generation+1, image: <live deploy's resolvedImage>, sourceGeneration: 0}`, alongside the `restartedAt` bump. The operator's `reusableArtifactImage` returns the selected image before `buildFromSource` (`app_controller.go:796`), and `sourceGeneration: 0` snapshots the **newly saved** values. `TestImageOverrideUsesSavedConfigurationAndExpiresOnNextRelease` already pins that in the operator, so no operator change was needed. `rollout.Tracker.open` records the selected image (so `buildLifecycleFacts` emits no build history) and the live artifact's commit, not the newest attempted commit. `Tracker.ServedRelease` reads the live row through an optional `ListDeploys` capability; Restart's own read is left as is.

Policy: a source-built service with no live build is refused with 409 `ENVIRONMENT_DEPLOY_NEEDS_LIVE_RELEASE` before any write; save only still works. The dashboard toasts the server message, and its option copy already reads "roll the current image once". Prebuilt-image services already run `spec.image` without a build. Static sites keep rebuilding on deploy, because their environment is build-time only. CR-only mode (no deploy store) keeps the old behavior. REST `PATCH …/environment`, GraphQL `patchServiceEnvironment` and MCP `patch_service_environment` share `PatchEnvironment`, so they move together. ADR013 records the contract.

Tests: `TestDeployingEnvironmentSaveReusesTheServingArtifact` checks the selection shape and generation, the release annotation, one row with the served image and commit, and the saved value. `TestDeployingEnvironmentSaveWithoutALiveBuildIsRefused` checks the 409, that nothing was written, and that save only still works. `TestDeployingStaticSiteEnvironmentStillRebuilds` covers static sites. The backend secrets, rollout, envgroups, apps and deploys suites and `make lint` pass.

Not verified until the live replay:

- Native/buildpack/private/worker/cron variants: they share the path, but only Docker web was observed.
- `undeployedChanges` after a selected-artifact release.
- Linked-group value removal reaching the process.
- File content.

Not covered:

- A second rolling write inside one `rollout.Batch` raises the generation again and expires the selection, so that release would build. Low risk: the environment batch is one patch.
- The w5/m44 clause table's unprobed UI items (dotenv import, uploads, generated preview, export, mobile/keyboard/zh) are not touched by this backend fix and are left to the closeout replay.
