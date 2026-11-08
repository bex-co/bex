# w6 · m290 — Deleting an environment or project handles its Postgres, Key Value and env groups as Render does

**Worker:** worker6 **Goal:** deleting an environment or a project leaves every contained datastore and env group in the state Render leaves it in (verified on a live Render account first), identically on REST, GraphQL, MCP and the dashboard. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Verify on a live Render account what deleting an environment and a project does to its Postgres, Key Value and env groups](t001.md) | 45m | — |
| t002 | [Environment delete removes its Postgres, Key Value and env groups through their normal delete paths](t002.md) | 1.5h | t001 |
| t003 | [Project delete cascades the same way through its environments](t003.md) | 45m | t002 |
| t004 | [REST, GraphQL and MCP report the cascade with the same semantics and error shapes](t004.md) | 45m | t002 |
| t005 | [Dashboard delete confirmation lists the datastores and env groups that will be deleted](t005.md) | 45m | t002 |
| t006 | [Amend ADR032 and ADR018 row 225 with the Render evidence and the shipped behavior](t006.md) | 30m | t001 |
| t007 | [Render parity](t007.md) | 30m | t003, t004, t005, t006 |
| t008 | [Simplify](t008.md) | 15m | t007 |
| t009 | [Test coverage](t009.md) | 30m | t008 |
| t010 | [Closeout](t010.md) | 10m | t009 |

## Definition of done

- t001 records Render's outcome for each of Postgres, Key Value and env group under environment delete and project delete.
- On production, deleting an environment (and a project) that contains a Postgres, a Key Value and an env group leaves each in exactly t001's recorded state, observed on REST, GraphQL, MCP and the dashboard.
- A non-Admin delete of a protected environment is refused and removes nothing; billing for deleted datastores stops.
- ADR032 and ADR018 row 225 match the shipped behavior.
- **Conditional scope:** if t001 shows Render keeps or detaches them, t002–t005 are closed as not needed with that evidence, and the DoD is t006 alone (row 225 ✅ with evidence).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w6` 2026-11-08, proposal 1; ADR018 row 225; `w4/done/m264` README lines 48 and 83 ("file any wider cascade as a follow-up", never filed).
- **Goal linkage:** ADR008 Render parity first — customer-facing delete semantics must match render.com.
- **Expected outcome:** a customer deleting an environment or project on bex gets the same result as on Render, with no orphaned, still-billed datastores if Render deletes them.
- **Why now:** w4/m264 just reworked the same delete path, nobody owns the follow-up, and w3/m281 (protected environments) is changing the adjacent gate.
- **Render parity:** included — changes REST, GraphQL, MCP and dashboard behavior.
