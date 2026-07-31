---
name: mock-cluster
description: >-
  Stand up or scale the local CAPD mock cluster and deploy the bex operator into it. Use when the user asks to create, inspect, or resize the repository's local development cluster. Optional argument: scale N.


allowed-tools: Bash(bash scripts/mock-cluster.sh:*), Bash(kubectl:*), Bash(docker:*), Bash(make:*), Bash(kind:*), Bash(clusterctl:*)
---

Bring up the local mock of the Hetzner substrate and deploy the bex operator into it.

If `$ARGUMENTS` is `scale N`: just run `bash scripts/mock-cluster.sh scale N`, then show `kubectl get nodes` (with `KUBECONFIG=infra/local/bex.kubeconfig`) and stop.

Otherwise, run the full bring-up:

1. `bash scripts/mock-cluster.sh` — kind infra cluster + Cluster API + CAPD app cluster. This can take several minutes; it writes the app-cluster kubeconfig to `infra/local/bex.kubeconfig`.
2. Use that kubeconfig for every following command: `export KUBECONFIG=$PWD/infra/local/bex.kubeconfig` (or `--kubeconfig`). Never print its contents.
3. Build and load the operator image (CAPD nodes can't pull local-only images):
   - `( cd operator && make docker-build IMG=bex-operator:dev )`
   - `docker save bex-operator:dev -o /tmp/bex-op.tar`
   - For each node in `kubectl get nodes -o name`: `docker cp /tmp/bex-op.tar <node>:/op.tar && docker exec <node> ctr -n k8s.io images import /op.tar`
4. `( cd operator && make deploy IMG=bex-operator:dev )` — deploys to ns `bex-system` with `BEX_RUNTIME=kubernetes`.
5. Local-CAPD-only fix — pin the operator to the control-plane node (OrbStack/Calico can't route cross-node pod→apiserver; see docs/ADR004-app-deployment.md):
   ```
   kubectl -n bex-system patch deploy bex-controller-manager --type merge -p \
    '{"spec":{"template":{"spec":{"nodeSelector":{"node-role.kubernetes.io/control-plane":""},
     "tolerations":[{"key":"node-role.kubernetes.io/control-plane","effect":"NoSchedule"}]}}}}'
   ```
6. `kubectl -n bex-system rollout status deploy/bex-controller-manager` — wait for Ready.

Finish by reporting: node list, operator pod status, and the one-liners to deploy an App (`/deploy-app`) and to scale machines (`/mock-cluster scale N`).
