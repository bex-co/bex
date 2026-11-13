# w10 · m377 — An unpaged Render-shaped list returns Render's first page of 20, as on Render

**Worker:** worker10 **Goal:** every REST list operation in Render's pinned contract that bex serves, called with neither `cursor` nor `limit`, answers Render's default page of 20 with a cursor to continue, as the user decided on 2026-11-13; bex's own clients that need whole lists page through. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Inventory: what each served list operation returns unpaged, and which in-repo clients read whole lists | 45m | — |
| [t002](t002.md) | Contract: retire the full-set default for REST in ADR006 and record the decision in ADR018 | 20m | t001 |
| [t003](t003.md) | Every served Render list answers Render's default page when unpaged | 1.5h | t002 |
| [t004](t004.md) | A guard test walks every served pinned list operation | 1h | t003 |
| [t005](t005.md) | bex's own REST clients page through whole lists; MCP follows Render's MCP server | 45m | t003 |
| [t006](t006.md) | Render parity | 20m | t003, t004, t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 40m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 5h45m**.

## Definition of done

1. Every pinned list operation bex serves (§ Findings), called without `cursor` and `limit` over more than 20 items, returns exactly 20 and a cursor that fetches the rest; `limit=100` returns up to 100; the spec-driven guard test fails if any one regresses. `bash scripts/ship-gates.sh` passes.
2. On production, after the deploy that carries this milestone, as the QA user in `bex-canary`: an unpaged `GET /v1/projects` (or another list, after creating 21 throwaway items and deleting them afterwards) returns 20 and a cursor; following the cursor returns the rest.
3. The dashboard's service, database and project lists (GraphQL) show every item as before.
4. The scripts t001 listed page through and pass where they run.
5. ADR006 and ADR018 record the 2026-11-13 decision and the per-surface scope.

## Source + Goal linkage

- **Source:** [w22/007](../../w22/done/007.md) (found 2026-11-10 while pinning the sandbox list `limit`, w22/001), blocked on the user's choice; **user decision 2026-11-13** in the `/pm-brainstorm for w10` follow-up: "w22/007 -> Render 返回 20 条" (switch to Render's 20), given with `/pmx them all to w10`. Placed in w10 because the switch is a cross-feature change larger than an hour, as the note's sizing said, and w10 is draining now.
- **Render:** the pinned `render-public-api-1.json` defines `limitParam` ("Defaults to 20", 1-100) on every list operation, and `auditLogLimitParam` (default 20, max 1000) on audit logs; [pagination](https://api-docs.render.com/reference/pagination).
- **Scope (recorded as the proposal's recommendation, not a user decision):** the decision applies to REST, Render's API contract. GraphQL is bex's own API (the dashboard and mobile app read it) with no Render counterpart, so it keeps returning whole lists; MCP follows Render's MCP server, whose pinned tools take no paging arguments, after t005 reads what it returns. Still open: the user's confirmation of this per-surface scope.
- **Goal linkage:** Render parity first (ADR008); ADR006/ADR018 REST compatibility.
- **Expected outcome:** a Render client that never pages sees what it sees on Render; bex's own scripts no longer rely on a bex-only default.
- **Why now:** the user decided it today, and every new list added under the old default widens the divergence.
- **Render parity:** included (REST and MCP behavior change).

## Findings

None yet.
