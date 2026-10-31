# Capacity source simplification — 2026-10-31

The three repository simplify roles reviewed the complete35-path capacity, snapshot/delete, inventory/fixture, factory and rollout change scope. Baseline manifests retain the exact23 main files plus12 supplemental paths and selected-commit diff identities. Quality and efficiency reviews found no worthwhile change; reuse found two small cleanups.

The cleanup observer now uses existing `CHANNEL_BYTES` for its unchanged2MiB output bound. Both snapshot-marker clearing sites call one helper; detach still uniquely removes the provisioned marker. Ownership fences, durable purge state, foreground deletion barriers and bounded observation are retained. These operations remain direct and use existing controller primitives.

All49 existing fixture regressions pass. Focused disk/deletion controller regressions pass. This completes the source-only t005 cleanup; it does not execute or complete the original production hard-full/grow/remount/snapshot/restore, billing, complete surface or exact infrastructure cleanup acceptance. No frozen prior receipt was edited and the verified CSI image/geometry mechanism is unchanged.
