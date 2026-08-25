---
name: deploy-app-from-local
description: >-
  Deploy a local checkout to a production Hetzner App using the manual image-import runbook. Use when the user asks to ship local working-tree code to a live bex App rather than deploy from remote Git. Arguments are the local repo path and app name.


allowed-tools: Bash(git:*), Bash(docker:*), Bash(ssh:*), Bash(scp:*), Bash(curl:*), Bash(kubectl:*), Bash(python3:*)
---

# Task: deploy a local checkout to prod Hetzner (manual image-import runbook)

**There is no build-from-git-in-cluster shortcut.** `App.spec.repo` + the operator's `build.Build()` (`lego/operator/internal/build/build.go`) is unimplemented in prod: the deployed operator image is `distroless/static:nonroot` (`lego/Dockerfile`) with no `git`/`docker`/`pack` binaries and no volumes (`lego/operator/config/manager/manager.yaml`). `.pm/w1/m5/README.md` (status: `todo`, never shipped) says this outright: _"the demo built locally and `ctr`-imported by hand."_ So this skill runs `docs/ADR004-app-deployment.md`'s manual runbook end-to-end. Do **not** use the `restart` verb as a substitute for a deploy — it only rolls pods on the App's existing cached `spec.image` (see `app_controller.go:120`); it never rebuilds anything.

This _is_ a genuine build-from-local-disk flow (unlike the old, wrong version of this skill): the image is built from whatever is on disk in `$ARGUMENTS`'s repo path right now, not from `origin/main`. If you want "latest remote main" instead, `git pull --ff-only` first.

## Step 0 — Confirm scope with the user before touching prod

This SSHes into a live Hetzner node and patches a running App CR — state a one-line plan (repo, app name, target image tag) and get a go-ahead before Step 4 (the actual `ctr import` + `kubectl patch`). Steps 1–3 (discovery, build, smoke test) are safe/local and don't need to wait for that confirmation.

## Step 1 — Fetch the app-cluster kubeconfig (self-managed, no infra node)

Since the w1/m19.1 pivot there is no mgmt cluster — `scripts/fetch-app-kubeconfig.sh` discovers a control-plane node via the hcloud API (label `caph-cluster-bex`) and SSH-fetches `/etc/kubernetes/admin.conf`:

```bash
# from the bex repo root (where this command runs)
set -a; source ./.env; set +a
# default key is ~/.ssh/id_bex (see scripts/fetch-app-kubeconfig.sh); override only if your key is elsewhere
BEX_SSH_KEY_PATH=~/.ssh/id_bex bash scripts/fetch-app-kubeconfig.sh /tmp/bex-app.kubeconfig
export KUBECONFIG=/tmp/bex-app.kubeconfig
kubectl get app <app-name> -o yaml   # baseline: current image, port, generation, revision
```

Never print `$HCLOUD_TOKEN` or kubeconfig contents (per repo rules) — only read specific `jsonpath` fields.

## Step 2 — Discover the node the App runs on (import target)

Apps run on the **tenant pool** (`bex-worker-*`), not the control plane — the image must be imported into the containerd of the node hosting the App's pod:

```bash
NODE=$(kubectl get pods -n default -l app.bex.co/app=<app-name> -o jsonpath='{.items[0].spec.nodeName}')
APP_NODE_IP=$(curl -sf -H "Authorization: Bearer $HCLOUD_TOKEN" \
  "https://api.hetzner.cloud/v1/servers?per_page=50" \
  | python3 -c "import json,sys; print([s['public_net']['ipv4']['ip'] for s in json.load(sys.stdin)['servers'] if s['name']=='$NODE'][0])")
```

(If the pod is Pending with no node yet — e.g. first deploy — import into every `bex-worker-*` node instead. ⚠ Known fragility: `ctr`-imported images live only in that node's containerd; an autoscaler node replacement loses them and the App falls to `ImagePullBackOff` until re-imported — re-run this runbook then.)

## Step 3 — Build + smoke-test — _laptop, from the local repo path in `$ARGUMENTS`_

```bash
cd <repo-path>
SHA=$(git rev-parse --short HEAD)
docker build --platform linux/amd64 -t <app-name>:$SHA .   # amd64 required — Hetzner node is x86_64
docker run -d --rm --name smoke -p 127.0.0.1:3999:<container-port> <app-name>:$SHA
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3999/   # expect 200
docker stop smoke
```

Use a **fresh tag** (git sha, never `:latest`) — the operator sets no `imagePullPolicy`, so k8s defaults to `IfNotPresent`; reusing a tag would silently serve the node's cached image.

## Step 4 — Import into the app node's containerd — _laptop → app node_ (confirm with user first, see Step 0)

```bash
docker save <app-name>:$SHA | ssh -i ~/.ssh/bex "root@${APP_NODE_IP}" \
  'ctr -n k8s.io images import -'
```

## Step 5 — Patch the App CR — _laptop → apiserver → operator_

```bash
KUBECONFIG=/tmp/bex-app.kubeconfig kubectl patch app <app-name> --type merge \
  -p "{\"spec\":{\"image\":\"<app-name>:$SHA\"}}"
```

This bumps `metadata.generation`, which is what makes the operator reconcile (it early-returns when already `Running` at the current generation).

## Step 6 — Watch the rollout and verify the edge serves the new code

```bash
KUBECONFIG=/tmp/bex-app.kubeconfig kubectl rollout status deployment/<app-name>
KUBECONFIG=/tmp/bex-app.kubeconfig kubectl get app <app-name>   # PHASE Running, REVISION bumped
curl -s https://<app-name>.onbex.co/ | grep -i '<something only in the new build>'
```

A `200` alone doesn't prove anything — grep for content that only exists in the new commit (e.g. the new blog post title, a changed string). This is the step that actually confirms a deploy happened, unlike the old skill's mistake of trusting a `restart` call's `200`.

## Step 7 — Report

App name, old image tag → new image tag (git sha), old revision → new revision, and the content check that proved the new code is live. Clean up `/tmp/bex-app.kubeconfig` when done.
