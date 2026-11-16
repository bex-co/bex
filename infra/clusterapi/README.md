# infra/clusterapi — node lifecycle via Cluster API

Declarative machines. A `Cluster` + `MachineDeployment` describe the desired node pool; the Cluster API controllers (installed via `clusterctl init`, kept in GitOps under `deploy/gitops/base/cluster-api.yaml`) reconcile them into real nodes.

- **`base/`** — shared, provider-agnostic bits (namespace, cluster-autoscaler annotations, MachineDeployment replica defaults used as patch targets).
- **`overlays/local-capd/`** — the full CAPD app-cluster manifest (Docker containers as machines). Generated with `clusterctl generate cluster ... --infrastructure docker`. Used by `infra/local`.
- **`overlays/hetzner-caph/`** — the CAPH equivalent (Hetzner). Same `Cluster` / `MachineDeployment` shape; provider-specific `*MachineTemplate`. [seam]

**Add / remove a machine:**

```
kubectl scale machinedeployment <name> --replicas=N     # or edit replicas
# or: let cluster-autoscaler scale it from pending pods (annotations in base/)
```

This is `bex-infra`; the bex control plane only observes the resulting `Node`s.

## Production version, bootstrap, and recovery

Production is pinned to Kubernetes **v1.34.9** in the CAPH overlay and to the matching `bex-worker-k8s-1-34` baked-worker selector. A fresh bootstrap or disaster-recovery cluster applies that current declaration directly; it does not replay historical Kubernetes minors.

An existing cluster is different: Kubernetes and kubeadm upgrades must advance exactly one minor at a time. The completed production path was `1.31 → 1.32.13 → 1.33.13 → 1.34.9`. For a future upgrade:

1. Take and verify fresh application-cluster etcd and OpenBao snapshots. Keep the preceding manifest commit and active worker snapshot selector as the rollback point.
2. Bake the target-minor worker image, then stage control plane, burst canary, tenant baseline, and platform pools in separate reviewed commits. Never combine minors or roll every pool in one mutation.
3. After each stage, run `scripts/wait-capi-rollout.sh`; after each minor, run `scripts/verify-substrate.sh`, the App lifecycle smoke, and the managed Postgres lifecycle/SQL smoke. Take new recovery snapshots before proceeding.
4. If a stage fails, stop the next stage. Revert to the immediately preceding declaration and retained worker image. Restore etcd only when the CAPI state itself is damaged; ordinary replacement failure is repaired declaratively.

A roll can also stall on Hetzner capacity rather than on anything declared here. When Hetzner refuses a server (`resource_unavailable`, for example the `cx` line sold out in every location on 2026-11-16), CAPH marks that surge Machine's HCloudMachine `ServerCreateFailedIrrecoverableError` and never retries it, and the pools' health-check remediation skips it because there is no server to reboot. The 2026-11-12 platform roll sat on one such Machine for four days. `scripts/wait-capi-rollout.sh` deletes a MachineSet-owned Machine whose server was refused five minutes ago (`SERVER_CREATE_RETRY_SECONDS`), so the MachineSet retries the creation and the roll completes as soon as capacity returns. A Machine that has a server is never deleted, and a control-plane Machine is reported, not deleted. If capacity stays out, the wait times out with the HCloudMachine reasons in its diagnostics; the old nodes keep serving, because CAPI releases an old Machine only once its replacement is Ready. Do not answer a stock-out by moving a pool to another server type unless [ADR053](../../docs/ADR053-node-instance-types.md) calls for it: that surrenders the working `cx` servers, which cannot be recreated while the line is out of stock.

The exact rollback objects, workflow runs, compatibility findings, and per-minor verification evidence are recorded in [`docs/drills/2026-07-27-platform-deprecation-preflight.md`](../../docs/drills/2026-07-27-platform-deprecation-preflight.md). The data restore procedures remain [`ADR011-etcd-backup-restore.md`](../../docs/ADR011-etcd-backup-restore.md) and [`ADR015-openbao-backup-restore.md`](../../docs/ADR015-openbao-backup-restore.md).
