# w5 · m601 — Close the MCP result-shape and `structuredContent` conformance blind spot across all 28 shared Render tools

**Worker:** worker5 **Goal:** pin exact upstream result types for all 6 previously untyped object tools (`get_blueprint`, `create_blueprint`, `plan_blueprint`, `approve_blueprint`, `update_blueprint`, `cancel_cron_job_run`) in `scripts/render-mcp-capture.py` and `render-mcp-results.json`, fix the verified Blueprint result drifts (`get_blueprint` unrecorded keys and `update_blueprint` no-op notice), enforce the `structuredContent` contract in `rendertest.CheckRenderMCPResult` (reconciling `ADR018` with `get_metrics`), and add a composed-server conformance suite covering all 28 shared Render tools through `Server.MCPServer()`. **Status:** open

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Pin exact upstream result types for the 6 untyped Blueprint and cron-cancel tools in `render-mcp-capture.py` and `render-mcp-results.json`, and reconcile `get_blueprint` / `update_blueprint` / `cancel_cron_job_run` shapes | 1h15m | — |
| [t002](t002.md) | Enforce `structuredContent` parity rules in `rendertest.CheckRenderMCPResult` and reconcile `ADR018` with `get_metrics` | 45m | t001 |
| [t003](t003.md) | Add a composed-server MCP result-conformance suite covering all 28 shared Render tools with a suite-wide coverage guard | 1h15m | t002 |
| [t004](t004.md) | Update `docs/ADR018-render-parity.md` § MCP result shapes and verify CI test inputs | 20m | t003 |
| [t005](t005.md) | Render parity | 15m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 25m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

Total: **8 tasks, about 4h35m**.

## Definition of done

1. Every `form: "object"` tool in `lego/backend/internal/api/openapi/render-mcp-results.json` carries a concrete `"type"` captured from `render-oss/render-mcp-server@a18ef3ee` (`get_blueprint` → `BlueprintDetail`, `create_blueprint` → `CreateBlueprintResponse`, `plan_blueprint` → `CreateBlueprintSyncResponse`, `approve_blueprint` → `ApprovedSync`, `update_blueprint` → `UpdateBlueprintResponse`, `cancel_cron_job_run` → `CancelCronJobRunResult`), with zero untyped object tools remaining.
2. `get_blueprint` conforms to `BlueprintDetail` (additive REST-shared extensions `manifest`/`createdAt`/`updatedAt` either stripped or recorded explicitly in `rendertest.MCPResultExtensions` + `ADR018`, consistent with REST `GET /v1/blueprints/{id}`), `cancel_cron_job_run`'s documented `run` extension is recorded in `rendertest.MCPResultExtensions`, and `update_blueprint` appends upstream's no-op text item when no setting changed (`pkg/blueprint/tools.go:179-181`).
3. `rendertest.CheckRenderMCPResult` verifies `StructuredContent == nil` for `array`, `arrayCursor`, and `text` forms (plus `emptyText` answers), and for `object` answers verifies that whenever `StructuredContent != nil`, it is JSON-equal to `Content[0].Text` (and `ADR018:16` accurately documents `get_metrics`'s text-only single/multi-resource exception).
4. A composed-server conformance test in `lego/backend/internal/api/` exercises all 28 shared Render tools through `Server.MCPServer()` and enforces via a suite-wide coverage guard that every tool in `render-mcp-results.json` is tested and every `form: "object"` tool declares a non-empty `type`.
5. `cd lego/backend && go test ./...` (including the real-DB suite) and `make lint` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity in backend for w5` (2026-11-23, round 12 item 2), verified against `render-oss/render-mcp-server@a18ef3ee` (`pkg/blueprint/tools.go`, `pkg/blueprint/update.go`, `pkg/client/blueprints/blueprints_gen.go`, `pkg/service/tools.go:876`) and `main@7c27a28992`: `scripts/render-mcp-capture.py:168-230`, `lego/backend/internal/api/openapi/render-mcp-results.json`, `lego/backend/internal/rendertest/mcpresult.go:116-155`, `lego/backend/internal/apps/blueprint.go:495-516`, `lego/backend/internal/apps/blueprint_onboarding.go:1006-1009`, `lego/backend/internal/metrics/mcp_render_test.go:37`, `docs/ADR018-render-parity.md:16`.
- **Goal linkage:** Advances Render MCP parity ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first)) and completes the result-shape contract introduced in `w4/m378` ([ADR018 § MCP result shapes](../../../docs/ADR018-render-parity.md)).
- **Recommendation (not a user decision):** For `get_blueprint`, keeping the REST-shared `manifest`, `createdAt`, `updatedAt` keys recorded in `rendertest.MCPResultExtensions` (matching how `Service`, `Deploy`, `Postgres`, and `KeyValue` record REST-shared additive keys on MCP in `ADR018:18`) OR stripping `manifest` if REST omits it — verify REST `GET /v1/blueprints/{id}` first so MCP and REST stay identical, following ADR008 Render parity first.
- **Render parity:** Included (`t005`). Tightens MCP result-shape conformance across all 28 shared tools and updates `ADR018`.
