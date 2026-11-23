# w6 · m559 — A disk service's volume stays on its node through a same-node deploy or restart

**Worker:** worker6 **Goal:** when a deploy, restart or plan change of a disk-bearing service starts its new instance on the node its volume is attached to, the volume never leaves that node in between. A minimal holder pod keeps it in use there through the swap, so the attach/detach controller has no moment to detach it, and the kubelet keeps it mounted for the new instance. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [The disk holder Job](done/t001.md) — **DONE** | 1h | — |
| t002 | [A disk service's `Recreate` rollout starts its holder first and ends it once the new instance has the volume](done/t002.md) — **DONE** | 2h | t001 |
| t003 | [The holder is never an instance](done/t003.md) — **DONE** | 45m | t001 |
| t004 | [Record the holder and the 2 s sync decision in ADR082, ADR004 and ADR018](done/t004.md) — **DONE** | 30m | t002 |
| t005 | [Ship and measure on production](done/t005.md) — **DONE** | 1h30m | t002, t003, t004 |
| t006 | [Simplify](done/t006.md) — **DONE** (run with the code, before t005) | 15m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** (written with the code, before t005) | 30m | t005 |
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t006, t007 |

## Definition of done

- On production, at least ten consecutive same-node deploys and restarts of a disk-bearing Starter fixture in `bex-canary` show no `DetachVolume` for its PV in the leader `kube-controller-manager` log (no `Verified volume is safe to detach` line), measured as in [m547 t005](../../blocked/m547/done/t005.md). They mix code deploys and restarts and name the port, as t005 did, and every new instance runs on the volume's node.
- In each of those runs a holder pod ran on the volume's node from before the old instance stopped until the new instance had the volume mounted, and no holder Job or pod is left once the rollout ends. The cordoned-node run makes no holder, or ends it at once, and moves the volume no slower than t005's 34.24 s plus the time to delete the holder.
- No customer surface shows the holder: the service's instances (`GET /v1/services/{id}/instances`), SSH targets, metrics and events.
- ADR082 describes the holder, its measured same-node window and the decision on the 2 s sync. ADR004 and ADR018 no longer list the same-node detach as open.

## Source + Goal linkage

- **Source:** inbox note [w6/161](../161.md), filed by w6/m547 t005 on 2026-11-21 and promoted by `/loopx w6` the same day. m547 ([blocked/m547](../../blocked/m547/README.md)) is parked until this milestone and w6/162 ship. Its t011 then re-measures the whole window.
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first)). [Render's persistent disks](https://render.com/docs/disks): "Render stops the old instance before starting the new one, which makes the service unavailable for a few seconds." One same-node swap in ten took 31.75 s instead of 7.25–9.0 s.
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

## Parked — 2026-11-21: t005 waited for the production pin (cleared 2026-11-22)

- **Shipped (`dd8dfaaf00`):**
  - a disk-bearing App's `Recreate` rollout holds its volume on its node with the holder Job `dskhld-<app>`. The template is written once the holder's pod has the volume mounted, and the holder ends once the new instance has mounted it there, or at once when the new instance lands elsewhere. Every failure to hold rolls as before (t001, t002);
  - the operator's stall and node-loss diagnostics, bex-api's store-less log reads and the metrics-server fallback no longer read the holder as an instance (t003);
  - ADR082 § D3, ADR004 and ADR018 record the holder and the 2 s sync decision (t004);
  - Simplify and Test coverage ran with the code (t006, t007);
  - [w6/165](../../165.md) files the one swap the holder does not cover, the restore of a served release after a failed rollout.
- **Gate (cleared 2026-11-22): the pin carried `dd8dfaaf00`.** Then t005 was to run ten or more same-node deploys and restarts and a cordoned run of a disk-bearing Starter fixture in `bex-canary` on a `bex-tenant-0` node, as [m547 t005](../../blocked/m547/done/t005.md) did. It checks that no `DetachVolume` appears and that no holder is left, records the phase table and the bound in ADR082, and deletes the fixture by id. t008 then closes, and m547's t011 re-measures.

## Gate cleared, and t005 found that no rollout held — 2026-11-22

- **Unblock record (verified evidence, `/loopx w6`; affects t005 and t008).** The newest pin, `19fdb46f1758` (`5330be9f54`), carries `dd8dfaaf00` (`git merge-base --is-ancestor dd8dfaaf00 19fdb46f1758`). `bex-controller-manager` and `bex-api` ran that pin's digest, `sha256:7a7051cf219c`. The gate held, so t005 ran. Nothing the user decides is involved.
- **Finding.** On that pin every holder timed out. Its 10m CPU limit is the quota runc starts the pod's sandbox and container under, and the pod took 24.7 s to start, past the 15 s mount bound. Both runs, a code deploy and a restart, therefore went on without a hold (`fallback_timeout`). Three pause pods of its shape started in 25.4 s at 10m, 2.4 s at 100m and 1.4 s at 500m. The record is in [t005](done/t005.md).
- **Fix (`5ada5fbcdc`).** The holder's CPU limit is 500m, a Starter instance's CPU. `TestDiskHolderStartsWellWithinItsMountBound` fails at 10m. ADR082 § D3 records it, and t006 and t007 ran again over it.
- **Render parity is unchanged.** Render stops the old instance before it starts the new one. The holder only keeps bex's same-node swap inside the few seconds that costs.

## BLOCKED — 2026-11-22: t005 waits for the pin that carries the CPU fix

- **Gate (clears by itself): pin must carry `5ada5fbcdc`.** Then t005 runs in full on a fresh fixture in `bex-canary` on a `bex-tenant-0` node: ten or more same-node deploys and restarts, then a cordoned run. Each same-node run should count `bex_disk_holds_total{result="held"}` and none `fallback_timeout`, the holder should start before the old instance's `SIGTERM`, and no `DetachVolume` should appear. t005 records the phase table and the bound in ADR082 and deletes the fixture by id. t008 then closes, and m547's t011 re-measures.

## Gate cleared — 2026-11-23: the pin carries the CPU fix

- **Unblock record (verified evidence, `/loopx w6`; affects t005 and t008).** The newest pin, `1a5a8e872398` (`e6c65f5c62`), carries `5ada5fbcdc` (`git merge-base --is-ancestor 5ada5fbcdc 1a5a8e872398`). `bex-controller-manager` (started 04:29:43Z) and both `bex-api` replicas run its digest, `sha256:65ef4f0d90b1`. Nothing the user decides is involved.
- **Next.** t005 runs in full: a fresh disk-bearing Starter fixture in `bex-canary` on a `bex-tenant-0` node, ten or more same-node deploys and restarts, then a cordoned run and the cleanup checks. Each same-node run should count `bex_disk_holds_total{result="held"}`, and none `fallback_timeout`. t008 then closes, and m547's t011 re-measures.

## Done — 2026-11-23: every same-node rollout held, and none detached

- **t005 ([record](done/t005.md)).** On pin `1a5a8e872398`, a fresh 1 GB Starter fixture in `bex-canary` on `bex-tenant-0-zdp6v-6p4cg` took six code deploys and six restarts in a row. All twelve counted `bex_disk_holds_total{result="held"}` and `bex_disk_hold_ends_total{reason="mounted"}`, and none `fallback_timeout`.
  - The leader `kube-controller-manager` logged no `DetachVolume` for the PV, and its `VolumeAttachment` never changed.
  - Each holder pod ran on that node. Its sandbox returned 0.58–1.31 s before the old instance's SIGTERM, and it ended 1.5–1.7 s after the new instance's sandbox returned.
  - No new instance did a `MountDevice`, so the LUKS open and the sync gate are gone. The window was 4.0–4.25 s for restarts and code deploys alike, against m547 t005's 5.74–6.25 s and 7.25–9.0 s.
  - The cordoned run made no holder (`skipped_unschedulable`) and moved in 33.25 s, within 34.24 s.
  - No `dskhld-` Job or pod was left. The instances list, the events and the instance-count series never showed a holder.
  - The fixture was deleted by id, with its disk, PV and attachment.
- **The 2 s sync stays.** The run showed no cost from it.
- **Not covered here.** [w6/165](../../165.md), the failed-rollout restore, stays open. m547's t011 re-measures the whole window.
