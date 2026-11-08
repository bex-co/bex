---
name: mock-cluster
description: >-
  Stand up or scale the local CAPD mock cluster and deploy the bex operator into it. Use when the user asks to create, inspect, or resize the repository's local development cluster. Optional argument: scale N.


allowed-tools: Bash(bash scripts/mock-cluster.sh:*), Bash(kubectl:*), Bash(docker:*), Bash(make:*), Bash(kind:*), Bash(clusterctl:*), Bash(cut:*)
---

Bring up the local mock of the Hetzner substrate and deploy the bex operator into it.

If `$ARGUMENTS` is `scale N`: just run `bash scripts/mock-cluster.sh scale N`, then show `kubectl get nodes` (with `KUBECONFIG=infra/local/bex.kubeconfig`) and stop.

Otherwise, run the full bring-up:

1. `bash scripts/mock-cluster.sh` — kind infra cluster + Cluster API + CAPD app cluster, plus the app-cluster baseline the operator needs: Calico, coredns/calico-kube-controllers control-plane pins, local-path default StorageClass, cert-manager (prod's pinned version; `make deploy`'s webhook cert requires it), and the Traefik edge with its `traefik.io` CRDs (the chart and sha256 `deploy/helm-artifacts.lock` pins for production, with the base values). This can take several minutes. It writes the app-cluster kubeconfig to `infra/local/bex.kubeconfig` and the mgmt kubeconfig to `infra/local/bex-mgmt.kubeconfig`. It never reads or changes `~/.kube/config`'s current-context, and its helm calls use an isolated registry config, so a locked macOS keychain cannot hang them.
2. `bash scripts/mock-cluster.sh operator` — builds `bex-operator:dev` from `lego/Dockerfile`, imports it into containerd on every node, runs `GOWORK=off make -C lego/operator deploy IMG=bex-operator:dev`, applies `deploy/gitops/base/operator-apps-rbac.yaml`, sets `imagePullPolicy: IfNotPresent` on every bex-system workload running the image, restarts them, and waits for `bex-controller-manager`. Each step names itself on failure. Re-run it after changing operator code. `BEX_OPERATOR_IMG` overrides the tag.
3. Use the workload kubeconfig for every following command: `export KUBECONFIG=$PWD/infra/local/bex.kubeconfig` (or `--kubeconfig`). Never print its contents. For the mgmt cluster use `kubectl --kubeconfig infra/local/bex-mgmt.kubeconfig --context kind-bex-mgmt`.

Finish by reporting: node list, operator pod status, and the one-liners to deploy an App (`/deploy-app`) and to scale machines (`/mock-cluster scale N`).
