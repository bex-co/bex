# w2 · m247 — High-availability Postgres fits in any workspace, as on Render

**Worker:** worker2 **Goal:** a Hobby workspace can create and run any sellable Postgres (including a `2c-4g` HA pair) and Key Value, because datastore Pods are not charged against the services' compute quota, and an instance's quota charge matches its plan. **Status:** todo

## Tasks (in order)

| id   | title                                                                                              | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Datastore Pods carry a dedicated PriorityClass, and the compute quota excludes it (new + existing namespaces) | 1h  | —          |
| t002 | Bounded barman-cloud sidecar resources, so a 1-CPU instance is not charged 3 CPU of limits        | 30m | —          |
| t003 | Prove on the local cluster that a Hobby-quota namespace runs a `2c-4g` HA pair, and that services are still capped | 40m | t001, t002 |
| t004 | Production: a Hobby workspace runs `1c-2g` and `2c-4g` HA databases with two healthy instances (then delete) | 30m | t003       |
| t005 | Correct ADR043/ADR009/ADR018 and the `quotaForPlan` rationale                                      | 20m | t004       |
| t006 | Render parity                                                                                      | 15m | t005       |
| t007 | Simplify                                                                                           | 15m | t006       |
| t008 | Test coverage                                                                                      | 30m | t006       |
| t009 | Closeout                                                                                           | 10m | t007, t008 |

## Definition of done

- In a production Hobby workspace that already runs services, `POST /v1/postgres {"plan":"2c-4g","enableHighAvailability":true}` reaches two healthy instances on distinct nodes, and so does `1c-2g`. No `exceeded quota: tenant-quota` appears.
- Services in a Hobby workspace are still bounded by the compute quota (a deploy past it is still refused), while datastores are bounded by their count caps (`QuotaCapsForPlan`: 25 Postgres / 25 Key Value) and per-instance billing.
- A Postgres instance Pod's quota-relevant requests/limits equal its plan plus a bounded, documented sidecar allowance.
- Existing tenant namespaces are migrated (their `tenant-quota` objects are reconciled to the new shape) without disrupting running workloads.

## Source + Goal linkage

- **Source:** promoted from inbox note [w2/054](source-w2-054.md) (found in w2/m245 t005, 2026-10-30).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) Render parity first. Render gates HA only on a ≥1-CPU plan, not on the workspace plan (its HA and features-by-plan docs, re-read 2026-10-30). This also completes what w2/m171 shipped (HA plans) for Hobby workspaces.
- **Expected outcome:** HA Postgres works in every workspace. The compute quota keeps protecting against runaway services only.
- **Why now:** w2/m171 just made HA sellable. Without this, every Hobby customer who enables it gets a database stuck at one instance.
- **Render parity included:** creation results across REST/GraphQL/MCP/dashboard/Blueprint change from stuck to healthy.

## Decision (2026-10-30, by the parity rule)

Exempt datastore Pods (CNPG instances, poolers and Key Value) from the services' compute quota rather than enlarging the Hobby quota. Hobby already allows 25 paid Postgres (`store/plans.go`: "paid datastores must not consume a Hobby cap"), but its 3-CPU compute quota contradicted that. Every datastore instance is billed, and its count is capped. Not adopted: sizing the Hobby quota for N HA pairs. That would reintroduce a plan-based cap Render does not have, and it does not scale with the 25-instance count cap.
