# w2 · m247 — High-availability Postgres fits in any workspace, as on Render

**Worker:** worker2 **Goal:** a Hobby workspace can create and run any sellable Postgres (including a `2c-4g` HA pair) and Key Value, because datastore Pods are not charged against the services' compute quota, and an instance's quota charge matches its plan. **Status:** done — every task done; production Hobby HA pairs verified 2026-10-31

## Tasks (in order)

| id   | title                                                                                              | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Datastore Pods carry a dedicated PriorityClass, and the compute quota excludes it (new + existing namespaces) — **DONE** | 1h  | —          |
| t002 | Bounded barman-cloud sidecar resources, so a 1-CPU instance is not charged 3 CPU of limits        — **MOVED to `w2/055`** (not needed for the DoD after t001; its rollout restarts every backed-up tenant Postgres) | 30m | —          |
| t003 | Prove on the local cluster that a Hobby-quota namespace runs a `2c-4g` HA pair, and that services are still capped — **DONE** | 40m | t001, t002 |
| t004 | Production: a Hobby workspace runs `1c-2g` and `2c-4g` HA databases with two healthy instances (then delete) — **DONE** | 30m | t003       |
| t005 | Correct ADR043/ADR009/ADR018 and the `quotaForPlan` rationale — **DONE**                                      | 20m | t004       |
| t006 | Render parity — **DONE**                                                                                      | 15m | t005       |
| t007 | Simplify — **DONE**                                                                                           | 15m | t006       |
| t008 | Test coverage — **DONE**                                                                                      | 30m | t006       |
| t009 | Closeout — **DONE**                                                                                           | 10m | t007, t008 |

## Definition of done

- In a production Hobby workspace that already runs services, `POST /v1/postgres {"plan":"2c-4g","enableHighAvailability":true}` reaches two healthy instances on distinct nodes, and so does `1c-2g`. No `exceeded quota: tenant-quota` appears.
- Services in a Hobby workspace are still bounded by the compute quota (a deploy past it is still refused), while datastores are bounded by their count caps (`QuotaCapsForPlan`: 25 Postgres / 25 Key Value) and per-instance billing.
- A Postgres instance Pod is not charged against the services' compute quota at all (it carries `bex-datastore`). Its scheduling footprint is its plan plus the barman sidecar's 100m/128Mi requests. _(Scope change 2026-10-30: the bounded sidecar **limits** of the original bullet moved to `w2/055`. Limits do not affect scheduling, the compute quota no longer sees datastores, and that rollout restarts every backed-up tenant Postgres.)_
- Existing tenant namespaces are migrated (their `tenant-quota` objects are reconciled to the new shape) without disrupting running workloads.

## Source + Goal linkage

- **Source:** promoted from inbox note [w2/054](source-w2-054.md) (found in w2/m245 t005, 2026-10-30).
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) Render parity first. Render gates HA only on a ≥1-CPU plan, not on the workspace plan (its HA and features-by-plan docs, re-read 2026-10-30). This also completes what w2/m171 shipped (HA plans) for Hobby workspaces.
- **Expected outcome:** HA Postgres works in every workspace. The compute quota keeps protecting against runaway services only.
- **Why now:** w2/m171 just made HA sellable. Without this, every Hobby customer who enables it gets a database stuck at one instance.
- **Render parity included:** creation results across REST/GraphQL/MCP/dashboard/Blueprint change from stuck to healthy.

## Decision (2026-10-30, by the parity rule)

Exempt datastore Pods (CNPG instances, poolers and Key Value) from the services' compute quota rather than enlarging the Hobby quota. Hobby already allows 25 paid Postgres (`store/plans.go`: "paid datastores must not consume a Hobby cap"), but its 3-CPU compute quota contradicted that. Every datastore instance is billed, and its count is capped. Not adopted: sizing the Hobby quota for N HA pairs. That would reintroduce a plan-based cap Render does not have, and it does not scale with the 25-instance count cap.

## t001 + t003 — 2026-10-30

- **t001 (shipped `56b2f5063`):** PriorityClass `bex-datastore` has value 0, so scheduling and preemption are unchanged. It lives in `deploy/gitops/base/datastore-priority.yaml` (GitOps wave −3, before the operator) and `scripts/mock-cluster.sh`. The quota is split, because the API server refuses a PriorityClass scope on storage and `count/*` keys. `tenant-compute-quota` holds requests/limits CPU and memory and is scoped `PriorityClass NotIn [bex-datastore]`. `tenant-quota` keeps its name and the counts, pods, storage, PVCs, LoadBalancers and ephemeral storage, unscoped. **NotIn semantics proven on a v1.36 apiserver:** a class-less Pod is charged, and `bex-datastore` Pods (even past the cap) are not. **Migration:** the periodic namespace reconcile creates the scoped quota first, then strips CPU/memory from `tenant-quota`, so a namespace is never uncapped. The operator stamps the class on CNPG Clusters, Pooler templates, the Valkey StatefulSet and the Key Value internal proxy, but on **existing** datastores only when their Pods roll anyway (resize, image change, added instance such as an HA standby, restart, hibernation), so the rollout restarts nothing. Operator workload admission lets only KeyValue-owned workloads carry the class.
- **t003 (local `dev-2`, throwaway Hobby workspace):** a `2c-4g` HA Postgres with a pooler reached 2/2 healthy instances on distinct nodes (`highAvailabilityEnabled: true`). Its Pods and a Key Value's Pods carried `bex-datastore`, and `tenant-compute-quota` showed 0 CPU used. Services stay capped: a `pro_plus` (4 CPU) web service was refused with `exceeded quota: tenant-compute-quota … limited: requests.cpu=3`. Everything was cleaned up.
- **Not taken:** t002's commit (bounding the barman-cloud sidecar to 1 CPU / 1 Gi limits). Its rollout recreates every backed-up tenant Postgres Pod once (the operator copies the ObjectStore into each namespace), which is a brief outage for every single-instance customer database. After t001 it no longer matters for quotas, and limits do not affect scheduling. Moved to `w2/055` for a planned window.
- **Follow-up found:** Hobby `persistentvolumeclaims: 8` and `pods: 50` in `tenant-quota` still count datastore volumes and Pods, so the number of HA databases is capped far below the 25-database count cap. Filed as `w2/056`.

