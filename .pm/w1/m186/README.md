# w1 · m186 — Keep single-instance services up through node rolls: drain-guard pre-drain hook

**Worker:** worker1 **Goal:** a CAPI node roll never takes a single-instance service down. Before CAPI drains a guarded Machine's node, each bex App whose only Ready pods are on that node already has a Ready replacement on another node. **Status:** in progress (t001–t006 done; the Definition of done is met on the local mock; t007 Closeout once the change is shipped and deployed)

## Tasks (in order)

| id   | title                                                     | est   | depends_on |
| ---- | --------------------------------------------------------- | ----- | ---------- |
| t001 | drain-guard controller: hook, evacuate, release — **DONE**           | 1h30m | —          |
| t002 | Wire drain-guard into the image, operator config and GitOps — **DONE** | 45m   | t001       |
| t003 | Admission boundary for the drain-guard identity, runbooks — **DONE** | 45m   | t002       |
| t004 | Live check on the local CAPD mock — **DONE**                         | 1h    | t003       |
| t005 | Simplify — **DONE**                                                  | 15m   | t004       |
| t006 | Test coverage — **DONE**                                             | 30m   | t004       |
| t007 | Closeout                                                  | 10m   | t006       |

## Definition of done

1. A guarded-pool Machine is deleted. Before CAPI drains its node, every bex App Deployment whose only Ready pods were on that node has a Ready replacement on another node. This is measured on the local CAPD mock, with a request loop against such an App seeing zero failed responses during the roll.
2. A replacement that cannot become Ready releases the hook within 10 minutes of the Machine's deletion, so drain-guard never pins a node.
3. The drain-guard identity is denied every Machine, Node and Deployment write except its own hook annotation, a cordon, and the evacuation annotation. It cannot read any `bex-capi` Secret. Both are shown with `kubectl --as` and `kubectl auth can-i` probes.
4. The operator's RBAC (`lego/operator/config/rbac/role.yaml`) is unchanged.

## Decision (2026-10-25)

`w1/138` asked whether a bex controller may patch CAPI Machines in the locked-down `bex-capi` namespace. The agent decided, on the user's delegation ("你自己决定", then "you should do by yourself"), on **a separate, narrowly scoped controller**. The operator was rejected for this job because the hook needs CAPI Machine, Node and Deployment rights:

- **Identity.** It runs as `bex-system:bex-drain-guard`, its own ServiceAccount.
- **Grants.** A Role in `bex-capi` gives get/list/watch/patch on `machines.cluster.x-k8s.io`, and nothing on Secrets. A ClusterRole gives nodes get/patch, pods list, and deployments get/patch. Workload reads are uncached, so no watch is needed.
- **Admission policy.** A ValidatingAdmissionPolicy limits its writes to three changes:
  - on Machines, only the `pre-drain.delete.hook.machine.cluster.x-k8s.io/bex-drain-guard` annotation;
  - on Nodes, only `spec.unschedulable` false→true (a cordon);
  - on Deployments, only the pod-template annotation `drain-guard.bex.co/evacuated-from`.

  The policy holds those limits even if its RBAC later drifts wider.
- **Unchanged.** The operator's RBAC and the `bex-capi` Secret lock-down (`docs/runbooks/operator-kubeconfig-isolation.md`) stay as they are.

## Source + Goal linkage

- **Source:** [`.pm/w1/done/138.md`](../done/138.md), promoted 2026-10-25. It came from the 2026-10-24 w5/174 tenant-pool roll, which evicted `beancount-cms-v2`'s only pod. Its replacement sat `Pending`, and beancount.io answered `no available server`.
- **Goal linkage:** this is the hosting core of the Render alternative (ADR008). Render does not drop a service during host maintenance.
- **Expected outcome:** a routine `bex-tenant-0` roll or MachineHealthCheck replacement costs a single-instance service zero failed requests. The only exception is a service whose replacement cannot start within 10 minutes; that service gets today's eviction behavior.
- **Why now:** m183 (images survive retention) and tenant-0 max 5 (room to reschedule) stopped the long outage. Every roll still costs each single-instance service seconds of downtime, and rolls are routine: Kubernetes minors, image refreshes, machine-type moves (ADR053).
- **Render parity task omitted:** no REST, GraphQL, MCP or UI surface changes. This is a platform-internal mechanism.
- **Ruled out (from 138):**
  - A zero-disruption PDB on single-instance pods, which kept nodes undrainable for 6 days (ADR060 D8).
  - Reacting to a cordon, which cannot work because CAPI cordons and evicts in one reconcile.
- **Out of scope:** Cluster Autoscaler scale-down evicts pods itself before it deletes the Machine. That is filed as `w1/142`.
