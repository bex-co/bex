# w5 · m283 — MCP Blueprint tools match Render's: create_blueprint plans, approve_blueprint applies

**Worker:** worker5 **Goal:** an agent that calls Render's Blueprint MCP tools on bex gets Render's semantics — `create_blueprint` plans without applying, `approve_blueprint` applies — with Render's tool names and arguments. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Re-pin Render's MCP tool set and diff it | 40m | — |
| t002 | `create_blueprint` plans without applying, with Render's arguments | 1h | t001 |
| t003 | Add `plan_blueprint`, `approve_blueprint`, `get_blueprint`, `update_blueprint` | 1h20m | t002 |
| t004 | Retire or alias the bex-only planning tools and update the guards | 40m | t003 |
| t005 | Live walk on dev-5: create → plan → approve through MCP | 30m | t004 |
| t006 | Render parity | 30m | t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 45m | t006, t007 |
| t009 | Closeout | 10m | t008 |

Total: **9 tasks, about 5h55m**.

## Definition of done

- On MCP, `create_blueprint` (Render's arguments) creates a Blueprint and a plan but no resources; `approve_blueprint` then applies it, including `manualEnvVars`.
- `plan_blueprint`, `get_blueprint` and `update_blueprint` exist with Render's names, arguments and annotations; the pinned contract guard covers all five.
- No bex MCP tool shares a Render tool name with different semantics; ADR018 records the re-pin date and upstream commit.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` round 7 item 1 (2026-11-07); Render MCP server #205/#218/#221/#222/#224/#230.
- **Goal linkage:** Render parity first (ADR008 § Render parity first) and AI-native (ADR008): MCP is how agents drive bex.
- **Expected outcome:** an agent trained on Render's tools can onboard a Blueprint on bex without accidentally applying it.
- **Why now:** bex's `create_blueprint` currently applies the whole stack — the opposite of Render's same-named tool — so a Render-style call creates (and bills) resources. w2/m256 already built the planning verbs, so this is wiring plus a re-pin.
- **Render parity:** included (MCP surface change; REST/GraphQL consistency checked).
