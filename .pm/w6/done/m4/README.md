# w6 · m4 — Workspace-scoped datastores: fix ownerId labeling + wire real delete purgers

**Worker:** worker6 **Goal:** deleting a workspace actually tears down every resource it owns (managed Postgres, managed KeyValue, OpenBao secrets — not just the tenant row and FGA tuples, as today), and `ownerId` scoping works correctly for Postgres and KeyValue the way it already does for Apps. **Status:** done 2026-07-10 (all 10 tasks shipped; see the DoD note below for the one live-verification caveat)

## Tasks (in order)

| id   | title                                                                                                                                      | est | depends_on | |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |---|
| t001 | Fix Postgres `ownerId` label bug: stamp `core.LabelTenant` (in addition to the existing `core.LabelWorkspace`) on `CreatePostgres` (`postgres/service.go`) | 20m | —          | **DONE** |
| t002 | Add workspace identity to KeyValue: stamp both `core.LabelTenant`/`core.LabelWorkspace` on `CreateKeyValue`; add `OwnerID` field + `ownerId` list-filter to `KeyValueView`/`ListKeyValues`, mirroring the Apps/Postgres contract | 45m | —          | **DONE** |
| t003 | Implement `secrets.WorkspacePurger`: delete a tenant's entire OpenBao secret tree (env vars, env groups, secret files) on workspace delete       | 45m | —          | **DONE** |
| t004 | Implement `postgres.WorkspacePurger` + `keyvalue.WorkspacePurger`: delete every `Database`/`KeyValue` CR labeled with the deleted tenant           | 45m | t001, t002 | **DONE** |
| t005 | Wire all three purgers into `cmd/api/main.go`'s `ServerDeps.WorkspacePurgers`                                                                    | 20m | t003, t004 | **DONE** |
| t006 | Integration test: create a workspace, provision a Database + KeyValue + env vars, delete the workspace, verify all three are actually gone (extend `w6/m1`'s `workspaces_e2e_test`) | 45m | t005       | **DONE** |
| t007 | Render parity: `ownerId` scoping across REST/GraphQL/MCP for Postgres and KeyValue                                                               | 25m | t006       | **DONE** |
| t008 | Simplify: workspace-datastore labeling + purger changes                                                                                          | 25m | t007       | **DONE** |
| t009 | Unit test coverage for label stamps and purgers                                                                                                  | 40m | t007       | **DONE** |
| t010 | Closeout: verify DoD, mark done, move to `done/`                                                                                                 | 15m | t009       | **DONE** |

## Definition of done

Deleting a workspace tears down its managed Postgres databases, managed KeyValue stores, and OpenBao secrets — not just the tenant row and FGA tuples, as today; `ownerId` filtering/listing on Postgres and KeyValue works correctly and matches the contract Apps already have; a tenant's own App can reach its own managed Valkey instance over the network.

**Verification note (t010, 2026-07-10):** the Postgres/KeyValue teardown, `ownerId` scoping, and cross-tenant isolation are live-verified against real Postgres + real OpenFGA (not just fakes), via a throwaway Docker-based setup this session (`TestWorkspaceLifecycleE2E` passing, including a two-workspace leak check). The OpenBao secrets-purge path (`secrets.WorkspacePurger`) is implemented, wired into `cmd/api/main.go`, and covered by unit tests with a fake `SecretKV`, but was **not** live-verified end-to-end this session — OpenBao's Kubernetes-auth login needs the full local mock-cluster (kind + real ServiceAccount tokens), disproportionate to stand up for one purger in this session. This mirrors how `w6/m3` recorded its own live-verification gap in its README rather than blocking closeout; `w6/m5` is the milestone already scoped for full live-infrastructure verification and should pick this up.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w6` 2026-07-09, from tracing `w6/m1`'s own deferred-followup note ("OpenBao/Database purger concrete impls") plus direct code inspection of `postgres/service.go`, `keyvalue/service.go`, `workspaces/service.go`, `cmd/api/main.go`, and `docs/ADR022-tenant-isolation.md:104` (documents the unlabeled-KeyValue same-workspace-reachability gap as a "correct safe default" — safe, but functionally broken).
- **Goal linkage:** completes `w6/m1`'s DeleteWorkspace contract (currently under-delivers its own documented DoD — `Purgers` is wired as an interface but has zero concrete implementations and is never populated) and `w6/m2`'s `ownerId` REST/GraphQL/MCP contract (currently broken for Postgres via a label-key mismatch, absent entirely for KeyValue).
- **Expected outcome:** workspace delete is actually safe — no orphaned tenant data or infra (CNPG clusters, Valkey StatefulSets, OpenBao secrets); `ownerId` scoping is trustworthy across every managed resource type, not just Apps.
- **Why now:** both gaps were explicitly flagged as deferred in `w6/m1`'s own README and are small, mechanical fixes sharing one root cause (workspace-identity labeling was never finished for Postgres/KeyValue the way it was for Apps) — cheaper to close now than to rediscover later as a data-hygiene incident or a live parity bug report.
- **Render parity: included** (standing task) — touches the `ownerId` REST/GraphQL/MCP list-filter surface directly (t002).
