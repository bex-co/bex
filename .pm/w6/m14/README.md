# w6 · m14 — Deterministic workspace targeting: multi-workspace caller-tenant resolution

**Worker:** worker6 **Goal:** a caller with more than one workspace stops getting an arbitrary one — the implicit caller→workspace resolution becomes deterministic, write paths accept an explicit `ownerId` (Render's own contract), and the dashboard's mutations and audit log follow the switcher. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                             | est | depends_on             |
| ---- | --------------------------------------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Deterministic default workspace: `TenantForIdentity` orders by membership `created_at` (oldest = default); document the contract  | 30m | —                      |
| t002 | `core.Base` seam: request-scoped workspace override for `Authorize`/`GetApp`/`Tenant`, honored only after a membership check      | 45m | t001                   |
| t003 | REST: `POST /v1/services` + the other `LabelTenant`-stamping write paths honor `ownerId` (body/query), per Render's OpenAPI       | 45m | t002                   |
| t004 | GraphQL: `createService` + sibling create mutations gain `ownerId`; MCP creates resolve via the session `select_workspace`        | 40m | t002                   |
| t005 | Dashboard: thread `currentWorkspaceId` into create/write mutations (services wizard, env vars, databases, keyvalue)               | 40m | t004                   |
| t006 | Root-cause + fix the `/settings` audit-log stale-workspace bug: align `use-audit-log.ts`'s workspace source with the switcher     | 30m | —                      |
| t007 | Two-workspace end-to-end check: create in B lands in B; B's app never 403s its owner; audit log follows the switcher              | 30m | t003, t005, t006       |
| t008 | Render parity: sweep REST/GraphQL/MCP/UI `ownerId` semantics vs Render's OpenAPI; update ADR018 row 19's stale omission note      | 30m | t007                   |
| t009 | Simplify: `/simplify` over the code this milestone changed                                                                        | 30m | t008                   |
| t010 | Test coverage: meaningful tests for the resolution order, the membership-checked override, and wrong-workspace 403 regressions    | 40m | t008                   |
| t011 | Closeout: verify the DoD holds, mark done, move to `w6/done/m14/`                                                                 | 15m | t010                   |

## Definition of done

A user with two workspaces can, over REST and the dashboard, create a service in a **named** workspace and find it there (`ownerId` honored end-to-end); naming a workspace the caller is not a member of is rejected (403, never silently redirected); `GetApp` never rejects an owner because the membership join picked their other workspace (regression test at `core.Base` level); with no explicit workspace given, resolution is deterministically the caller's oldest membership — documented on `TenantForIdentity`; the Settings → Security & Compliance audit-log card shows the currently-selected workspace's events after switching (live or stub-verified click-through).

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w6` 2026-07-12 — `w6/done/m11/README.md` t003 ("no reliable way to target the second workspace over REST": `TenantForIdentity`, `lego/backend/internal/store/store.go:277`, is a bare join with no `ORDER BY`) and t004 (the `/settings` audit-log stale-workspace bug); `docs/ADR018-render-parity.md` row "Create service" whose `ownerId` omission rationale ("single workspace") went stale when w6/m1 shipped multi-workspace; `RESEARCH-workspaces.md` findings 7 (user id → default workspace) and 10 (every Render resource carries `ownerId` for workspace scoping).
- **Goal linkage:** w6's founding theme — multi-workspace as a real product surface (today only *lists* honor a workspace; auth checks, tenant gates, and creates all resolve arbitrarily); GOAL multi-tenancy.
- **Expected outcome:** multi-workspace goes from "lists work" to "everything works": no wrong-workspace creates, no spurious 403s on `core.Base.GetApp`'s tenant gate (base.go:255), no mis-scoped audit log.
- **Why now:** it is a live correctness bug class m11 hit on real prod, and every ingredient already exists (switcher state, m2's `ownerId` list plumbing, `core.WorkspaceSelections`, the membership store) — the wiring cost will never be lower.
- **Render parity:** **included** (t008) — the milestone touches REST/GraphQL/MCP and the dashboard, and its whole point is adopting Render's `ownerId` create contract; the parity task also corrects ADR018's stale row-19 note.
