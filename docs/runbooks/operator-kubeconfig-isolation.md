# Runbook: isolate the cluster-admin kubeconfig from the operator (codex #5)

**Status:** repository desired state implemented; live cluster migration + cert rotation remain an operator procedure. Production manifests, rollout checks, autoscaler placement, and CI now use `bex-capi`; CI refuses to apply them over an unmigrated `default/bex` Cluster.

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

2. **Move the CAPI resources** into it during a maintenance window. Kubernetes
   namespaces are immutable, and `clusterctl move --to-kubeconfig` preserves an
   object's namespace; merely changing the destination context namespace does
   not relocate the graph. Use the directory move path so the serialized graph
   can be transformed explicitly. This is outside clusterctl's E2E-tested
   bootstrap-pivot case, so first exercise the exact procedure against a
   production snapshot in an isolated management cluster.

   ```bash
   migration_dir="$(mktemp -d)"
   chmod 700 "$migration_dir"

   # Inspect the discovered graph before the destructive move-to-directory.
   clusterctl move --namespace default --to-directory "$migration_dir" --dry-run -v 5
   clusterctl move --namespace default --to-directory "$migration_dir"

   # Rewrite only objects from the former workload-cluster namespace. Provider
   # objects discovered in capi-system/caph-system/etc. retain their namespace.
   while IFS= read -r -d '' object_file; do
     yq -i 'if .metadata.namespace == "default" then .metadata.namespace = "bex-capi" else . end' "$object_file"
   done < <(find "$migration_dir" -type f \( -name '*.yaml' -o -name '*.yml' \) -print0)
   if rg -n 'namespace: default' "$migration_dir"; then
     echo "unmigrated default-namespace object remains in the CAPI graph" >&2
     exit 1
   fi

   # Review the transformed credential-bearing files on the secured host, then
   # restore the graph. from-directory rebuilds owner-reference UIDs.
   clusterctl move --from-directory "$migration_dir" --dry-run -v 5
   clusterctl move --from-directory "$migration_dir"

   # verify the regenerated secrets landed there
   kubectl -n bex-capi get secret bex-kubeconfig bex-ca bex-etcd bex-proxy bex-sa
   ```

   The directory contains cluster PKI and must be treated as a root credential:
   keep it on an encrypted operator host, never attach it to a ticket or commit,
   and securely remove it after the verification and rotation below.

   > If `clusterctl move` between namespaces on a self-managed (pivoted) cluster is impractical for your topology, use **Approach B** below instead.

3. **Delete the stale `default` copies** once the `bex-capi` copies are confirmed:

   ```bash
   kubectl -n default delete secret bex-kubeconfig bex-ca bex-etcd bex-proxy bex-sa
   ```

4. **Cert-expiry alert** — deploy the repository version after the move. `AdminCertExpiringSoon` now selects only `bex-capi/bex-kubeconfig`.

5. **Run the app-cluster workflow.** It now verifies no legacy `default/bex` Cluster remains before applying the canonical `bex-capi` overlay.

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
