# w5 · m266 — Key Value Pro, Pro Plus and size-named plans create, as on Render

**Worker:** worker5 **Goal:** a customer can create and upgrade to Key Value stores larger than 1 GB using Render's plan names on every surface. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Verify Render's Key Value plan specs and add the tiers | 45m | — |
| t002 | Operator schedules large Key Value stores onto pools that fit them | 1h30m | t001 |
| t003 | Backend admission and price sheet accept the new plans on REST, GraphQL, MCP and Blueprints | 1h | t001 |
| t004 | Dashboard plan picker and ADR updates | 1h | t003 |
| t005 | Live: create a pro store in production and upgrade standard → pro | 1h | t002, t004 |
| t006 | Dated widen task: 20g and 40g go live once w2/m250's large pools exist | 30m | t005 |
| t007 | Render parity | 30m | t006 |
| t008 | Simplify | 20m | t007 |
| t009 | Test coverage | 45m | t007, t008 |
| t010 | Closeout | 10m | t009 |

Total: **10 tasks, about 7h30m**.

## Definition of done

- Render's `keyValuePlan` values `pro`, `pro_plus`, `5g`, `10g`, `20g`, `40g` are accepted on REST, GraphQL, MCP, Blueprints and the dashboard, with prices.
- A pro store runs and connects in production; a standard → pro upgrade grows its PVC.
- 20g and 40g are verified live on w2/m250's large pools, or their remaining gate is recorded with an owner (and any interim refusal carries the user's ADR018 sign-off).

## Source + Goal linkage

- **Source:** `/pm-brainstorm w5` 2026-11-05 item 2; `lego/types/tiers/tiers.yaml:155`; Render CLI v2.35.0 `KeyValuePlanValues`; ADR018 row 222; Render changelog 2026-10-01 (Key Value plans repriced, 3 GB/50 GB added).
- **Goal linkage:** ADR021 managed Key Value; ADR008 Render parity first.
- **Expected outcome:** Key Value stores above 1 GB exist on bex.
- **Why now:** one of Render's most common upgrade paths is impossible on bex today, and Render just reworked its Key Value ladder.
- **Render parity:** included.
- **Open decision (not yet signed):** whether 20g/40g get a temporary named refusal while w2/m250's pools are blocked; t006 records the outcome.
