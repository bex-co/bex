# CLI sweep 551 — valid Git-to-image source switch, after an invalid-registry detour

**Verdict: no new bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI, production still on the `f4e38371055f` image pin, 2026-10-19T11:26–11:48Z. One owned service completed the full Git → failed image → corrected image journey and was deleted by exact ID, with public URL `404`, deploy read `404`, and service-list absence verified.

## Journey

1. Created `qa-20261019-5b6da0-src551` from `bex-co/bex@main`, Docker runtime, root `examples/hello-go`; first deploy reached `live` and returned `OK`.
2. `services update --image docker.io/google-samples/hello-app:2.0` saved the new image source without deploying it. Saved read showed repo/branch cleared, `undeployedChanges: true`, old Git release still `Running` and serving `OK`.
3. An explicit deploy then spent **15m07s** in `update_in_progress` and settled `update_failed`, because `google-samples/hello-app` is not a Docker Hub repository in this form: `pull access denied, repository does not exist or may require authorization: server message: insufficient_scope`. The old Git pod served throughout and after the failure. This was a harness reference error, and on the pre-w8/131 production pin it independently reproduced w8/131's exact Docker Hub wording trap; no duplicate finding is warranted.
4. Corrected the save to `gcr.io/google-samples/hello-app:2.0` (the reference used successfully in sweep 537) and explicitly deployed again. It reached `live` in about half a minute, body flipped to `Hello, world! Version: 2.0.0`, revision advanced, and `undeployedChanges` cleared.

The final read retained `rootDir: examples/hello-go` and Docker context as inactive saved build settings after switching to an image. That residue matched sweep 549's retained branch pattern and did not affect the image rollout; it is metadata available for a later switch back, not misreported active source (`repo`/`branch` were absent and `imagePath` named the live GCR image).

## Cleanup

Deleted exact-owned `srv-dbavssoqabuc73f7t0g0`; former URL and deploy reads returned `404`, and the workspace list no longer contained it.

## Unrun / queued

- W8/131's fix is on `main` but remains behind the production pin; once deployed, this sweep's accidental Docker Hub leg is a ready-made acceptance vector and should settle around the pull grace rather than 15 minutes.
- A valid image switch while another deploy is already in flight remains governed by the in-flight release work (m119–m121) and was not repeated.
- Switching back again from image to the previously saved Git root was covered directionally by sweep 550, but not as a same-service round trip after a failed intermediate deploy.
