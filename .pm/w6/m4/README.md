# w6 · m4 — Workspace-scoped datastores: fix ownerId labeling + wire real delete purgers

**Worker:** worker6 **Goal:** deleting a workspace actually tears down every resource it owns (managed Postgres, managed KeyValue, OpenBao secrets — not just the tenant row and FGA tuples, as today), and `ownerId` scoping works correctly for Postgres and KeyValue the way it already does for Apps. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                      | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Fix Postgres `ownerId` label bug: stamp `core.LabelTenant` (in addition to the existing `core.LabelWorkspace`) on `CreatePostgres` (`postgres/service.go`) | 20m | —          |
| t002 | Add workspace identity to KeyValue: stamp both `core.LabelTenant`/`core.LabelWorkspace` on `CreateKeyValue`; add `OwnerID` field + `ownerId` list-filter to `KeyValueView`/`ListKeyValues`, mirroring the Apps/Postgres contract | 45m | —          |
| t003 | Implement `secrets.WorkspacePurger`: delete a tenant's entire OpenBao secret tree (env vars, env groups, secret files) on workspace delete       | 45m | —          |
| t004 | Implement `postgres.WorkspacePurger` + `keyvalue.WorkspacePurger`: delete every `Database`/`KeyValue` CR labeled with the deleted tenant           | 45m | t001, t002 |
| t005 | Wire all three purgers into `cmd/api/main.go`'s `ServerDeps.WorkspacePurgers`                                                                    | 20m | t003, t004 |
| t006 | Integration test: create a workspace, provision a Database + KeyValue + env vars, delete the workspace, verify all three are actually gone (extend `w6/m1`'s `workspaces_e2e_test`) | 45m | t005       |

## Definition of done

Deleting a workspace tears down its managed Postgres databases, managed KeyValue stores, and OpenBao secrets — not just the tenant row and FGA tuples, as today; `ownerId` filtering/listing on Postgres and KeyValue works correctly and matches the contract Apps already have; a tenant's own App can reach its own managed Valkey instance over the network.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w6` 2026-07-09, from tracing `w6/m1`'s own deferred-followup note ("OpenBao/Database purger concrete impls") plus direct code inspection of `postgres/service.go`, `keyvalue/service.go`, `workspaces/service.go`, `cmd/api/main.go`, and `docs/tenant-isolation.md:104` (documents the unlabeled-KeyValue same-workspace-reachability gap as a "correct safe default" — safe, but functionally broken).
- **Goal linkage:** completes `w6/m1`'s DeleteWorkspace contract (currently under-delivers its own documented DoD — `Purgers` is wired as an interface but has zero concrete implementations and is never populated) and `w6/m2`'s `ownerId` REST/GraphQL/MCP contract (currently broken for Postgres via a label-key mismatch, absent entirely for KeyValue).
- **Expected outcome:** workspace delete is actually safe — no orphaned tenant data or infra (CNPG clusters, Valkey StatefulSets, OpenBao secrets); `ownerId` scoping is trustworthy across every managed resource type, not just Apps.
- **Why now:** both gaps were explicitly flagged as deferred in `w6/m1`'s own README and are small, mechanical fixes sharing one root cause (workspace-identity labeling was never finished for Postgres/KeyValue the way it was for Apps) — cheaper to close now than to rediscover later as a data-hygiene incident or a live parity bug report.
- **Render parity: included** (standing task) — touches the `ownerId` REST/GraphQL/MCP list-filter surface directly (t002).
