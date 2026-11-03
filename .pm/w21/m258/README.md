# w21 · m258 — Cloud Agents MCP phase 2: the `/mcp/cloud-agents` toolset, `wait_agent_sessions`, idempotent create, `bex.cloud_agents` scope

**Worker:** worker21 **Goal:** An orchestrator or CI bot can consent to a narrow scope, connect to `/mcp/cloud-agents`, create a task idempotently, wait for it server-side, and read the outcome. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| -- | ----- | --- | ---------- |
| t001 | Toolset tag, `/mcp/cloud-agents` mount and `api mcp-stdio --toolset cloud-agents` | 2h | w21/m257/t004 |
| t002 | `bex.cloud_agents` capability scope | 1.5h | — |
| t003 | Idempotent create: `idempotencyKey` / `Idempotency-Key` across REST, GraphQL, MCP | 2h | — |
| t004 | Optional server-picked `branch`, `title`, `tags`, and `state`/`tag` list filters | 1.5h | w21/m257/t002 |
| t005 | `wait_agent_sessions`: blocking wait over Postgres LISTEN/NOTIFY with progress notifications | 2h | t001, w21/m257/t002 |
| t006 | Docs: connecting an orchestrator to `/mcp/cloud-agents` (ADR025 section) | 45m | t001, t002, t005 |
| t007 | Load-test held `wait_agent_sessions` requests per replica | 45m | t005 |
| t008 | Render parity (vs Devin): REST, GraphQL, MCP and dashboard consistency | 30m | t001, t002, t003, t004, t005, t006, t007 |
| t009 | Simplify | 20m | t008 |
| t010 | Test coverage | 45m | t008 |
| t011 | Closeout | 10m | t009, t010 |

## Definition of done

`/mcp/cloud-agents` serves exactly the nine-tool toolset plus `list_workspaces`/`list_repos`; a token with only `bex.cloud_agents` runs the create→wait→get loop but cannot touch services; retried creates with one `idempotencyKey` yield one session; `wait_agent_sessions` returns on state change within ~1 s and caps at 25 s; ADR025 documents connecting. Verified with backend tests plus a live run against the `dev-21` stack.

## Source + Goal linkage

- **Source:** [ADR094](../../../docs/ADR094-coding-agent-mcp.md) D1, D2, D3, D4, D6, D9 (scope), D10 phase 2.
- **Goal linkage:** Pillar: AI-native platform / connect-an-agent (ADR025, ADR008).
- **Expected outcome:** One narrow, cheap-to-approve MCP endpoint for delegating coding work.
- **Why now:** Today a delegating agent must load 191 tools, request `bex.write`, poll at token cost and risks duplicate sandboxes on retry; this removes all four problems.
- **Render parity:** included as a closing task, checked against Devin (the reference product for cloud coding-agent surfaces, ADR047), because this milestone changes REST/GraphQL/MCP/dashboard-facing behavior.
