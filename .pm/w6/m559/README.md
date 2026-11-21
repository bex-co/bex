# w6 · m559 — A disk service's volume stays on its node through a same-node deploy or restart

**Worker:** worker6 **Goal:** when a deploy, restart or plan change of a disk-bearing service starts its new instance on the node its volume is attached to, the volume never leaves that node in between. A minimal holder pod keeps it in use there through the swap, so the attach/detach controller has no moment to detach it, and the kubelet keeps it mounted for the new instance. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [The disk holder Job](t001.md) | 1h | — |
| t002 | [A disk service's `Recreate` rollout starts its holder first and ends it once the new instance has the volume](t002.md) | 2h | t001 |
| t003 | [The holder is never an instance](t003.md) | 45m | t001 |
| t004 | [Record the holder and the 2 s sync decision in ADR082, ADR004 and ADR018](t004.md) | 30m | t002 |
| t005 | [Ship and measure on production](t005.md) | 1h30m | t002, t003, t004 |
| t006 | [Simplify](t006.md) | 15m | t005 |
| t007 | [Test coverage](t007.md) | 30m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007 |

## Definition of done

- On production, at least ten consecutive same-node deploys and restarts of a disk-bearing Starter fixture in `bex-canary` show no `DetachVolume` for its PV in the leader `kube-controller-manager` log (no `Verified volume is safe to detach` line), measured as in [m547 t005](../blocked/m547/done/t005.md). They mix code deploys and restarts and name the port, as t005 did, and every new instance runs on the volume's node.
- In each of those runs a holder pod ran on the volume's node from before the old instance stopped until the new instance had the volume mounted, and no holder Job or pod is left once the rollout ends. The cordoned-node run makes no holder, or ends it at once, and moves the volume no slower than t005's 34.24 s plus the time to delete the holder.
- No customer surface shows the holder: the service's instances (`GET /v1/services/{id}/instances`), SSH targets, metrics and events.
- ADR082 describes the holder, its measured same-node window and the decision on the 2 s sync. ADR004 and ADR018 no longer list the same-node detach as open.

## Source + Goal linkage

- **Source:** inbox note [w6/161](../done/161.md), filed by w6/m547 t005 on 2026-11-21 and promoted by `/loopx w6` the same day. m547 ([blocked/m547](../blocked/m547/README.md)) is parked until this milestone and w6/162 ship. Its t011 then re-measures the whole window.
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first)). [Render's persistent disks](https://render.com/docs/disks): "Render stops the old instance before starting the new one, which makes the service unavailable for a few seconds." One same-node swap in ten took 31.75 s instead of 7.25–9.0 s.
- **Expected outcome:** a same-node deploy or restart never pays Hetzner's detach (about 9.4 s) and attach (15–16 s). The new instance also skips the sync wait (0.03–1.96 s in t005), because the volume stays reported in use. It may skip the LUKS open (about 2.2 s) too, because the holder keeps the mapping open. t005 measures that.
- **Why now:** it gates m547, and each disk service's deploys, restarts and plan changes hit the race at random. m547 t010's 2 s sync made it likelier, so this milestone also settles whether 2 s stays.
- **Render parity task omitted:** operator mechanism only. No REST, GraphQL, MCP or dashboard field changes; t003 checks that the holder never appears on any of them.

## Direction chosen, and why the other was rejected (triage, 2026-11-21)

The note offered two options. This milestone takes the first.

- **Mechanism (m547 t005, run D1).** The old pod turned `Failed` at 11:23:20.21. The kubelet tore down its mount and marked the volume detached in its own state at 20.35. The next 2 s node-status sync dropped the volume from `volumesInUse`. From the moment the pod turned `Failed`, no scheduled pod used the volume, so the attach/detach controller's desired state no longer held it. At 20.624 the controller logged `Verified volume is safe to detach`, and the new pod was bound to the same node 50 ms later. The race window runs from the kubelet's teardown to the new pod's binding (0.3–0.6 s, t001). The controller detaches only when both its desired state lacks the volume and the node no longer reports it in use, so any pod on that node that mounts the claim through the gap closes the window.
- **1. A holder pod on the volume's node: chosen.**
  - While the holder runs, the controller's desired state keeps the volume on that node, and the kubelet's own desired state keeps it mounted and reported in use. The new pod joins a volume the kubelet already holds. Its mount is then a publish only: it waits for no sync, and it does not stage the device again.
  - The volume is `ReadWriteOnce` (`disk.go:460`), so a second pod may mount it on the same node. The nightly backup already does this beside a running service. The claim source stays read-write and only the container's mount is read-only, because a read-only source makes the LUKS publish fail with `EBUSY` (`disk_backup.go:326`, w2/m86). A second read-write publish of the open mapping shares its superblock. So the LUKS open is likely spared too. t005 checks it.
  - The operator cannot author a bare pod. Its RBAC grants pods only `get/list/patch/watch`, and `operator-workload-admission.yaml` refuses `nodeName`. It may author an App-owned Job, which may tolerate the App's serving pool (`appOwnedWorkload`). The holder is therefore a Job whose pod requires the volume's node by node affinity.
- **2. Return to the 10 s sync: rejected.** It lowers the odds but does not close the race. A sync lands inside a 0.3–0.6 s gap on about 15–30 % of swaps at 2 s, and on about 3–6 % at 10 s. It would also bring back the mount wait t010 removed, 1.7–8.6 s on every same-node swap, and up to 8.1 s before a move's detach.
- **The 2 s sync stays.** With the holder, a same-node swap no longer depends on the sync period, and 2 s still shortens a move: the old node reported a leaving volume free in 1.3 s in t005, against 1.0–8.1 s at 10 s in t001. t004 records this in ADR082. t005 confirms it, unless the measurement shows a cost from the 2 s sync.
