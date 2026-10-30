# w2 · m247 — High-availability Postgres fits in any workspace, as on Render

**Worker:** worker2 **Goal:** a Hobby workspace can create and run any sellable Postgres (including a `2c-4g` HA pair) and Key Value, because datastore Pods are not charged against the services' compute quota, and an instance's quota charge matches its plan. **Status:** todo — t001 shipped (`56b2f5063`), t003 verified locally; next t004 on production after the deploy

## Tasks (in order)

| id   | title                                                                                              | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Datastore Pods carry a dedicated PriorityClass, and the compute quota excludes it (new + existing namespaces) — **DONE** | 1h  | —          |
| t002 | Bounded barman-cloud sidecar resources, so a 1-CPU instance is not charged 3 CPU of limits        — **MOVED to `w2/055`** (not needed for the DoD after t001; its rollout restarts every backed-up tenant Postgres) | 30m | —          |
| t003 | Prove on the local cluster that a Hobby-quota namespace runs a `2c-4g` HA pair, and that services are still capped — **DONE** | 40m | t001, t002 |
| t004 | Production: a Hobby workspace runs `1c-2g` and `2c-4g` HA databases with two healthy instances (then delete) | 30m | t003       |
| t005 | Correct ADR043/ADR009/ADR018 and the `quotaForPlan` rationale                                      | 20m | t004       |
| t006 | Render parity                                                                                      | 15m | t005       |
| t007 | Simplify                                                                                           | 15m | t006       |
| t008 | Test coverage                                                                                      | 30m | t006       |
| t009 | Closeout                                                                                           | 10m | t007, t008 |

## Definition of done

- In a production Hobby workspace that already runs services, `POST /v1/postgres {"plan":"2c-4g","enableHighAvailability":true}` reaches two healthy instances on distinct nodes, and so does `1c-2g`. No `exceeded quota: tenant-quota` appears.
- Services in a Hobby workspace are still bounded by the compute quota (a deploy past it is still refused), while datastores are bounded by their count caps (`QuotaCapsForPlan`: 25 Postgres / 25 Key Value) and per-instance billing.
- A Postgres instance Pod is not charged against the services' compute quota at all (it carries `bex-datastore`). Its scheduling footprint is its plan plus the barman sidecar's 100m/128Mi requests. _(Scope change 2026-10-30: the bounded sidecar **limits** of the original bullet moved to `w2/055`. Limits do not affect scheduling, the compute quota no longer sees datastores, and that rollout restarts every backed-up tenant Postgres.)_
- Existing tenant namespaces are migrated (their `tenant-quota` objects are reconciled to the new shape) without disrupting running workloads.

## Source + Goal linkage

- **Source:** promoted from inbox note [w2/054](source-w2-054.md) (found in w2/m245 t005, 2026-10-30).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) Render parity first. Render gates HA only on a ≥1-CPU plan, not on the workspace plan (its HA and features-by-plan docs, re-read 2026-10-30). This also completes what w2/m171 shipped (HA plans) for Hobby workspaces.
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
