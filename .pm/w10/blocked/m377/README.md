# w10 · m377 — An unpaged Render-shaped list returns Render's first page of 20, as on Render

**Worker:** worker10 **Goal:** every REST list operation in Render's pinned contract that bex serves, called with neither `cursor` nor `limit`, answers Render's default page of 20 with a cursor to continue, as the user decided on 2026-11-13; bex's own clients that need whole lists page through. **Status:** blocked (release gate: t001–t008 done 2026-11-14; t009 waits for the batched production deploy, then a worker replays DoD 2 as the QA user on a list that changed, such as `/v1/sandboxes` or a static site's `/routes`; no user action. The REST-only scope is still the proposal's recommendation, for the user to confirm)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Inventory: what each served list operation returns unpaged, and which in-repo clients read whole lists — **DONE** | 45m | — |
| [t002](done/t002.md) | Contract: retire the full-set default for REST in ADR006 and record the decision in ADR018 — **DONE** | 20m | t001 |
| [t003](done/t003.md) | Every served Render list answers Render's default page when unpaged — **DONE** | 1.5h | t002 |
| [t004](done/t004.md) | A guard test walks every served pinned list operation — **DONE** | 1h | t003 |
| [t005](done/t005.md) | bex's own REST clients page through whole lists; MCP follows Render's MCP server — **DONE** | 45m | t003 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t003, t004, t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 40m | t006, t007 |
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

**2026-11-14 (`/loopx w10`).** Only six of the 45 pinned list operations still returned more than Render's page unpaged; the rest already answered 20 (most since w8/191). DoD 2's `GET /v1/projects` was one of those, so it cannot show the change; the live check uses a switched list instead. Several ADR006/ADR018 lines had described the old default wrongly before this change (env vars, secret files and Postgres already answered 20; deploys answered 100, not the full history). The guard caught one list whose `limit` is defined inline (`list-sandbox-groups`), which a `$ref`-only check would have missed.
