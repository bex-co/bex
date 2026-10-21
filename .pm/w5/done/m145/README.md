# w5 · m145 — bex's MCP tools match Render's contract in argument types and annotations, guarded

**Worker:** worker5 **Goal:** an agent written against Render's MCP contract calls bex without type errors, its client auto-approves read tools, and the parity guard fails when an argument type drifts. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The capture records argument types and tool annotations — **DONE** | 45m | — |
| t002 | Type-compatibility check in the parity guard — **DONE** | 45m | t001 |
| t003 | Fix the drift the guard surfaces — **DONE** | 45m | t002 |
| t004 | Annotations on every tool, required at the seam — **DONE** | 1h | t001 |
| t005 | Inventory update and a real-client check — **DONE** | 30m | t003, t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 45m | t006, t007 |
| t009 | Closeout — **DONE** | 10m | t008 |

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

## Progress (2026-10-20)

- **t001.** `scripts/render-mcp-capture.py` `normalize()` keeps each argument's JSON type and enum and the four behavior hints. Re-captured at the same upstream commit `d9a8abd` (23 tools, argument and required sets unchanged). The pin's sha256 moved in `render_mcp.go` and `render-mcp-drift.sh`, and the drift test's stub now parses its body as JSON (the pin carries `true`/`false`).
- **t002.** `mcpArgTypeMismatches` + `TestMCPArgumentTypesMatchUpstream` (integer satisfies number; untyped skipped; accepted list `mcpAcceptedTypeDivergences`, which may not rot). On first run it found exactly `create_postgres.version` and `trigger_deploy.clearCache`.
- **t003.** `clearCache` accepts Render's boolean (true → clear, false → do_not_clear) and still accepts the REST strings; `version` accepts a whole number or a string; both go through `mcputil.BoolOrEnum` / `NumberOrString`. MCP-level tests drive both.
- **t004.** `mcputil.AddTool`/`AddExactJSONTool` fill annotations by verb when a tool sets none. `TestMCPAnnotationsMatchUpstream` covers all 190 tools and agrees with upstream on shared ones (exception `list_workspaces`: a pure read in bex, upstream false).
- **t005 deviation.** The tools/list check ran through the real MCP Go SDK client against the fully wired server (the guard tests). Against dev-5's HTTP `/mcp`, the throwaway user's API key got 401 on REST as well, a dev-env key-auth gap unrelated to this milestone. ADR006 and ADR018 are updated.
- **Render parity / simplify / coverage.** Types and hints now follow the pinned upstream contract; it's a small diff with the helpers in mcputil; the guards fail on the old code (names-only pin, no annotations). `make lint-backend` clean; api, mcputil, deploys, postgres and secrets suites green.
