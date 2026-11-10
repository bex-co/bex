# w22 · m311 — Sandbox groups, as on Render: a default group per workspace, a concurrency limit, and 429 at the limit

**Worker:** worker22 **Goal:** `sandbox-groups create|list` and `sandboxes create --group` work as on Render, with a default group per workspace and a 429 at the concurrency limit **Status:** todo (t001–t006 and t012 done; t007 needs the deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design: sandbox groups, the default group and the limit semantics, checked against render.com — **DONE** | 45m | — |
| t002 | Store: sandbox_groups table, default group and sandbox binding (default created lazily, no backfill) — **DONE** | 1h | t001 |
| t003 | REST: serve the group routes and the group on create and list — **DONE** | 1h | t002 |
| t004 | Concurrency limit: 429 on create and on every resume, without racing — **DONE** | 1h | t003 |
| t005 | GraphQL and MCP: groups and sandboxGroupId, answering the same as REST — **DONE** | 1h | t004 |
| t006 | Resource and abuse bounds for compute (ADR104 D11); the egress alert moved to w22/002 — **DONE** | 1h30m | t004 |
| t012 | Dashboard: "+ New > Sandbox Group", as on Render — **DONE** | 1h30m | t005 |
| t007 | Production walk with the unmodified CLI, and flip the ledger rows | 1h | t005, t006, t012 |
| t008 | Render parity | 30m | t007 |
| t009 | Simplify | 20m | t008 |
| t010 | Test coverage | 40m | t008 |
| t011 | Closeout | 10m | t009, t010 |

## Definition of done

- On production with the unmodified pinned Render CLI: `sandbox-groups create` and `list` return groups with Render's fields; every workspace has exactly one default group; `sandboxes create --group <id>` places the sandbox in it
- A group at its limit answers 429 on create and on every resume path; suspended sandboxes do not count; lowering the limit stops nothing
- REST, GraphQL and MCP answer the same way for every new behavior, and the dashboard can create a group from "+ New" as Render's does
- ADR018 and the CLI checklist rows that called groups a non-goal are amended; ADR104 gaps 2 and 12 are closed and the compute part of gap 14 is in place (the egress alert is `w22/002`)

## Source + Goal linkage

- **Source:** [docs/ADR104-sandbox-render-parity.md](../../../docs/ADR104-sandbox-render-parity.md), whose gap table is the proposal (no separate `/pm-brainstorm` run), filed by `/pmx adr104 to w22` on 2026-11-10; gaps 2, 12 (`group`) and 14 (compute and egress part)
- **Goal linkage:** Render parity first (ADR008 § Render parity first; ADR104 D1 and D7); pillar 5 sandboxes (re-opened 2026-07-27, `.pm/DO_NOT_DO.md` #18)
- **Expected outcome:** The official Render CLI and SDKs can create and list sandbox groups against bex, and a group at its limit refuses new work as on Render
- **Why now:** Groups are the largest unscheduled parity gap (ADR104 D7): snapshots (w32/m285, blocked) are scoped to a group and need at least a default group per workspace before m285 t002.
- **Render parity:** included, sandbox REST/GraphQL/MCP surfaces change. Added 2026-11-10 during triage: Render's docs show a Dashboard "+ New > Sandbox Group" menu, so ADR104 D1's "dashboard N/A" was wrong for groups; t012 covers it (the optional project environment is a recorded gap, see t012).
- **Not duplicated:** w3/m303 owns the CLI pin bump, the Hobby timeout contract and the named refusals for `sandbox-groups delete`, `default set` and `credentials`.
- **Open decisions:** none new. The over-10-groups error and `environmentId` are decided in t001 from Render's docs as a recommendation, not a user decision.
