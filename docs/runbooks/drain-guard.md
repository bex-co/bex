# Runbook: drain-guard (node rolls hand single-instance services over)

**What it is:** `bex-system/bex-drain-guard` holds a CAPI pre-drain hook on every `bex-tenant-0` Machine. When a Machine is deleted, it cordons the node, rolls each App whose only Ready pods are there onto another node, and then releases the hook so CAPI drains. Design: [ADR004 § Node rolls](../ADR004-app-deployment.md#node-rolls-hand-single-instance-services-over-before-the-drain-w1m186). Board: `w1/m186`.

## What you will see during a roll

- The deleting Machine reports that it is waiting for a pre-drain hook (`kubectl -n bex-capi get machine <m> -o yaml`, `Deleting` condition). The hook is `pre-drain.delete.hook.machine.cluster.x-k8s.io/bex-drain-guard: bex-drain-guard`.
- The node is cordoned before CAPI drains it.
- Each handed-over App's Deployment gains `drain-guard.bex.co/evacuated-from: <node>` on its pod template. The annotation stays there, like `kubectl rollout restart`'s, and is harmless.
- drain-guard logs in this order: `cordoned the node`, `starting a replacement off the node`, then `releasing the pre-drain hook: <reason>`.

```bash
kubectl -n bex-system logs deploy/bex-drain-guard --since=1h
```

## The hold is bounded

drain-guard releases the hook at the latest **10 minutes** after the Machine's deletion (`--max-hold`), whatever the state of the replacements. It also releases at once in these cases:

- the Machine has no node;
- the node is gone or not Ready;
- every App on the node already has a Ready instance elsewhere;
- the only Apps there are disk-attached (`Recreate`).

A roll is therefore never pinned by drain-guard. The worst case is the eviction behavior from before m186.

## If drain-guard is down

CAPI waits for the hook while drain-guard is not running, because nothing else removes it. Restore the Deployment first (`kubectl -n bex-system rollout status deploy/bex-drain-guard`). If a roll must proceed without it, release the hook by hand:

```bash
kubectl -n bex-capi annotate machine <machine> pre-drain.delete.hook.machine.cluster.x-k8s.io/bex-drain-guard-
```

Removing the Deployment does not remove the hooks already on the Machines. To retire drain-guard, strip the hook from every Machine:

```bash
kubectl -n bex-capi annotate machines --all pre-drain.delete.hook.machine.cluster.x-k8s.io/bex-drain-guard-
```

## What the identity can do

`system:serviceaccount:bex-system:bex-drain-guard` holds the following grants, defined in `lego/operator/config/drain-guard/`:

- a Role in `bex-capi`: Machines get/list/watch/patch, and no Secrets;
- a ClusterRole: nodes get/patch, pods list, deployments get/patch.

The fail-closed policy `bex-drain-guard-writes` (`deploy/gitops/base/drain-guard-admission.yaml`) narrows every write to three changes:

- its own hook annotation on a Machine;
- `spec.unschedulable: true` on a Node;
- the evacuation annotation on a Deployment's pod template.

Check it with:

```bash
kubectl auth can-i get secret/bex-kubeconfig -n bex-capi --as=system:serviceaccount:bex-system:bex-drain-guard   # no
kubectl label node <n> x=y --as=system:serviceaccount:bex-system:bex-drain-guard --dry-run=server                # denied
kubectl cordon <n> --as=system:serviceaccount:bex-system:bex-drain-guard --dry-run=server                       # allowed
```
