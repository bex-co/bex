# Existing service disks: inventory and preservation plan

[Read-only production inventory 40991781847](https://github.com/bex-co/bex/actions/runs/40991781847) succeeded on source `84dba2bab1527dc3c5c3900514c92e4e0750b8d4`, updated 2026-10-30 03:42:07 UTC. [Selected metadata](evidence/production-inventory-20261029/inventory.json) contains exactly two App disks, both in `tea-d98210cbbpdc73dcrkvg`. Every claim is Bound, its controller owner matches the exact App UID, and its PV claimRef matches the exact PVC UID. There are no unmatched App-owned claims.

| Existing service | Requested / allocated / observed / reported GB | PVC / PV | Class | Provider handle |
| --- | --- | --- | --- | --- |
| `srv-d9nqg9dcavls73fp8m2g` — beancount-forum | 10 / 10 / 10 / 10 | 10Gi / 10Gi | `hcloud-volumes-luks` | `107293514` |
| `srv-dbg4lfdn83uc738uug70` — crypto-price-clickhouse | 10 / 10 / 10 / 10 | 10Gi / 10Gi | `hcloud-volumes-luks` | `107282157` |

There are **no existing below-provider-floor claims** in this inventory. Preserve these two legacy numeric handles, claim/class bindings, LUKS keys, provider labels and existing contents. They request the provider minimum already; a destructive offline shrink, immutable-handle rewrite, retained-PV rebind or volume replacement is not required to correct a 1–9Gi over-allocation for either target. The reviewed derivative keeps the existing legacy-ID/class path, including its normal grow/remount/backup/restore behavior. Do not opt these claims into the new mode or inspect tenant files as part of the owned-fixture walkthrough.

This is a bounded preservation decision from the observed metadata, not a claim that a foreign disk's raw filesystem, data hashes, encryption state or restore behavior was inspected. No existing disk was changed. Metadata alone does not close t003 or m144.

The rollout sequence remains:

1. Complete the reviewed controller/node CSI upgrade and require every old driver pod to terminate. Verify the live operator image contains the admitted-PVC high-water writer. Root owns the all-workspace `BEX_DISK_STORAGE_CLASS=hcloud-volumes-luks-bounded` GitOps activation; no workspace allowlist is introduced.
2. Repeat read-only inventory after the operator/class change converges, preserving both listed App/PVC/PV/handle identities. This catches a new legacy small claim created between the first inventory and activation. If any appears, retain its exact identity and data; make a concrete migration plan for that actual target before claiming existing-disk acceptance. Never relabel/rebind/shrink an unknown or foreign volume to force completion.
3. Create the new dated, guarded QA fixture only after those readiness checks. Its own exact class/handle and App/PVC/PV lifetimes must bind in the committed proof before fixed node inspection or an own-CronJob backup can run. The historical deleted QA fixtures are not migration targets and grant no authority over these two live services.
4. Complete the original real 1Gi full, online 1→5Gi grow, hash retention, remount, snapshot/restore, kubelet metrics, requested-size billing, every REST/GraphQL/MCP/Blueprint/dashboard surface and owned cleanup checks. Compare the preservation inventory again at closeout. Record any case not actually verified.

The same inventory observes operator/API image `ghcr.io/bex-co/bex-operator@sha256:2e9286ad4080f0ff1f607fd1e77515556c9e8f04b60c37956caf4e1d4a964538`: operator 1/1 and API 2/2 updated/Ready/available, with matching observed generations and running image IDs. Root's [strict workflow signature/source receipt](evidence/production-inventory-20261029/receipt.json) maps it to `2ac6fd95402e940ce906873e9c9dc77c1ce53077`, which predates the admitted-capacity writer in `902630131`. **Class activation must wait for a newer verified operator image and its actual adoption.** CSI readiness and old operator readiness cannot substitute for that condition.


## Repeat inventory — 2026-10-30 06:14:59 UTC

[Run 41009352173](evidence/production-inventory-20261029/receipt-41009352173.json) again preserves both exact App/PVC/PV lifetimes and provider handles, with zero small or unmatched claims. The bounded StorageClass is now directly observed with UID `45392ab9-1e77-477d-9a95-6a9a84670869` and the complete reviewed contract. The effective class and signed operator remain the legacy class and old `2ac6fd…` source. No disk/class changed; repeat the preservation inventory after actual activation before treating the existing-disk prerequisite as complete.
