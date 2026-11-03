# w21 · m260 — Cloud Agents MCP phase 4: completion webhooks, per-session spend ceiling, `outcome.cost`

**Worker:** worker21 **Goal:** Orchestrators are pushed completion events instead of polling and can cap spend per task, with cost visible on `outcome`. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| -- | ----- | --- | ---------- |
| t001 | Completion webhook events: turn succeeded / failed / needs input | 1.5h | w21/m257/t003 |
| t002 | Per-session `maxSpendUSD` ceiling in the model proxy | 2h | w10/m14/t004, w10/m14/t005 |
| t003 | `outcome.cost`: decimal-string model USD and sandbox seconds | 1h | w10/m14/t004, w21/m257/t003 |
| t004 | Render parity (vs Devin): REST, GraphQL, MCP and dashboard consistency | 30m | t001, t002, t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test coverage | 45m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

Workspace webhooks deliver the three agent-session event types with `{id,state,outcome}`; `maxSpendUSD` is enforced in the model proxy (and rejected with the named 400 until the meter exists); `outcome.cost` matches the ADR091 meter. Verified by backend tests and a live run.

## Source + Goal linkage

- **Source:** [ADR094](../../../docs/ADR094-coding-agent-mcp.md) D8, D9 (completion push), D10 phase 4.
- **Goal linkage:** Pillar: AI-native coding agents with priced, capped usage (ADR091).
- **Expected outcome:** Callers react to events and bound per-task spend.
- **Why now:** t001 is independent; t002/t003 are gated on `w10/m14` t004–t005 (cost meter, caps), so this milestone is last in the ADR094 sequence.
- **Render parity:** included as a closing task, checked against Devin (the reference product for cloud coding-agent surfaces, ADR047), because this milestone changes REST/GraphQL/MCP/dashboard-facing behavior.
