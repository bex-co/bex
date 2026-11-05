# w21 · m259 — Cloud Agents MCP phase 3: schema-validated structured output (`submit_result`)

**Worker:** worker21 **Goal:** A calling agent gets `{tests_passed, files_changed, risk}`-style machine-readable results from a cloud agent turn instead of parsing prose. **Status:** done (2026-11-05)

## Tasks (in order)

| id | title | est | depends_on |
| -- | ----- | --- | ---------- |
| t001 | `outputSchema` / `outputRequired` inputs, validation limits, turn columns — **DONE** | 1.5h | w21/m258/t001 |
| t002 | Internal `/v1/agent-session-results` gateway verb — **DONE** | 1.5h | t001 |
| t003 | `submit_result` tool in the in-sandbox `bex` MCP server + platform overlay paragraph — **DONE** | 2h | t002 |
| t004 | Missing-result handling: one bounded reminder, then `STRUCTURED_OUTPUT_MISSING` — **DONE** | 1.5h | t003 |
| t005 | Expose `outcome.result` and turn result; confirm repo-less sessions — **DONE** | 1h | t001, w21/m257/t003 |
| t006 | Render parity (vs Devin): REST, GraphQL, MCP and dashboard consistency — **DONE** | 30m | t001, t002, t003, t004, t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 45m | t006 |
| t009 | Closeout — **DONE** | 10m | t007, t008 |

## Definition of done

A turn created with `outputSchema` ends with a validated `outcome.result` or fails with `STRUCTURED_OUTPUT_MISSING`; bad schemas fail with `AGENT_SESSION_OUTPUT_SCHEMA_INVALID`; works for repo-less sessions; verified by backend/driver tests and one live session on `dev-21`.

## Source + Goal linkage

- **Source:** [ADR094](../../../../docs/ADR094-coding-agent-mcp.md) D5, D7 (`result`), D10 phase 3.
- **Goal linkage:** Pillar: AI-native coding agents (ADR008); matches Devin's `structured_output_schema`.
- **Expected outcome:** Callers consume typed results from cloud-agent turns.
- **Why now:** Needs m257 `outcome` and the m258 inputs; unlocks orchestrator use cases that need typed results.
- **Render parity:** included as a closing task, checked against Devin (the reference product for cloud coding-agent surfaces, ADR047), because this milestone changes REST/GraphQL/MCP/dashboard-facing behavior.

## Completion record

- Shipped 2026-11-05: migration 0200 (`output_schema`, `output_required`, `result`, `result_final` on `agent_session_turns`); `outputSchema`/`outputRequired` on create and send-message across REST, GraphQL (JSON text) and MCP with `400 AGENT_SESSION_OUTPUT_SCHEMA_INVALID`; the `/agent-results` gateway route and bex-api `/v1/agent-session-results` verb (own MAC domain, schema re-validated server-side, last valid wins); the driver's `submit_result` tool, schema-only overlay paragraph and one bounded reminder; `STRUCTURED_OUTPUT_MISSING` decided by the Completer from what bex-api stored; `outcome.result` and transcript-turn `result`/`resultFinal`/`outputSchema`/`outputRequired`; repo-less sessions covered.
- Deviations from ADR094 D5, recorded there ("As built"): two extra turn columns (`output_required`, `result_final`); `outcome.result` is final-only and the partial value is on the transcript turn; the missing-result verdict is made by bex-api, not the driver; GraphQL carries the schema and result as JSON text.
- Parity (Devin): see `w21/m259/done/t006.md`; the dashboard gap is `w21/003`.
- NOT done: the live structured-output session. `scripts/dev-env.sh` supports N 1-10, so no `dev-21` harness exists; the definition of done's live check is filed as `w21/004`. Everything else was verified by Go (real Postgres/OpenFGA suite), gateway and driver tests.
