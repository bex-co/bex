# w1 · m5 — Build & deploy from git, in-cluster

**Worker:** worker1 **Goal:** Make `App.spec.repo` (build-from-git) work **in-cluster** on containerd nodes, image pullable by nodes, and redeploy on push. Today the operator shells `docker build` (impossible on containerd — the demo built locally and `ctr`-imported by hand). **Status:** todo

## Tasks (in order)

| id   | title                                             | est | depends_on |
| ---- | ------------------------------------------------- | --- | ---------- |
| t001 | Build via BuildKit/kpack Job → push to Zot        | 30m | —          |
| t002 | Verify build-from-git end-to-end (node pulls Zot) | 25m | t001       |
| t003 | Deploy-on-push HMAC webhook → bump revision       | 30m | —          |

## Definition of done

Setting `App.spec.repo` builds the image in-cluster (Dockerfile or CNB), pushes to Zot, and runs it; a git push to `App.branch` (with `autoDeploy`) bumps the revision automatically.

## Current state (2026-07-07)

- **Host-based MVP exists** (`lego/operator/internal/build/build.go`): `Build()` clones the repo, runs `docker build` (Dockerfile) or `pack build` (CNB) via `exec.Command`, and pushes to `BEX_REGISTRY`. This proved the full git-clone → build → push → run flow but shells out to host tools — impossible on containerd app nodes. Comment in the file explicitly calls out "an in-cluster BuildKit/kpack Job is the productionization."
- **Zot already deployed**: `deploy/gitops/base/zot.yaml` → `zot-0` StatefulSet in-cluster; nodes already pull from it (`BEX_REGISTRY` points there).
- **t001**: replace `exec.Command("docker", ...)` / `exec.Command("pack", ...)` in `build.go` with dispatching a Kubernetes Job that runs `moby/buildkit` or `kpack`. The `Build()` function signature can stay; only the backend changes.

## Source

Converted from `.tmp/008-in-cluster-builds-webhook.md`.
