# w5 · m600 — MCP input-schema parity: emit Render's 21 pinned argument enums, type `any` union args, clean leaked `jsonschema` tags, and unblock `envVars[].generateValue`

**Worker:** worker5 **Goal:** bring every shared Render MCP tool's advertised `inputSchema` into parity with `render-oss/render-mcp-server@a18ef3ee` by emitting JSON Schema `enum` constraints on all 21 pinned enum-constrained arguments (preserving documented bex extension values as strict supersets), replacing untyped `{}` schemas on `create_postgres.version` and `trigger_deploy.clearCache` with explicit union types, cleaning leaked `google/jsonschema-go` tag tokens from tool descriptions, unblocking `envVars[].generateValue` in MCP schema validation, and locking all four guarantees in `TestMCPArgumentTypesMatchUpstream` and schema hygiene tests. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Emit JSON Schema `enum` on all 21 pinned shared-tool arguments and explicit union types on `create_postgres.version` and `trigger_deploy.clearCache` — **DONE** | 1h15m | — |
| [t002](done/t002.md) | Clean leaked `jsonschema` tag directives from tool descriptions and make `envVarInput.Value` optional in MCP schema so `generateValue: true` passes validation — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Harden `mcpArgTypeMismatches` and MCP schema guards to check empty types, superset enums, and leaked tag tokens — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Add composed-server behavioral tests and update `ADR018` § MCP parity — **DONE** | 30m | t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 25m | t005, t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 10m | t007 |

Total: **8 tasks, about 4h35m**.

## Definition of done

1. `tools/list` on the composed MCP server emits JSON Schema `"enum"` on all 21 pinned enum-constrained arguments across the 9 shared Render tools (`create_blueprint`, `create_cron_job`, `create_key_value`, `create_postgres`, `create_static_site`, `create_web_service`, `get_metrics`, `list_log_label_values`, `list_logs`), where each emitted enum is a superset of upstream's pinned values (`bexEnum ⊇ upstreamEnum`) and preserves documented bex extension values (`runtime: "image"`, `list_log_label_values.label: "build"`, accepted plan aliases).
2. `create_postgres.version` and `trigger_deploy.clearCache` advertise explicit JSON Schema union types (`number`/`string` and `boolean`/`string` respectively) instead of untyped `{}`.
3. Zero MCP tool properties leak `,enum=` or `required,` tag directives into `tools/list` descriptions, and `{key: "SECRET", generateValue: true}` passes MCP SDK input validation on `create_web_service`, `create_cron_job`, and `create_static_site` (while `{key: "SECRET"}` with neither `value` nor `generateValue` and `{key: "SECRET", value: "x", generateValue: true}` continue to fail in `toEnvVars`).
4. `TestMCPArgumentTypesMatchUpstream` fails if any shared tool arg drops its `"type"` (`len(have) == 0`) or omits any pinned upstream `"enum"` value, and a schema-hygiene test fails if any tool description leaks `jsonschema` tag directives.
5. `cd lego/backend && go test ./...` (including the real-DB suite) and `make lint` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity in backend for w5` (2026-11-23, round 12 item 1), verified against `render-oss/render-mcp-server@a18ef3ee` and `main@7c27a28992`: `lego/backend/internal/api/openapi/render-mcp-tools.json`, `lego/backend/internal/api/mcp_parity.go:155-171`, `lego/backend/internal/apps/blueprint_onboarding.go:862`, `lego/backend/internal/apps/render.go:34-43`, `lego/backend/internal/events/mcp.go:94`, `lego/backend/internal/webhooks/mcp.go:63-65`, `lego/backend/internal/postgres/mcp.go:65`, `lego/backend/internal/deploys/mcp.go:76`.
- **Goal linkage:** Advances Render MCP parity ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first)) and closes schema-level gaps under [ADR018 § Types and behavior hints](../../../../docs/ADR018-render-parity.md).
- **Expected outcome:** MCP clients and agents inspecting `tools/list` see Render's exact enum choices and union types without trial-and-error 400s, leaked struct-tag artifacts disappear from descriptions, `envVars[].generateValue` works over MCP as it already does over REST, and future schema regressions fail CI.
- **Note on `generateValue`:** Upstream `render-oss/render-mcp-server@a18ef3ee` `envVars` only declares `key` and `value`; `generateValue` is a documented bex extension matching Render's REST `POST /v1/services` `envVarInput` oneOf (`apps/render.go:37-43`). Making `Value` optional in MCP JSON Schema repairs internal REST↔MCP schema-vs-handler parity while keeping `{key, value}` calls 100% compatible with Render.
- **Render parity:** Included (`t005`). Touches MCP input schemas across shared Render tools and updates `ADR018`.
