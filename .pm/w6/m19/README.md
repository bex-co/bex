# w6 · m19 — Protected-environment ACLs

**Worker:** worker6 **Goal:** bring Environments (`w1/m32`) to Render parity on `protectedStatus`/`networkIsolationEnabled`/`ipAllowList` **Status:** todo

## Tasks (in order)

| id   | title                                                                                                | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Store: add `protectedStatus`/`networkIsolationEnabled`/`ipAllowList` columns + migration + `Environment` struct fields | 45m | —          |
| t002 | REST/GraphQL/MCP verbs to get/set the three fields on an Environment                                   | 1h  | t001       |
| t003 | New `core.LabelEnvironment` CR label + writer path projecting environment membership onto App CRs (`environments/service.go`) | 1h  | t001       |
| t004 | Operator: environment-scoped `NetworkPolicy` variant in `reconcileNetworkPolicy` (`app_controller.go`), active when `networkIsolationEnabled` | 1.5h | t003      |
| t005 | Enforce `protectedStatus: protected` — block delete/suspend/direct-deploy-override verbs on member Apps without explicit confirmation | 1h  | t002       |
| t006 | Propagate environment `ipAllowList` to member Database/KeyValue resources' existing per-resource allowlist mechanism | 1h  | t002       |
| t007 | Render parity: verify field shape/semantics/errors consistent across REST/GraphQL/MCP + dashboard UI  | 45m | t006       |
| t008 | Simplify: run `/simplify` over the code this milestone changed                                         | 30m | t007       |
| t009 | Test coverage: enforcement tests for protection/isolation/allowlist, not just storage round-trip       | 1h  | t007       |
| t010 | Closeout                                                                                                | 15m | t009       |

## Definition of done

Setting `protectedStatus=protected` on an Environment blocks unguarded destructive verbs (delete/suspend/direct-deploy-override) on member Apps via REST/GraphQL/MCP; `networkIsolationEnabled=true` produces a `NetworkPolicy` that denies traffic between that environment's Apps and everything outside it; an environment-level `ipAllowList` is enforced on member Postgres/KeyValue. All three verified with tests that assert enforcement, not just storage.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones` 2026-07-13 — `docs/ADR018-render-parity.md` Environments row + `docs/ADR032-environments.md`'s "Known divergence" section, which names this gap explicitly (`w1/m32` shipped Environments without the protection/isolation half of Render's API). Materialized under `w6` (Workspaces/tenancy) rather than `w1` (where the Environments mechanism itself lives) **per user direction** — same cross-workstream-placement pattern as `014.md` → `w2/m28`.
- **Goal linkage:** Render parity pillar — closes a documented gap on an already-shipped surface (Environments).
- **Expected outcome:** Environments API matches Render's `protectedStatus`/`networkIsolationEnabled`/`ipAllowList` fields with real enforcement.
- **Why now:** the App-CR label plumbing this needs (t003) is also a prerequisite for `w6/m20`; landing it here unblocks both. Render parity included — this changes REST/GraphQL/MCP fields and should be checked against dashboard UI.
