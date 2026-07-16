# Render Postgres disk autoscaling capture

Captured 2026-07-15 from Render's public Postgres documentation and API field names. This record pins the constants used by bex's operator control loop.

## Render contract

| Concern | Render behavior | Evidence |
| --- | --- | --- |
| Trigger | Resize when storage is 90% full. | [Create and Connect to Render Postgres](https://render.com/docs/postgresql-creating-connecting#storage-autoscaling) |
| Step | Increase the current size by 50%. | Same documentation section. |
| Rounding | Round the result up to the nearest 5 GB. Render's examples are 1→5 GB, 10→15 GB, and 25→40 GB. | Same documentation section. |
| Cap | 16 TB (16,384 GB) through the normal product/API path. Larger volumes require contacting Render support. | [Adding storage](https://render.com/docs/postgresql-creating-connecting#adding-storage) |
| Cooldown | No second storage increase for 12 hours after an increase. | Same adding-storage section. |
| Shrink | Never. Storage increases are permanent. | Same adding-storage section. |
| Write field | `enableDiskAutoscaling` on create and update input. | The documentation names the update field; Render's pinned OpenAPI names it on `postgresPOSTInput` and `postgresPATCHInput`. |
| Read field | `diskAutoscalingEnabled` on the Postgres object. | Render's pinned OpenAPI `postgres` schema. |
| Notification | The public storage-autoscaling documentation does not promise an email, webhook, or activity-feed event for an automatic resize. | No notification behavior is stated in the cited flow. |

The threshold comparison is inclusive (`used / capacity >= 0.90`). The step is computed from the currently provisioned size, not the number of used bytes:

```text
nextGB = min(16384, ceil((currentGB * 1.5) / 5) * 5)
```

At the cap the loop holds steady. Missing or invalid usage data also holds steady; it never guesses.

## bex implementation and explicit divergences

- The operator reads the existing `kubelet_volume_stats_used_bytes` and `kubelet_volume_stats_capacity_bytes` series from Prometheus. This works with or without `BEX_CP_DB_URI`: Prometheus is cluster telemetry, not the optional control-plane store. For an HA database, the fullest CNPG instance PVC drives the decision because every replica has its own full copy.
- Each automatic resize emits a Kubernetes `Normal` Event with reason `DiskAutoscaled` and appends a bounded entry to `Database.status.diskResizeHistory`. Render does not publish a notification contract; bex deliberately supplies an auditable operator/status trail instead of resizing silently.
- The last resize timestamp is persisted with the resize intent, so a stale kubelet sample or slow CNPG PVC expansion cannot cause another growth during the 12-hour cooldown.
- CNPG remains the executor: the operator changes `Database.spec.storageGB`, projects the larger `Cluster.spec.storage.size`, and CNPG expands the PVC.

## Live mock-cluster verification

Verified 2026-07-15 on the CAPD mock with a real CNPG 18 database and the operator image built from this change. The mock's default `rancher.io/local-path` provisioner neither exports kubelet volume statistics nor supports expansion, so the drill temporarily substituted the upstream CSI hostpath test driver behind the same `hcloud-volumes` StorageClass and installed the repository's production kubelet Prometheus scrape. All temporary resources were removed afterward and the original StorageClass was restored.

The test database started with a 1 GiB PVC. Its filled filesystem produced a live Prometheus sample of 584,032,485,376 used bytes out of 590,705,065,984 capacity bytes (98.87%). On the next reconcile:

- `Database.spec.storageGB` grew from 1 to 5 and `Database.status.diskResizeHistory` recorded the timestamp, old/new sizes, and exact triggering sample.
- CNPG projected `Cluster.spec.storage.size=5Gi`; the CSI resizer moved the PVC request and capacity from `1Gi/1Gi` to `5Gi/5Gi`, finishing with no resize conditions after the required remount.
- A second controlled resize check produced the Kubernetes Event `DiskAutoscaled` with message `grew Postgres storage from 5 GB to 10 GB ...` and the PVC converged to `10Gi/10Gi`. This check exposed and fixed the missing Event RBAC grant (`create;patch;update` on core Events).
