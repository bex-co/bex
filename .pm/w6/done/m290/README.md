# w6 · m290 — Deleting an environment or project handles its Postgres, Key Value and env groups as Render does

**Worker:** worker6 **Goal:** deleting an environment or a project leaves every contained datastore and env group in the state Render leaves it in (verified on a live Render account first), identically on REST, GraphQL, MCP and the dashboard. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Verify on a live Render account what deleting an environment and a project does to its Postgres, Key Value and env groups](done/t001.md) — **DONE** | 45m | — |
| t002 | [Environment delete removes its Postgres, Key Value and env groups through their normal delete paths](done/t002.md) — **DONE** | 1.5h | t001 |
| t003 | [Project delete cascades the same way through its environments](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [REST, GraphQL and MCP report the cascade with the same semantics and error shapes](done/t004.md) — **DONE** | 45m | t002 |
| t005 | [Dashboard delete confirmation lists the datastores and env groups that will be deleted](done/t005.md) — **DONE** | 45m | t002 |
| t006 | [Amend ADR032 and ADR018 row 225 with the Render evidence and the shipped behavior](done/t006.md) — **DONE** | 30m | t001 |
| t007 | [Render parity](done/t007.md) — **DONE** | 30m | t003, t004, t005, t006 |
| t008 | [Simplify](done/t008.md) — **DONE** | 15m | t007 |
| t009 | [Test coverage](done/t009.md) — **DONE** | 30m | t008 |
| t010 | [Closeout](done/t010.md) — **DONE** | 10m | t009 |

## Definition of done

- t001 records Render's outcome for each of Postgres, Key Value and env group under environment delete and project delete.
- On production, deleting an environment (and a project) that contains a Postgres, a Key Value and an env group leaves each in exactly t001's recorded state, observed on REST, GraphQL, MCP and the dashboard.
- A non-Admin delete of a protected environment is refused and removes nothing; billing for deleted datastores stops.
- ADR032 and ADR018 row 225 match the shipped behavior.
- **Conditional scope:** if t001 shows Render keeps or detaches them, t002–t005 are closed as not needed with that evidence, and the DoD is t006 alone (row 225 ✅ with evidence).

## Render evidence (t001, 2026-11-08)

Probed on the maintainer's render.com workspace via the public API (Render CLI token, never printed). Probe project `prj-dbo46qu417fc73f7rk80` with environment `evm-dbo46qu417fc73f7rk8g` held one each of a free Postgres, a free Key Value, an env group and a free image web service. Every probe resource was deleted afterwards, and the project now answers 404.

- **API environment delete with resources:** `DELETE /v1/environments/{id}` returned **500** `internal server error` twice, the second time after Postgres was `available`. Nothing was deleted, and all four resources stayed in the environment.
- **API project delete with resources:** `DELETE /v1/projects/{id}` returned **409** `cannot delete environment that contains resources: evm-…`. Nothing was deleted.
- **API empty deletes:** the empty environment and the empty project each returned **204**.
- **Dashboard:** not driven, because it needs the maintainer's browser session. render.com/docs/projects says "Deleting an environment deletes all of its associated services", and the same page counts "a database" among a project's "services" ("a static site, a GraphQL backend, and a database. By adding all of these services to the same project…"). The protected-environment section lists "services, environment groups, etc." as separate resource kinds.

**Verdict and revised scope:**

1. **REST** already matches the Render API. `DeleteIfEmpty` (w8/188) returns 409 and deletes nothing (`environments/rest.go:222`, `projects/rest.go:208`). Render's 500 on the environment route is a Render bug, not behavior to copy. No REST change.
2. **Dashboard, GraphQL and MCP** use the cascading `Delete`, which mirrors Render's dashboard. Postgres and Key Value count as services in Render's own wording, so they are **deleted** with the environment or project, through their normal deletes (`DeletePostgres`/`DeleteKeyValue`: authorization, protected gate, fresh can_create).
3. **Env groups** stay **detached**. Render lists them apart from services, and nothing says the dashboard deletes them. Detaching is the reversible choice.
4. Evidence gap: the Render dashboard's own confirmation dialog was not seen. A maintainer with a browser session can confirm it, and that check would refine the env-group choice. It is not a gate.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w6` 2026-11-08, proposal 1; ADR018 row 225; `w4/done/m264` README lines 48 and 83 ("file any wider cascade as a follow-up", never filed).
- **Goal linkage:** ADR008 Render parity first — customer-facing delete semantics must match render.com.
- **Expected outcome:** a customer deleting an environment or project on bex gets the same result as on Render, with no orphaned, still-billed datastores if Render deletes them.
- **Why now:** w4/m264 just reworked the same delete path, nobody owns the follow-up, and w3/m281 (protected environments) is changing the adjacent gate.
- **Render parity:** included — changes REST, GraphQL, MCP and dashboard behavior.
