# w4 · m418 — A configuration-only release of an image-backed service reruns the image it is running, as on Render

**Worker:** worker4 **Goal:** an environment "Save and deploy", an env-group link/unlink/variable/secret-file write, and a Blueprint plan no longer silently roll an image-backed service back from a per-deploy `imageUrl` / `imgURL` override to the saved tag. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                   | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Shared `rollout` helper selecting the served image for an image-backed service's config-only release    | 30m | —          |
| t002 | Environment "Save and deploy" on an image service reruns the served image, and waits under Render's Wait | 40m | t001       |
| t003 | Env-group link, unlink, variable and secret-file rolls keep the served image                            | 30m | t001       |
| t004 | Blueprint plan names `image` when a sync replaces a running image override                              | 40m | —          |
| t005 | Render parity                                                                                           | 15m | t002, t003, t004 |
| t006 | Simplify                                                                                                | 15m | t005       |
| t007 | Test coverage                                                                                           | 30m | t005       |
| t008 | Closeout                                                                                                | 10m | t007       |

## Definition of done

- On an image-backed web service whose live release runs an `imageUrl` override (saved `…:1.36`, serving `…:1.37`):
  - `PATCH /v1/services/{id}/environment {"saveMode":"deploy"}` (and GraphQL `patchServiceEnvironment`, MCP) opens a `service_updated` deploy that goes live on the 1.37 digest; the URL keeps serving 1.37.
  - Linking an env group, editing a group variable, and unlinking it each open a deploy that goes live on the 1.37 digest.
  - Under the workspace's Wait policy a "Save and deploy" behind an in-flight deploy waits for it and then reruns what it leaves serving; under Override (w4/653) it deploys at once on the image serving at save time.
  - `POST /v1/blueprints/validate` for a changed manifest lists `image` among the update's `changedFields`, with a message naming the running and the declared image; the dashboard Blueprint review shows that message.
- An image service with no live release still saves and deploys the saved image (never m180's 409 `ENVIRONMENT_DEPLOY_NEEDS_LIVE_RELEASE`). "Save, rebuild, and deploy" still pulls the saved image.
- A service serving its saved tag does not start reading `undeployedChangesReason: image_override` after an environment save or a group roll.
- The Git-backed regressions (w4/m180, w4/667) and Restart's (`restart_test.go`) stay green; `cd lego/backend && go test ./...`, `make lint`, and `dashboard` `yarn lint` + `yarn test` pass.

## Source + Goal linkage

- **Source:** promoted from inbox note w4/673 ([source-673.md](source-673.md)), live `/qa-find-bugs w4` loop rounds 399, 400, 407 and 418, 2026-11-15 UTC (pin `89ac87445bec`; major).
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)): Render's "Save and deploy" "redeploys your service's _existing_ build with the new environment variables".
- **Expected outcome:** saving one variable or touching a linked group never silently ships the previous release of an image service.
- **Why now:** w8/042 and w4/414 made per-deploy overrides leave the saved image alone, so every configuration-only release now rolls such a service back. Restart and Resume already keep the running image, so only these paths diverge.
- **Render parity task included:** the fix changes REST, GraphQL, MCP environment saves, env-group writes, and the Blueprint plan the dashboard shows.

## Triage notes (2026-11-15, loopx worker)

Code read at `8378bb0c5`; the note's premise holds:

- `servedArtifact` (`lego/backend/internal/secrets/batch.go:195-211`) returns `""` for every `a.Spec.Image != ""` service, so `selectServedBuild` (`:463-476`) writes no `Spec.ReleaseConfig` and the release runs `Spec.Image`.
- `rollLinkedService` (`lego/backend/internal/envgroups/service.go:1140`) is the single writer behind `rollLinked` and `rollOne`; it bumps `spec.restartedAt` with no selection, and the operator reads that as `savedImageRelease` (`lego/operator/internal/controller/release_identity.go:657`).
- **Trap: do not select a digest blindly.** A `ReleaseConfig{Image: digest, SourceGeneration: 0}` keeps the tag as its `ImageSource` only when the operator reuses the cached image network (`reusableImageNetwork`, `image_network.go:211`). When a save changes the port configuration the network resolves afresh with `Source == digest`. Then `servesSavedImage` (`saved_configuration.go:277`) says the service is not serving its saved image, and an un-overridden service reads `image_override`. Two safe options:
  - select only when the serving image is not the saved one: `live.ResolvedImage != a.Spec.Image`, and no `Status.ServingNetwork`/`ImageNetwork` with `Image == live.ResolvedImage && Source == a.Spec.Image`;
  - or teach `releaseImageSource` to carry the served network's `Source` for a SourceGeneration-0 digest selection.
- `rollout.Tracker.open` (`rollout/rollout.go:318-324`) already records the selected image on the deploy row when `ReleaseConfig.Generation` matches.
- Deferral (w4/667): `deferServedBuildDeploy` decides on "has a live artifact", and `DeployServedBuild` selects again when the deploy runs. Under Wait it should defer for every image service with a live release, including one serving its saved tag, because the deploy in flight may be an override. So keep "has a live release" (deferral and precheck) separate from "needs a selection" (t002).
