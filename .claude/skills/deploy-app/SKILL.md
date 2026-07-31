---
name: deploy-app
description: >-
  Deploy a bex App to the current Kubernetes cluster and verify it reaches Ready. Use when the user asks to deploy a bex.yml or the sample app and validate the rollout end to end. Optional argument: path to bex.yml.


allowed-tools: Bash(kubectl:*), Bash(bash scripts/app-apply.sh:*), Bash(curl:*)
---

Deploy an App and verify it end-to-end. Use `KUBECONFIG=infra/local/bex.kubeconfig` if no other cluster is targeted.

1. If `$ARGUMENTS` is a path to a `bex.yml`, apply it with `bash scripts/app-apply.sh $ARGUMENTS` (run `DRY_RUN=1` first and show the generated App CR). Otherwise apply the sample: `kubectl apply -f examples/whoami-app.yaml`.
2. Watch until ready: poll `kubectl get apps.app.bex.co <name> -o jsonpath='{.status.phase} {.status.revision} {.status.url}'` until phase is `Ready` (timeout ~3 minutes; on timeout, show `kubectl describe apps.app.bex.co <name>` and the operator logs `kubectl -n bex-system logs deploy/bex-controller-manager --tail=50`).
3. If `status.url` is set and resolvable, `curl -fsS` it (use `--resolve`/`-k` only if the local mock has no real DNS/TLS) and show the response status.
4. Report: App name, phase, revision, url, and where the pods landed (`kubectl get pods -l app.bex.co/app=<name> -o wide`).