## t004 + t005 + t006 — 2026-10-31

- **t004 (production, PASS).** Hobby workspace `tian-personal` (`tea-da2isimlm39c739m4ofg`), already running two services (`tianpan-v4-web` starter, a free fixture). `POST /v1/postgres {"plan":"1c-2g","enableHighAvailability":true}` → `dpg-dbirn1ilpuls73ei2gg0` and `{"plan":"2c-4g",…}` → `dpg-dbirn1qt5fis7381js80` both reached `available`, `highAvailabilityEnabled: true`, with instances on two nodes each (`bex-tenant-0-tghbn-s67g5` / `-f2fsk`) and `priorityClassName: bex-datastore`. No `exceeded quota` event; `tenant-compute-quota` used stayed `requests.cpu 600m` (the services only). **Services still capped:** a `pro_plus` (4 CPU) image service in the same workspace was refused by `exceeded quota: tenant-compute-quota, requested: requests.cpu=4` (the customer-facing wording of that refusal is poor; filed `w2/060`). The live quota is the post-056 shape (`pods` in the scoped quota, PVCs 208). All three fixtures deleted.
- **t005.** ADR043 already carried the scoped-quota design (056/057), and `quotaForPlan`'s comment states the services-only budget (`namespaces.go:544`). Added to ADR009 § High availability ("Available in every workspace plan") and the ADR018 HA row, with the production evidence.
- **t006 (Render parity).** Render gates HA only on a ≥1-CPU plan, not on the workspace plan. The change sits below every surface (API-server quota scope + operator PriorityClass), so REST, GraphQL, MCP, the CLI, Blueprint and the dashboard toggle all reach the same healthy pair; no wire field or error code changed. Remaining divergence, recorded in `w2/060`: Render lets Hobby run any paid instance type, while bex keeps a 3-CPU services quota whose refusal surfaces only as a misleading stall hint.

## t007 + t008 — 2026-10-31

- **t007 (Simplify).** A read-only review of `56b2f5063`, `7a4df5737` and `25b99d8a3` found no logic worth simplifying (small single-purpose helpers). Comment-only cleanups in `lego/backend/internal/store/namespaces.go`: the per-datastore footprint derivation duplicated in the `quotaForPlan` comment now points at `datastoreFootprint` and `TestDatastoreQuotaRoomCoversLargestShape`; the w3/m34→w7/m77 history paragraph points at ADR043 D8; the twice-told v1.36 scope proof is one sentence; a stale `exceeded quota: tenant-quota` (now `tenant-compute-quota`) is corrected.
- **t008 (Test coverage).** Three gaps closed, each mutation-checked (the test fails when the protected behavior is removed):
  - `TestQuotaMigrationNeverLeavesNamespaceUncapped` (backend store): `tenant-compute-quota` exists before the update that strips CPU/pods from `tenant-quota`, and a failed compute-quota create leaves `tenant-quota`'s caps intact. Swapping the order in `baseResourceQuotas` fails both subtests.
  - `TestReconcileClusterHoldsThePriorityClassUntilTheClusterRolls` (operator): the `reconcileCluster` call site keeps an existing class-less Cluster class-less on an unchanged reconcile and adopts the class when HA adds an instance. Removing the gate fails it.
  - `TestKeyValueInternalProxyAdoptsThePriorityClassOnlyWhenItRolls` (operator): the same rule for the Key Value proxy Deployment. Passing `existed=false` at the call site fails it.
  - Not added: the Pooler call site (same pattern; `TestExistingPoolerFollowsItsClustersPriorityClass` covers its rule) and a 7-instance version of `TestHobbyDatastoreCountCapsBindFirst` (the arithmetic is pinned by `TestDatastoreQuotaRoomCoversLargestShape`).

## Closeout — 2026-10-31

Every Definition-of-done bullet holds: (1) production Hobby `1c-2g`/`2c-4g` HA pairs healthy on distinct nodes, no `exceeded quota` (t004); (2) services still refused past `tenant-compute-quota` (t004); (3) instance Pods carry `bex-datastore` and are not charged to the compute quota; the bounded sidecar limits shipped separately in `w2/055` (done 2026-10-31); (4) existing namespaces carry the migrated two-quota shape on production (`tea-daif693dqjvc73e7as3g`, `tea-da2isimlm39c739m4ofg`) with no workload disruption, and the migration order is now pinned by test. Follow-ups: `w2/060` (quota-refused deploy wording), `w2/061` (Hobby services cap vs Render, user decision).
