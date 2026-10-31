# Activate bounded encrypted disks — 2026-10-30 UTC

Actual pre-activation inventory 41029256969 confirms the signed Ready operator/API on source `dc80c29bf…`, including the admitted-capacity writer, every bounded StorageClass field and both preserved original 10Gi disk bindings/handles. There are no small or unmatched claims. The production CSI derivative already passed rollout verification.

The source patch now selects `hcloud-volumes-luks-bounded` as the production manager default for every workspace's subsequent creates. The legacy class remains for existing claims. Guards require exact bounded geometry, publish/expand Secret templates and the reviewed CSI pin, while local/default renders retain their current behavior. The same patch previously passed three final simplify reviews; full GitOps validation passes again on the actual rebased tree.

This records the source activation and prerequisites. Effective class convergence, an actual owned 1Gi hard-full/5Gi online-grow/remount/restore/metrics/API/UI/requested-billing walkthrough and exact cleanup still remain. No fixture was created by this checkpoint. `receipt.json` records exact source/log/pre-activation hashes; the full log is losslessly gzip-compressed.
