# w5 · m145 — bex's MCP tools match Render's contract in argument types and annotations, guarded

**Worker:** worker5 **Goal:** an agent written against Render's MCP contract calls bex without type errors, its client auto-approves read tools, and the parity guard fails when an argument type drifts. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The capture records argument types and tool annotations | 45m | — |
| t002 | Type-compatibility check in the parity guard | 45m | t001 |
| t003 | Fix the drift the guard surfaces | 45m | t002 |
| t004 | Annotations on every tool, required at the seam | 1h | t001 |
| t005 | Inventory update and a real-client check | 30m | t003, t004 |
| t006 | Render parity | 30m | t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 45m | t006, t007 |
| t009 | Closeout | 10m | t008 |

Total: **9 tasks, about 5h 30m**.

## Definition of done

- On dev-5, `trigger_deploy(clearCache: true)` and `create_postgres(version: 16)` succeed over MCP.
- The parity guard fails when an argument type drifts.
- Every tool carries annotations and a real client sees reads as read-only.

## Source + Goal linkage

- **Source:** third `/pm-brainstorm for w5` (2026-10-20), re-offered as the parity-only set and chosen by the user the same day. Driving upstream `render-mcp-server` (pinned `d9a8abd`) over stdio reproduced both type mismatches (`has type "boolean", want "string"`); the guard compares names only. bex's non-test code has no MCP annotations while upstream sets them on all 24 tools, so clients treat `list_services` as a destructive write.
- **Goal linkage:** ADR008 AI-native thesis (pillar 3, MCP); ADR006.
- **Expected outcome:** Render-written agents work on bex without type errors and with auto-approved reads.
- **Why now:** agents are bex's headline surface and both failures pass today's guard silently.
- **Render parity:** included: MCP.
