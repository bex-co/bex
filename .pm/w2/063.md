# 063 — Pro Plus, Pro Max and Pro Ultra services can never start: no serving node is large enough

Why: bex sells instance types up to Pro Ultra (as Render does), but a customer who picks Pro Plus or larger gets a deploy that waits and then fails, in every workspace plan, because no node in the serving pool can hold the instance.

- **Found:** w2/061 production check, 2026-11-01. A `pro_plus` image service in Hobby `tian-personal` (quota now allowing it) stayed `Pending`: `0/13 nodes are available: … Insufficient cpu, Insufficient memory, … untolerated taint(s)`; the autoscaler: `pod didn't trigger scale-up: … max node group size reached`. The customer sees the stall reason "no node currently has room for this service's instance …".
- **Already hitting a customer:** Scale workspace `tea-db9f0dkfcf2c73c2mq4g` has `base-notebook2` (pro-plus) `Failed`, `nginxhdjdjs` (pro-ultra) `Failed` and `nginxtyey` (pro-ultra) `Canceled`. No 4- or 8-CPU tenant pod is running anywhere on production.
- **Why:** `lego/types/tiers/tiers.yaml` pro-plus = 4 CPU / 8Gi, pro-max = 4 CPU / 16Gi, pro-ultra = 8 CPU / 32Gi, all Guaranteed. Every serving node (`bex-tenant-0`, autoscaler max 8) is a Hetzner cx33 with 4 CPU / ~7.47Gi allocatable after system reservations, so even an empty node cannot hold pro-plus. The only larger nodes are `bex-tenant-lg` (one cx43, 8 CPU / ~15Gi) and `bex-tenant-burst`, both tainted `bex.co/build-only`.
- **Render:** sells Pro Plus (4 CPU / 8 GB), Pro Max (4 CPU / 16 GB) and Pro Ultra (8 CPU / 32 GB) on every workspace plan, and they start.
- **Decision (user, infrastructure spend):**
  - **A. Add a large serving node pool (recommended, matches Render):** a new autoscaled MachineDeployment for serving (for example Hetzner cx53 or ccx33, 8+ CPU / 32 GB, min 0), tainted so that only pods of tiers that do not fit `bex-tenant-0` tolerate it. Nodes exist, and are billed, only while such a service runs. The worker writes the CAPI overlay, operator toleration/affinity per tier and the validator guard; `app-cluster.yml` applies it.
  - **B. Stop selling the types that do not fit:** hide or refuse pro-plus/pro-max/pro-ultra until A ships, with a clear refusal at create. Customers can no longer pick them (a divergence from Render, recorded in ADR018 with your sign-off).
- **Release condition:** the user chooses A (and the node type / max size) or B. Either way, tell the affected Scale workspace's owner, or let their services start once A is live.

## Unblock 2026-11-02 — user chose A with two autoscaled pools; resumed

- **Affected:** `w2/063`. **Source:** explicit user decisions in `/unblock w2`: "A (recommended, matches Render)" on 2026-11-01, then "Two pools" on 2026-11-02. Hetzner server types and prices were read from the Hetzner API (`/server_types`, read-only) the same day.
- **Decision:**
  - **`bex-tenant-xl`:** Hetzner `cx53` (16 shared vCPU / 32 GB, €34.99/mo), autoscaled **min 0 / max 4**, for `pro-plus` (4 CPU / 8Gi) and `pro-max` (4 CPU / 16Gi).
  - **`bex-tenant-xxl`:** Hetzner `ccx43` (16 dedicated vCPU / 64 GB, €325.49/mo), autoscaled **min 0 / max 2**, only for `pro-ultra` (8 CPU / 32Gi), the one tier that no 32 GB node can hold after system reservations.
  - Each pool carries its own `NoSchedule` taint; only App pods of the matching tiers tolerate it (with node affinity to the pool), so small services never land there and the nodes exist, and are billed, only while such a service runs. Builds and datastores are unaffected.
  - Not adopted: one ccx43 pool for all three (a Pro Plus customer would cost €325/mo), and cx53 only with Pro Ultra refused (a Render divergence).
- **Render basis:** Render sells Pro Plus, Pro Max and Pro Ultra on every workspace plan, and they start.
- **Remaining gates:** none.
- **Worker next steps:** add both MachineDeployments (with HetznerMachineTemplates, MachineHealthChecks and autoscaler min/max/capacity annotations, mirroring `bex-tenant-lg` but with serving taints) in `infra/clusterapi/overlays/hetzner-caph/cluster.yaml`; guard them in `scripts/clusterapi-validate.sh`; have the operator add the toleration and node affinity per tier (from `tiers.yaml`, not hard-coded names) in the App Deployment; make sure admission and the build-only rules still hold; document in ADR053/ADR060 and ADR018. `app-cluster.yml` applies the overlay on merge. **Acceptance on production:** a `pro_plus` service goes `live` on a new `bex-tenant-xl` node and a `pro_ultra` service goes `live` on a new `bex-tenant-xxl` node, both scale back to 0 after the services are deleted, and small services never schedule there. Then tell the owner of Scale workspace `tea-db9f0dkfcf2c73c2mq4g` (whose pro-plus/pro-ultra services failed), or confirm their redeploy now starts.
