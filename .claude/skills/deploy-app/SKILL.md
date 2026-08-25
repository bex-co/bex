---
name: deploy-app
description: >-
  Deploy a bex App to the current Kubernetes cluster and verify it reaches Ready. Use when the user asks to deploy a render.yaml (or legacy bex.yml) or the sample app and validate the rollout end to end. Optional argument: path to render.yaml.


allowed-tools: Bash(kubectl:*), Bash(bash scripts/app-apply.sh:*), Bash(curl:*), Bash(jq:*)
---

Deploy an App and verify it end-to-end. Use `KUBECONFIG=infra/local/bex.kubeconfig` if no other cluster is targeted.

Requires `BEX_API_URL` and `BEX_API_TOKEN` in the environment (see `.env.example` — `BEX_API_PUBLIC_URL` / `BEX_API_TOKEN`). `scripts/app-apply.sh` reads them and fails with `set BEX_API_URL` if unset. Optional `BEX_OWNER_ID`, `BEX_REPO`, `BEX_BRANCH` narrow the deploy.

1. If `$ARGUMENTS` is a path to a `render.yaml` (or legacy `bex.yml` alias — canonical is `render.yaml`, see `scripts/app-apply.sh`), apply it with `DRY_RUN=1 bash scripts/app-apply.sh $ARGUMENTS` first and show the generated App CR, then `bash scripts/app-apply.sh $ARGUMENTS` to deploy. If `$ARGUMENTS` is a directory, pass the directory — the script discovers `render.yaml` then `bex.yml` inside it. Otherwise apply the sample: `kubectl apply -f examples/whoami-app.yaml`.
2. Watch until ready: poll `kubectl get apps.app.bex.co <name> -o jsonpath='{.status.phase} {.status.revision} {.status.url}'` until phase is `Ready` (timeout ~3 minutes; on timeout, show `kubectl describe apps.app.bex.co <name>` and the operator logs `kubectl -n bex-system logs deploy/bex-controller-manager --tail=50`).
3. If `status.url` is set and resolvable, `curl -fsS` it (use `--resolve`/`-k` only if the local mock has no real DNS/TLS) and show the response status.
4. Report: App name, phase, revision, url, and where the pods landed (`kubectl get pods -l app.bex.co/app=<name> -o wide`).
