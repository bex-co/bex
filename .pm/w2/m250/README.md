# w2 · m250 — Pro Plus, Pro Max and Pro Ultra services start, as on Render

**Worker:** worker2 **Goal:** a customer on any workspace plan who picks Pro Plus, Pro Max or Pro Ultra gets a running service, because autoscaled serving pools with nodes large enough for those tiers exist, and only those tiers use them. **Status:** todo — t001, t002 done (`1f42f9bcf`); t003 blocked on Hetzner (cx53 stock; dedicated-core limit); t009 (pause Pro Ultra) in progress

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Two serving pools in the CAPI overlay (cx53 min 0/max 4, ccx43 min 0/max 2), each with its own taint, plus validator guards — **DONE** | 1h  | —          |
| t002 | Operator: App pods of the large tiers tolerate and prefer their pool (from `tiers.yaml`); admission and build-only rules still hold — **DONE** | 1h  | —          |
| t003 | Production: pro-plus and pro-ultra services go live on new nodes and the pools scale back to 0 | 45m | t001, t002 |
| t009 | Pause Pro Ultra at create and plan change until the Hetzner dedicated-core limit is raised | 1h | — |
| t004 | Docs: ADR053/ADR060 pools, ADR018 instance-type row                                            | 20m | t003       |
| t005 | Render parity                                                                                  | 15m | t004       |
| t006 | Simplify                                                                                       | 15m | t005       |
| t007 | Test coverage                                                                                  | 30m | t005       |
| t008 | Closeout                                                                                       | 10m | t006, t007 |

## Definition of done

- On production, a `pro_plus` web service reaches `live` on a node of the cx53 pool and a `pro_ultra` one on a node of the ccx43 pool, created by the autoscaler from zero; after both are deleted, both pools return to 0 nodes.
- `starter`…`pro` services, builds and datastores never schedule on either pool (taints), and the build-only pool is unchanged.
- `scripts/clusterapi-validate.sh` fails if a large tier no longer fits its pool's allocatable, or if a pool loses its taint or min 0.
- The Scale workspace whose pro-plus/pro-ultra services failed (`tea-db9f0dkfcf2c73c2mq4g`) can redeploy them successfully, or its owner is told the types now work.

## Source + Goal linkage

- **Source:** promoted from inbox note [w2/063](source-w2-063.md) (found in w2/061's production check, 2026-11-01); user decisions "A" (2026-11-01) and "Two pools" (2026-11-02).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) Render parity first: Render sells and starts these instance types on every plan.
- **Expected outcome:** every compute tier bex sells actually runs; nodes for them cost money only while such a service exists.
- **Why now:** a Scale customer's pro-plus and pro-ultra services already fail, and w2/061 just let Hobby workspaces pick these tiers too.
- **Render parity included:** the instance types on REST, GraphQL, MCP, CLI and the dashboard go from "never starts" to "starts".

## t001 + t002 — 2026-11-02 (`1f42f9bcf`)

- **Pools:** `bex-tenant-xl` (cx53, min 0 / max 4) and `bex-tenant-xxl` (ccx43, min 0 / max 2), each mirroring `bex-tenant-lg` (MachineDeployment, HCloudMachineTemplate, MachineHealthCheck, autoscaler min/max/cpu/memory/disk/label/taint hints) with bootstrap templates `bex-tenant-xl`/`-xxl` that equal `bex-tenant-0` except for the taint and node labels. Both use a new spread placement group `tenant-large` (declared on the HetznerCluster), because Hetzner caps one at 10 servers and tenant-0's pools can already reach 11. The autoscaler discovers MachineDeployments by cluster name, so no allowlist change.
- **Placement:** taint and label `bex.co/serving-pool=xl|xxl` (nodes keep `bex.co/pool=tenant`). `tiers.yaml` names the pool (`servingPool`) for pro-plus/pro-max (xl) and pro-ultra (xxl). The operator gives those tiers' serving Deployment, cron/one-off runs and pre-deploy Jobs the toleration plus a required node affinity; disk snapshot Jobs get the toleration only. Other tiers keep no toleration/affinity, so no existing service rolls.
- **Admission:** the serving-pool toleration is allowed only on App-owned workloads with a catalog-pinned value; the build-only toleration is now confined to build Jobs (it was not restricted before). Egress meter, node exporter and drain guard cover the new nodes (missing-meter alert test added).
- **Validator:** `clusterapi-validate.sh` fails when a tier no longer fits its pool or `bex-tenant-0`, a pool loses its taint/label/min 0/max/placement group, or a bootstrap template drifts from `bex-tenant-0`; each case proven red. `make test`, `make lint` (all modules), both validators and their self-tests green.
- **Open for t003:** cx53/ccx43 allocatable values in the validator are estimates; cx53 failed to create in fsn1 once (2026-08-11), so the first scale-up is the real test (fall back to an equivalent type if Hetzner has no stock).

## t003 attempt 2026-11-02 — placement works; Hetzner refuses both node types

- Pools live on production: `bex-tenant-xl` 0/0 (min 0, max 4) and `bex-tenant-xxl` 0/0 (min 0, max 2), `app-cluster.yml` green for `0acc15714`; operator pinned `0acc15714458`.
- A `pro_plus` and a `pro_ultra` image service in Hobby `tian-personal` (02:29:51Z) made the autoscaler scale **each pool from 0 to 1** (routing, taints and the scale-from-zero hints work). Both Machines then failed at Hetzner:
  - `bex-tenant-xl` (cx53, fsn1): `error during placement (resource_unavailable)`, Hetzner has no cx53 stock in fsn1 (as on 2026-08-11).
  - `bex-tenant-xxl` (ccx43, fsn1): `dedicated core limit exceeded (resource_limit_exceeded)`, the Hetzner project's dedicated vCPU limit is below 16.
- Stock probe (Hetzner API, servers created stopped and deleted at once, none left): `cpx62` (16 shared vCPU / 32 GB, €152.99) **available** in fsn1; `cx53` `resource_unavailable`; `cpx51` not offered there.
- Both test services were deleted; the failed Machines are reclaimed by the autoscaler.

## Decisions 2026-11-02 (user, in this session)

- **xl node type:** "Wait for cx53 stock". Keep cx53 (€35); do not switch to cpx62 (€153). Until Hetzner restocks fsn1, Pro Plus and Pro Max services stay Pending with the stall reason "no node currently has room for this service's instance…". **Gate:** cx53 stock in fsn1. The worker re-probes (create stopped, delete at once) and, when it succeeds, re-runs t003 for pro-plus.
- **Pro Ultra:** "Pause Pro Ultra for now". Refuse `pro_ultra` at create and plan change with a clear message (`INSTANCE_TYPE_UNAVAILABLE`), show it disabled in the dashboard picker, and record a temporary ADR018 divergence with the user's sign-off (new task t009). **Release condition:** the Hetzner dedicated vCPU limit is raised to ≥32 (account owner, Hetzner Console → Limits or a support ticket); then remove the pause and re-run t003 for pro-ultra.
