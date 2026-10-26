# w1 · m187 — Keep single-instance services up through Cluster Autoscaler scale-downs

**Worker:** worker1 **Goal:** a Cluster Autoscaler consolidation never takes a single-instance service down. Before CA evicts the pods of a guarded tenant node it is removing, each bex App whose only Ready pods are on that node already has a Ready replacement on another node. **Status:** done (2026-10-26; the Definition of done was met on the local CAPD mock, and the production rollout rides the next deploy)

## Tasks (in order)

| id   | title                                                               | est   | depends_on |
| ---- | ------------------------------------------------------------------- | ----- | ---------- |
| t001 | drain-guard evacuates a node Cluster Autoscaler has marked for removal — **DONE** | 1h30m | —          |
| t002 | Give drain-guard a window: CA's delay between taint and drain, guard, docs — **DONE** | 45m   | t001       |
| t003 | Live check on the local CAPD mock — **DONE** | 1h    | t002       |
| t004 | Simplify — **DONE** | 15m   | t003       |
| t005 | Test coverage — **DONE** | 30m   | t003       |
| t006 | Closeout — **DONE** | 10m   | t005       |

## Definition of done

1. Cluster Autoscaler scales down a guarded tenant node that holds a single-instance App. Before CA evicts the App's pod, the App has a Ready replacement on another node. On the local CAPD mock, a request loop against the App sees zero failed responses through the scale-down.
2. A replacement that cannot become Ready never stops a scale-down: CA evicts and deletes the node after its delay, exactly as before m187.
3. drain-guard's grants widen only by `nodes` list/watch. The admission policy `bex-drain-guard-writes` is unchanged, and its envtest still passes.
4. `scripts/clusterapi-validate.sh` fails if the CA delay is removed or made shorter than the window drain-guard needs.

## Decision (2026-10-26)

`w1/142` listed three options. The agent decided on the user's standing delegation ("你自己决定", "you should do by yourself"):

- **Chosen: evacuate on CA's `ToBeDeletedByClusterAutoscaler` taint, inside CA's own delay.** 142 ruled this out as a race, because CA evicts right after it taints. That holds only at CA's default. In CA 1.34.5, `Actuator.startDeletion` taints synchronously (`taintNodesSync`), and `deleteNodesAsync` then sleeps `--node-delete-delay-after-taint` (default 5s) before it snapshots the node and drains it (`core/scaledown/actuation/actuator.go:294-296`). Raising that delay gives drain-guard a window. The taint is `NoSchedule`, so the replacement never lands back on the node, and no cordon is needed.
- **Rejected: `safe-to-evict: "false"` on single-instance pods.** It blocks consolidation of every node holding one, which is most tenant nodes, and gives up the bin-pack economics `ADR004 § Rollout headroom` protects.
- **Rejected: accept the gap.** Render does not drop a service when it consolidates hosts (ADR008 parity).

## Source + Goal linkage

- **Source:** [`.pm/w1/done/142.md`](../142.md), promoted 2026-10-26 by `/loopx w1`. It was filed from w1/m186's scoping: the pre-drain hook covers CAPI deletions, but CA evicts a node's pods itself before it deletes the Machine.
- **Goal linkage:** this is the hosting core of the Render alternative (ADR008). It is the second half of m186: a node leaving the tenant pool, by roll or by consolidation, costs a single-instance service nothing.
- **Expected outcome:** a CA scale-down of a tenant node costs each single-instance service on it zero failed requests. The only exception is a service whose replacement cannot become Ready within CA's delay; that service gets today's eviction behavior.
- **Why now:** CA consolidation runs whenever a tenant node falls under the utilization threshold for 5 minutes (`infra/clusterapi/autoscaler-values.yaml:56-57`). This is the path that evicted `beancount-cms-v2` twice in one hour (ADR004 § Rollout headroom). m186 shipped the evacuation machinery, so this reuses it.
- **Render parity task omitted:** no REST, GraphQL, MCP or UI surface changes. This is a platform-internal mechanism.
