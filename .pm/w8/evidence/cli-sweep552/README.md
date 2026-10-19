# CLI sweep 552 — same-service image → Git → image round trip

**Verdict: no bug filed.** Closes the same-service round-trip leg left unrun by sweeps 550/551. Same isolated QA session, `bex-canary`, HEAD-built CLI at `d98d07f62`, production pin still `f4e38371055f`, 2026-10-19T11:50–11:54Z. One owned free web service was created, switched in both directions, deleted by exact ID, and verified absent (public URL `404`, deploy read `404`, list omission).

## Journey

1. **Image leg:** created from `docker.io/ealen/echo-server:0.9.2`; reached `live` and served echo JSON.
2. **Image → Git:** one combined update (`--repo https://github.com/bex-co/bex --branch main --root-directory examples/hello-go`) dispatched a `service_updated` build (root-directory is a build-triggering field, sweep 550). The old image kept serving during the build; at `live`, the body became hello-go's `OK`.
3. **Git → image:** `--image docker.io/ealen/echo-server:0.9.2` saved the image source without deploying (repo/branch cleared from the read, `undeployedChanges: true`, hello-go still serving). An explicit `deploys create` then rolled the echo image to `live` in ~30 seconds; body returned to echo JSON, revision advanced to `rev-4`, and `undeployedChanges` cleared.

The inactive `rootDir: examples/hello-go` stayed on the read after switching back to an image, as in sweep 551: retained build metadata, while `repo`/`branch` were absent and `imagePath` named the actual serving source. No step served a mixed or wrong source, and each deploy's identity (image ref vs commit) matched the leg that produced it.

## Cleanup

Deleted exact-owned `srv-dbb0825k8aos739580j0`; former URL and deploy reads returned `404`, and the workspace list no longer contains it.

## Unrun

- Switching back to Git again after the return leg (a second full cycle), and source switching on a suspended service.
- W8/129, w8/130, and w8/131 remain fixed on `main` ahead of the deployed pin; their live verification stays queued behind the next image pin.
