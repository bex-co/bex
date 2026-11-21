# w6 · m557 — A disk grow reaches the instance within seconds, as on Render

**Worker:** worker6 **Goal:** as Render documents ("Increasing size causes no downtime, and the added storage is available within a few seconds"), a larger `sizeGB` on a running service's disk shows in `df` inside the instance within seconds on every surface, and nothing says it waits for a restart. **Status:** todo (t001, t002 and t004–t006 done; t003 waits for production to carry t001)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [The operator starts the kubelet's pod sync when a disk claim waits for the node, and DiskReady follows the claim](done/t001.md) — **DONE** | 75m | — |
| t002 | [The dashboard's grow hint says what Render says](done/t002.md) — **DONE** | 20m | — |
| t003 | [Production: three grows reach `df` within seconds, then the gap leaves ADR018 and ADR082](t003.md) | 45m | t001, t002 |
| t004 | [Render parity](done/t004.md) — **DONE** | 20m | t001, t002 |
| t005 | [Simplify](done/t005.md) — **DONE** | 15m | t004 |
| t006 | [Test coverage](done/t006.md) — **DONE** | 30m | t004 |
| t007 | [Closeout](t007.md) | 10m | t003, t005, t006 |

## Definition of done

- On production, three grows of a running paid fixture's disk in `bex-canary` through `PATCH /v1/disks/{id}` (10→11, 11→12, 12→13 GB) each show the new size in `df` inside the instance, read every second, within 15 s of the request, with the same pod (same UID, 0 restarts) throughout. The fixture is deleted by id with its PV and Hetzner volume.
- The App's `DiskReady` is True within 5 s of the claim's capacity reaching the request, and no condition message says the grow finishes on a restart unless the node failed to expand it.
- The dashboard's grow hint carries Render's sentence in all eight locales, with no restart caveat.
- ADR018's Persistent disks row and ADR082 § D4 no longer record a growth gap; `DiskResizePending` stays documented as the fallback for a node that cannot expand online.

## Source + Goal linkage

- **Source:** [w6/157](../done/157.md), measured on production 2026-11-21. Four grows of one 10 GB LUKS disk (`hcloud-volumes-luks-bounded`) on a Starter worker were all online with one pod, but `df` showed them 7.5, 46.5 and 63.7 s after the request. Each time the controller step (Hetzner volume, PV) took 6–10 s, and the claim then sat in `NodeResizePending` until the kubelet's next sync of the pod: tenant kubelets run the default `syncFrequency: 1m0s` (read from the node's `configz`), and the kubelet's volume manager reprocesses a pod's volumes, and so runs `NodeExpandVolume` for a mounted one, only when it syncs that pod. The fourth grow annotated the pod as the claim reached `NodeResizePending`, and the filesystem grew 0.35 s later (14.5 s after the request, most of it the controller step).
- **Code today:** `growDiskPVC` (`lego/operator/internal/controller/disk.go:376`) patches the claim and sets `DiskResizePending` with "finishes growing on the next restart" (`:409`); the App controller watches Deployments and Pods but not claims (`app_controller.go:8366-8371`), so `DiskReady` came back up to 30 s after the filesystem had. It already may patch pods (`app_controller.go:566`). The dashboard's `services.diskGrowHint` (`dashboard/src/features/services/locales/*.ts`, shown by `disk-section.tsx:531`) says "on some volumes the filesystem finishes growing on the next restart".
- **Goal linkage:** ADR008 § Render parity first; the ADR018 Persistent disks row has no user sign-off for the gap.
- **Expected outcome:** a customer who grows a disk because it is full gets the space in seconds, the way Render's docs promise, without a restart or a minute's wait.
- **Why now:** w6/157 found the gap while removing the divergence text; it is the last disk-resize difference ADR082 records.
- **Rejected alternative:** lowering tenant kubelets' `syncFrequency` to a few seconds would resync every pod on dense tenant nodes that often, and rolls every tenant node, to speed up a rare event; nudging the one pod costs one patch per grow.
- **Render parity task included:** the dashboard copy changes, and REST, GraphQL, MCP and Blueprint sync all reach this operator path.
