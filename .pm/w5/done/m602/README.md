# w5 · m602 — Full MCP behavior-hint parity (`idempotentHint`, `openWorldHint`, `readOnlyHint`, `destructiveHint`) guarded across all 28 shared tools and extensions

**Worker:** worker5 **Goal:** extend `TestMCPAnnotationsMatchUpstream` to guard all four MCP behavior hints (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`) on all 28 shared Render tools (narrowing `list_workspaces`'s exemption to `readOnlyHint` alone), and audit and fix misclassified extension tools that currently fall through `mcputil.DefaultAnnotations` to `DestructiveHint: true, IdempotentHint: false` (`verify_custom_domain`, `attach_agent_session`, `archive_agent_session`, `unarchive_agent_session`, and idempotent state-setters). **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Extend `TestMCPAnnotationsMatchUpstream` to guard `idempotentHint` and `openWorldHint` alongside `readOnlyHint`/`destructiveHint`, narrowing `list_workspaces`'s exemption to `readOnlyHint` only — **DONE** | 45m | — |
| [t002](done/t002.md) | Audit extension tools falling through `mcputil.DefaultAnnotations` and fix non-destructive and idempotent tool hints — **DONE** | 1h | t001 |
| [t003](done/t003.md) | Add regression tests in `internal/api` and `internal/mcputil` and update `docs/ADR018-render-parity.md` — **DONE** | 30m | t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 15m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 25m | t004, t005 |
| [t007](done/t007.md) | Closeout — **DONE** | 10m | t006 |

Total: **7 tasks, about 3h20m**.

## Definition of done

1. `TestMCPAnnotationsMatchUpstream` (`lego/backend/internal/api/mcp_parity_test.go`) asserts all four pinned behavior hints (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`) whenever `render-mcp-tools.json` sets them on any of the 28 shared Render tools.
2. `list_workspaces`'s accepted divergence is narrowed to `readOnlyHint` only (its `destructiveHint: false`, `idempotentHint: true`, and `openWorldHint: false` remain checked against upstream).
3. Non-destructive / read-like and idempotent extension tools currently misclassified as `DestructiveHint: true, IdempotentHint: false` (`verify_custom_domain`, `attach_agent_session`, `archive_agent_session`, `unarchive_agent_session`, and audited idempotent state-setters) carry accurate behavior hints without disturbing already-annotated tools (`wait_agent_sessions`, `pause/resume_agent_automation`, `cancel_cron_job_run`, `plan_blueprint`, `update_environment_variables`).
4. `cd lego/backend && go test ./...` (including the real-DB suite) and `make lint` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity in backend for w5` (2026-11-23, round 12 item 4), verified against `render-oss/render-mcp-server@a18ef3ee` and `main@7c27a28992`: `lego/backend/internal/api/mcp_parity_test.go:453-489`, `lego/backend/internal/mcputil/mcputil.go:223-236`, `lego/backend/internal/api/openapi/render-mcp-tools.json`, `lego/backend/internal/apps/mcp.go` (`verify_custom_domain`), `lego/backend/internal/agentsessions/mcp.go` (`attach_agent_session`, `archive_agent_session`, `unarchive_agent_session`).
- **Goal linkage:** Advances Render MCP parity ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first)) and completes the annotation guard started in `w5/m145` ([ADR018 § Types and behavior hints](../../../../docs/ADR018-render-parity.md)).
- **Render parity:** Included (`t004`). Guards all four MCP behavior hints against `render-mcp-tools.json` and updates `ADR018`.
