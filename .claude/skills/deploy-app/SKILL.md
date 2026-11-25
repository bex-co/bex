---
name: deploy-app
metadata:
  internal: "true"
description: >-
  Deploy a bex App to the current Kubernetes cluster and verify it reaches Running. Use when the user asks to deploy a render.yaml (or legacy bex.yml) or the sample app and validate the rollout end to end. Optional argument: path to render.yaml.


allowed-tools: Bash(kubectl:*), Bash(bash scripts/app-apply.sh:*), Bash(curl:*), Bash(jq:*)
---

Deploy an App and verify it end-to-end. Use `KUBECONFIG=infra/local/bex.kubeconfig` if no other cluster is targeted.

Requires `BEX_API_URL` and `BEX_API_TOKEN` in the environment (`scripts/app-apply.sh` reads them and fails with `set BEX_API_URL` if unset — both are client-supplied, not in `.env.example`; `BEX_API_PUBLIC_URL` in `.env.example` is the unrelated backend DeployHook origin). `BEX_API_TOKEN` is the Hydra-issued Bearer token (API key minted via `/v1/api-keys`). Optional `BEX_OWNER_ID`, `BEX_REPO`, `BEX_BRANCH` narrow the deploy.

1. If `$ARGUMENTS` is a path to a `render.yaml` (or legacy `bex.yml` alias — canonical is `render.yaml`, see `scripts/app-apply.sh`), run `DRY_RUN=1 bash scripts/app-apply.sh $ARGUMENTS` first and show bex-api's validation result (`/v1/blueprints/validate`; the script writes no App CR itself), then `bash scripts/app-apply.sh $ARGUMENTS` to deploy. If `$ARGUMENTS` is a directory, pass the directory — the script discovers `render.yaml` then `bex.yml` inside it. Otherwise apply the sample: `kubectl apply -f examples/whoami-app.yaml`.
2. Find the App. The sample is `whoami` in `default`. A Blueprint deploy is created by bex-api in the workspace's namespace under bex-api's own object name (ADR043), so take the namespace and name from `kubectl get apps.app.bex.co -A`.
3. Watch until ready: poll `kubectl -n <namespace> get apps.app.bex.co <name> -o jsonpath='{.status.phase} {.status.activeRevision} {.status.url}'` until phase is `Running`. Stop at once on `Failed`. Otherwise time out after ~3 minutes for a prebuilt image; a source build stays in `Building` longer, so keep polling while the phase is still advancing. On failure or timeout, show `kubectl -n <namespace> describe apps.app.bex.co <name>` and the operator logs `kubectl -n bex-system logs deploy/bex-controller-manager --tail=50`.
4. If `status.url` is set and resolvable, `curl -fsS` it (use `--resolve`/`-k` only if the local mock has no real DNS/TLS) and show the response status.
5. Report: App name, namespace, phase, revision, url, and where the pods landed (`kubectl -n <namespace> get pods -l app.bex.co/app=<name> -o wide`).
