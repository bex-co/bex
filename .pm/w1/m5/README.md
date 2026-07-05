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

## Source

Converted from `.tmp/008-in-cluster-builds-webhook.md`.
