# Render disk contract recheck — 2026-10-30

Seven current primary pages returned200; retrieval timestamps, URLs and original byte hashes are retained. Raw pages remain private. This is a documentation comparison, with no Render paid-resource creation or live Bex lifecycle claim.

| Contract | Primary source | Owned Bex acceptance |
| --- | --- | --- |
| Paid-service disk data survives restarts/deploys. Size increases online without downtime and cannot decrease. Single-instance attachment prevents overlapping disk-backed deploys. Daily snapshots retain seven days; restore replaces the disk state. | [Persistent disks](https://render.com/docs/disks) | Preserve data hashes, process identity during1→5 growth, then verify remount/redeploy and selected snapshot restoration. |
| PATCH updates an active disk and requires a larger resize;200 returns the disk including createdAt/updatedAt. | [Update disk](https://api-docs.render.com/reference/update-disk) | One recorded grow intent/response; bind weighted requested-size periods to server timestamps. |
| Snapshot listing returns201. Restore accepts snapshotKey/instanceId and returns200; keys expire after24 hours. | [List](https://api-docs.render.com/reference/list-snapshots), [restore](https://api-docs.render.com/reference/restore-snapshot) | Read only the fresh owned disk, use its actual key, and check restored markers/capacity. |
| Resource-filtered capacity/usage GETs return200 time series. Streaming storage capacity and occupied usage are measured in bytes. | [Capacity](https://api-docs.render.com/reference/get-disk-capacity), [usage](https://api-docs.render.com/reference/get-disk-usage), [metric units](https://render.com/docs/metrics-streams-reference) | Compare own kubelet bytes with coherent REST/GQL/MCP/dashboard readings; keep requested capacity separate from filesystem metadata overhead. |

The reviewed pages do not specify an exact ENOSPC byte boundary or name that errno. The1Gi hard-full/5Gi raw-filesystem checks remain the explicit m144 product acceptance requirement; no direct Render full-disk experiment is claimed. Financial sealing/invoices and actual Bex surface behavior remain unverified.
