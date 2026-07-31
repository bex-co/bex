---
name: mock-cluster
description: >-
  Stand up or scale the local CAPD mock cluster and deploy the bex operator into it. Use when the user asks to create, inspect, or resize the repository's local development cluster. Optional argument: scale N.


allowed-tools: Bash(bash scripts/mock-cluster.sh:*), Bash(kubectl:*), Bash(docker:*), Bash(make:*), Bash(kind:*), Bash(clusterctl:*)
---

Bring up the local mock of the Hetzner substrate and deploy the bex operator into it.

If `$ARGUMENTS` is `scale N`: just run `bash scripts/mock-cluster.sh scale N`, then show `kubectl get nodes` (with `KUBECONFIG=infra/local/bex.kubeconfig`) and stop.

Otherwise, run the full bring-up:

1. `bash scripts/mock-cluster.sh` — kind infra cluster + Cluster API + CAPD app cluster, plus the app-cluster baseline the operator needs: Calico, coredns/calico-kube-controllers control-plane pins, local-path default StorageClass, cert-manager (prod's pinned version; `make deploy`'s webhook cert requires it). This can take several minutes; it writes the app-cluster kubeconfig to `infra/local/bex.kubeconfig`.
2. Use that kubeconfig for every following command: `export KUBECONFIG=$PWD/infra/local/bex.kubeconfig` (or `--kubeconfig`). Never print its contents.
3. Build and load the operator image (CAPD nodes can't pull local-only images):
   - `( cd lego/operator && make docker-build IMG=bex-operator:dev )`
   - `docker save bex-operator:dev -o /tmp/bex-op.tar`
   - For each node in `kubectl get nodes -o name`: `docker cp /tmp/bex-op.tar <node>:/op.tar && docker exec <node> ctr -n k8s.io images import /op.tar`
4. `( cd lego/operator && make deploy IMG=bex-operator:dev )` — deploys to ns `bex-system` with `BEX_RUNTIME=kubernetes`. The `bex-ssh-gateway` IngressRouteTCP fails without Traefik CRDs — expected and harmless on the mock; everything else applies. Then `kubectl apply -f deploy/gitops/base/operator-apps-rbac.yaml` — the apps-namespace `bex-operator-apps` Role/RoleBinding is Argo-managed in prod and absent from the operator's kustomize config; without it the manager crashloops on "failed to wait for … caches to sync" (its namespace-scoped Secret/Job watches in `default` are forbidden; w1/043).
5. Local-CAPD-only fixes (re-apply all three after **every** `make deploy` — it reverts them; w1/043):
   - Label the control-plane node into the platform pool (the manifest's nodeSelector is `bex.co/pool=platform`, and the operator must sit on the control-plane node because OrbStack/Calico can't route cross-node pod→apiserver; see docs/ADR004-app-deployment.md): `kubectl label node <control-plane-node> bex.co/pool=platform --overwrite`
   - Pin the operator to the control-plane node:
     ```
     kubectl -n bex-system patch deploy bex-controller-manager --type merge -p \
      '{"spec":{"template":{"spec":{"nodeSelector":{"node-role.kubernetes.io/control-plane":""},
       "tolerations":[{"key":"node-role.kubernetes.io/control-plane","effect":"NoSchedule"}]}}}}'
     ```
   - The manifest sets `imagePullPolicy: Always`, which can never be satisfied by the ctr-imported local image (ErrImagePull) — override it:
     ```
     kubectl -n bex-system patch deploy bex-controller-manager --type strategic -p \
      '{"spec":{"template":{"spec":{"containers":[{"name":"manager","imagePullPolicy":"IfNotPresent"}]}}}}'
     ```
6. `kubectl -n bex-system rollout status deploy/bex-controller-manager` — wait for Ready.

Finish by reporting: node list, operator pod status, and the one-liners to deploy an App (`/deploy-app`) and to scale machines (`/mock-cluster scale N`).
