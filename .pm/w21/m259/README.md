# w21 · m259 — Cloud Agents MCP phase 3: schema-validated structured output (`submit_result`)

**Worker:** worker21 **Goal:** A calling agent gets `{tests_passed, files_changed, risk}`-style machine-readable results from a cloud agent turn instead of parsing prose. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| -- | ----- | --- | ---------- |
| t001 | `outputSchema` / `outputRequired` inputs, validation limits, turn columns | 1.5h | w21/m258/t001 |
| t002 | Internal `/v1/agent-session-results` gateway verb | 1.5h | t001 |
| t003 | `submit_result` tool in the in-sandbox `bex` MCP server + platform overlay paragraph | 2h | t002 |
| t004 | Missing-result handling: one bounded reminder, then `STRUCTURED_OUTPUT_MISSING` | 1.5h | t003 |
| t005 | Expose `outcome.result` and turn result; confirm repo-less sessions | 1h | t001, w21/m257/t003 |
| t006 | Render parity (vs Devin): REST, GraphQL, MCP and dashboard consistency | 30m | t001, t002, t003, t004, t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 45m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

A turn created with `outputSchema` ends with a validated `outcome.result` or fails with `STRUCTURED_OUTPUT_MISSING`; bad schemas fail with `AGENT_SESSION_OUTPUT_SCHEMA_INVALID`; works for repo-less sessions; verified by backend/driver tests and one live session on `dev-21`.

## Source + Goal linkage

- **Source:** [ADR094](../../../docs/ADR094-coding-agent-mcp.md) D5, D7 (`result`), D10 phase 3.
- **Goal linkage:** Pillar: AI-native coding agents (ADR008); matches Devin's `structured_output_schema`.
- **Expected outcome:** Callers consume typed results from cloud-agent turns.
- **Why now:** Needs m257 `outcome` and the m258 inputs; unlocks orchestrator use cases that need typed results.
- **Render parity:** included as a closing task, checked against Devin (the reference product for cloud coding-agent surfaces, ADR047), because this milestone changes REST/GraphQL/MCP/dashboard-facing behavior.
