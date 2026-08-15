# Runbook: isolate the cluster-admin kubeconfig from the operator (codex #5)

**Status:** production namespace migration and admin-credential refresh completed 2026-08-14. Production manifests, rollout checks, autoscaler placement, and CI use `bex-capi`; CI refuses to apply them over an unmigrated `default/bex` Cluster. Because Kubernetes has no client-certificate revocation mechanism, the replaced certificates remain valid until their original expiry unless the cluster CA is rotated.

## Finding

Before this remediation, the operator's namespace-scoped Role `bex-operator-apps` granted `get/list/watch` (and more) on **all** Secrets in `default`, while CAPI also generated `default/bex-kubeconfig` there. Its embedded client cert is `CN=kubernetes-admin, O=system:masters`, so an operator compromise escalated to cluster-admin by reading one Secret. The repository now puts every production CAPI object and generated Secret in `bex-capi`, where neither the operator nor bex-api has RBAC.

CAPI PKI Secrets (`bex-ca`, `bex-etcd`, `bex-proxy`, `bex-sa`) live in `default` for the same reason and are exposed the same way.

## What makes this low-risk

**Post-pivot, nothing in-cluster consumes the CAPI-generated `bex-kubeconfig` Secret:**

- The cluster-autoscaler runs `clusterAPIMode=incluster-incluster` (`deploy/gitops/base/autoscaler.yaml`) and needs **no** kubeconfig Secret. The `--set clusterAPIKubeconfigSecret=bex-kubeconfig` reference in `scripts/install-autoscaler.sh` is the **bootstrap-only** (pre-pivot) installer, uninstalled at pivot.
- Humans/CI fetch the admin credential from a control-plane node's `/etc/kubernetes/admin.conf` over SSH (`scripts/fetch-app-kubeconfig.sh`), **not** from this Secret.
- The only in-cluster references are CAPI generating `bex-capi/bex-kubeconfig` and one Prometheus alert (`AdminCertExpiringSoon`) reading its creation timestamp as a cert-issue-time proxy.

So the Secret can be relocated out of `default` without breaking any runtime consumer. Only the alert's namespace selector needs to follow it.

## Recommended approach (A): relocate the CAPI Cluster resources to a locked-down namespace

Move the CAPI `Cluster`/`Machine*`/related resources into a dedicated namespace (`bex-capi`) that has **no** application-reconciler RBAC. CAPI regenerates `<cluster>-kubeconfig` and the PKI Secrets in that namespace, out of the operator's reach.

1. **Create the locked-down namespace** (no operator Role/RoleBinding targets it):

   ```bash
   kubectl create namespace bex-capi
   ```

2. **Move the CAPI resources** into it during a maintenance window. Kubernetes namespaces are immutable, and the `clusterctl` CLI preserves an object's namespace. Production was moved on 2026-08-14 by a one-time protected workflow using clusterctl v1.13.2's library `Move` operation with its experimental resource-mutator hook. The mutator rewrote only `default` object namespaces and known namespaced CAPI references to `bex-capi`; clusterctl created the complete target graph, rebuilt owner-reference UIDs, force-deleted the paused source graph with its `delete-for-move` annotation, and then resumed the target.

   The implementation is preserved in the history beginning at commit `83cf22c3`; the intentionally temporary workflow and Go mutator were removed after the migration and should not be replayed blindly. Before the move it required one healthy `default/bex` Cluster, no target Cluster, a converged fleet, a fresh successful off-cluster etcd snapshot, a credential-bearing in-run graph backup, and a successful clusterctl dry run. Afterward a separate read-only pass required the target graph and five PKI/admin Secrets, no source copies, a converged fleet, and a negative operator `can-i` check for every target Secret. The migration completed in [Actions run 31861196794](https://github.com/bex-co/bex/actions/runs/31861196794), the corrected read-only verification passed in [run 31861540662](https://github.com/bex-co/bex/actions/runs/31861540662), and the canonical app-cluster workflow passed in [run 31861593153](https://github.com/bex-co/bex/actions/runs/31861593153).

   > **Do not use `clusterctl move --to-directory` followed by `--from-directory` for namespace relocation.** In clusterctl v1.13.2, `to-directory` is a backup operation: it pauses, serializes, and then resumes the source objects without deleting them. Restoring a namespace-rewritten directory would create a second live controller graph. The library `Move` path is required because it couples target creation and owner-UID repair with source deletion.

   The one-time workflow was designed to resume the sole surviving graph on failure. If both roots existed it paused both rather than allowing competing reconciliation; if neither root existed it restored the original graph from the protected in-run backup. The fresh off-cluster etcd snapshot was the final recovery boundary.

3. **Cert-expiry alert** — deploy the repository version after the move. `AdminCertExpiringSoon` now selects only `bex-capi/bex-kubeconfig`.

4. **Run the app-cluster workflow.** It verifies no legacy `default/bex` Cluster remains before applying the canonical `bex-capi` overlay.

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
  kubectl auth can-i get secret/$s -n bex-capi \
    --as=system:serviceaccount:bex-system:bex-controller-manager
done
# every line must print: no
```

Reconciliation must still pass with only the intended per-workspace Secret access — run the operator's envtest suite / a smoke deploy.

## Refresh the exposed admin credentials

Because `kubernetes-admin` was readable by the operator, every control-plane node's `admin.conf` and the independently generated `bex-capi/bex-kubeconfig` Secret were refreshed after relocation. The protected one-time implementation is preserved at commit `328c0abb`; it took a fresh off-cluster etcd snapshot, renewed and verified each node serially, backed up and regenerated the CAPI Secret, rechecked operator denial and fleet convergence, and then passed the canonical app-cluster workflow. See [rotation run 31862133147](https://github.com/bex-co/bex/actions/runs/31862133147) and [app-cluster run 31862253647](https://github.com/bex-co/bex/actions/runs/31862253647).

For a future node-local refresh, per [ADR036 §1](../ADR036-ca-rotation-runbook.md):

```bash
# on each control-plane node
sudo kubeadm certs renew admin.conf
# then re-fetch and re-run CI's kubeconfig acquisition
HCLOUD_TOKEN=... scripts/fetch-app-kubeconfig.sh <out>
```

That command does **not** update the CAPI-managed kubeconfig Secret; its lifecycle and safe regeneration are documented in ADR036. Neither renewal invalidates a previously copied client certificate: with the CA unchanged it remains accepted until its original expiry. Full CA rotation (emergency) is ADR036 §2 and is the only documented way to invalidate it immediately. Update `.env` / CI secrets if the fetched kubeconfig is cached anywhere per ADR019.

## Follow-up

- Link this runbook from [ADR028 §follow-up register](../ADR028-security-review.md) and [ADR019](../ADR019-infra-credentials.md).
- Consider giving the autoscaler and any future in-cluster CAPI client a **scoped, non-system:masters** credential (the pattern `scripts/operator-kubeconfig.sh` already mints and self-checks), so no cluster-admin kubeconfig needs to exist in a workload-reachable namespace at all.
