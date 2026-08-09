# Runbook: isolate the cluster-admin kubeconfig from the operator (codex #5)

**Status:** prepared, awaiting cluster execution + cert rotation. **Owner action required:** this runbook is a live-cluster migration plus an admin-cert rotation. Claude prepared the plan and the manifest diff; a cluster operator executes the steps and rotates.

## Finding

The operator's namespace-scoped Role `bex-operator-apps` (`deploy/gitops/base/operator-apps-rbac.yaml`) grants `get/list/watch` (and more) on **all** Secrets in the `default` namespace, and is bound to the `bex-controller-manager` ServiceAccount. The `default` namespace also holds `default/bex-kubeconfig` — a CAPI-generated kubeconfig whose embedded client cert is `CN=kubernetes-admin, O=system:masters` (full cluster-admin; see [ADR019](../ADR019-infra-credentials.md) and [ADR036](../ADR036-ca-rotation-runbook.md)). So a compromise of the operator pod or its SA token escalates from the operator's intended, scoped reconcile privileges to **cluster-admin** by reading one Secret.

CAPI PKI Secrets (`bex-ca`, `bex-etcd`, `bex-proxy`, `bex-sa`) live in `default` for the same reason and are exposed the same way.

## What makes this low-risk

**Post-pivot, nothing in-cluster consumes the `default/bex-kubeconfig` Secret:**

- The cluster-autoscaler runs `clusterAPIMode=incluster-incluster` (`deploy/gitops/base/autoscaler.yaml`) and needs **no** kubeconfig Secret. The `--set clusterAPIKubeconfigSecret=bex-kubeconfig` reference in `scripts/install-autoscaler.sh` is the **bootstrap-only** (pre-pivot) installer, uninstalled at pivot.
- Humans/CI fetch the admin credential from a control-plane node's `/etc/kubernetes/admin.conf` over SSH (`scripts/fetch-app-kubeconfig.sh`), **not** from this Secret.
- The only in-cluster references to `default/bex-kubeconfig` are (a) CAPI generating it and (b) one Prometheus alert (`AdminCertExpiringSoon`) that reads its creation timestamp as a cert-issue-time proxy.

So the Secret can be relocated out of `default` without breaking any runtime consumer. Only the alert's namespace selector needs to follow it.

## Recommended approach (A): relocate the CAPI Cluster resources to a locked-down namespace

Move the CAPI `Cluster`/`Machine*`/related resources into a dedicated namespace (`bex-capi`) that has **no** application-reconciler RBAC. CAPI regenerates `<cluster>-kubeconfig` and the PKI Secrets in that namespace, out of the operator's reach.

1. **Create the locked-down namespace** (no operator Role/RoleBinding targets it):

   ```bash
   kubectl create namespace bex-capi
   ```

2. **Move the CAPI resources** into it (pause the Cluster first per CAPI's move procedure):

   ```bash
   clusterctl move --namespace default --to-namespace bex-capi
   # verify the regenerated secrets landed there
   kubectl -n bex-capi get secret bex-kubeconfig bex-ca bex-etcd bex-proxy bex-sa
   ```

   > If `clusterctl move` between namespaces on a self-managed (pivoted) cluster is impractical for your topology, use **Approach B** below instead.

3. **Delete the stale `default` copies** once the `bex-capi` copies are confirmed:

   ```bash
   kubectl -n default delete secret bex-kubeconfig bex-ca bex-etcd bex-proxy bex-sa
   ```

4. **Cert-expiry alert** — no action needed. `AdminCertExpiringSoon` (`deploy/gitops/base/prometheus.yaml`) already matches `namespace=~"default|bex-capi"`, so it keeps working across the move (pre-applied, migration-agnostic). Nothing to change in lockstep.

5. **Update docs** that name `default/bex-kubeconfig` to `bex-capi/bex-kubeconfig`: `docs/ADR004-app-deployment.md`, `docs/ADR019-infra-credentials.md`, `docs/ADR036-ca-rotation-runbook.md`.

## Alternative approach (B): drop the operator's `default`-Secret read

If relocating the CAPI resources is impractical, remove the operator's ability to read `default` Secrets instead — valid **only if** no tenant App/Database/KeyValue resources still live in `default` (they now project into per-workspace `<ws>` namespaces via `namespaceFor = WorkspaceNamespace(tenantID)`, with per-namespace operator RBAC provisioned by the control plane's NamespaceReconciler).

1. Confirm nothing tenant-owned remains in `default`:

   ```bash
   kubectl -n default get apps.app.bex.co,databases.app.bex.co,keyvalues.app.bex.co
   # also confirm no CNPG "<db>-app" / clone / valkey Secrets the operator must read
   ```

2. If empty, delete the broad Role+RoleBinding `deploy/gitops/base/operator-apps-rbac.yaml` (or scope it to the actual per-workspace namespaces). Note: RBAC `list`/`watch` cannot be narrowed by `resourceNames`, so you cannot keep `list` on `default` Secrets while excluding `bex-kubeconfig` — the Role must not cover `default` at all.

## Verification (both approaches)

The operator SA must not be able to read the admin kubeconfig or CAPI PKI:

```bash
for s in bex-kubeconfig bex-ca bex-etcd bex-proxy bex-sa; do
  kubectl auth can-i get secret/$s -n default \
    --as=system:serviceaccount:bex-system:bex-controller-manager
done
# every line must print: no
```

Reconciliation must still pass with only the intended per-workspace Secret access — run the operator's envtest suite / a smoke deploy.

## Rotate the exposed admin cert

Because `kubernetes-admin` was readable by the operator, rotate it after relocation, per [ADR036 §1](../ADR036-ca-rotation-runbook.md):

```bash
# on each control-plane node
sudo kubeadm certs renew admin.conf
# then re-fetch and re-run CI's kubeconfig acquisition
HCLOUD_TOKEN=... scripts/fetch-app-kubeconfig.sh <out>
```

Full CA rotation (emergency) is ADR036 §2. Update `.env` / CI secrets if the fetched kubeconfig is cached anywhere per ADR019.

## Follow-up

- Link this runbook from [ADR028 §follow-up register](../ADR028-security-review.md) and [ADR019](../ADR019-infra-credentials.md).
- Consider giving the autoscaler and any future in-cluster CAPI client a **scoped, non-system:masters** credential (the pattern `scripts/operator-kubeconfig.sh` already mints and self-checks), so no cluster-admin kubeconfig needs to exist in a workload-reachable namespace at all.
