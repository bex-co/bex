# w32 · m297 — Sandbox environment variables, as on Render

**Worker:** worker32 **Goal:** `bex ea sandboxes create --env-var K=V` and the SDK's `create(env=…)` inject variables into the sandbox, write-only, as on Render **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design: storage, reserved names and limits, checked against render.com | 30m | — |
| t002 | Accept `env` on REST create, GraphQL and MCP; validate names and limits | 60m | t001 |
| t003 | Pass env to OpenSandbox and keep values off every read, log and event | 40m | t002 |
| t004 | Production check: env probe in the isolation matrix | 40m | t003 |
| t005 | Docs: parity ledger, ea-sandbox and checklists | 20m | t004 |
| t006 | Render parity | 20m | t005 |
| t007 | Simplify | 15m | t006 |
| t008 | Test coverage | 30m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- On production, `bex ea sandboxes create --env-var K=V` succeeds, and `bex ea sandboxes exec <id> -- printenv K` prints `V`.
- REST/GraphQL/MCP reads never return the value.
- A reserved or invalid name is refused by name (`400 SANDBOX_ENV_INVALID`), naming only the variable.
- The isolation matrix on production is green, including the env probe.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w32`, 2026-11-09, item 1. Render OpenAPI `POST /sandboxes` `env`; bex refuses it as `SANDBOX_ENV_UNSUPPORTED` (`lego/backend/internal/sandbox/service.go:281`, ADR018 sandbox row `env`); w9/067 closed the refusal's wording only.
- **Goal linkage:** Pillar 5 sandboxes and [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** The last everyday create flag the pinned CLI and SDK still refuse works, so Render sandbox scripts that pass env run unchanged.
- **Why now:** The create plumbing already carries env, and m283 established the isolation matrix as a production verification path that doesn't depend on the broken QA login (m284's gate).
- **Render parity:** Included: REST/GraphQL/MCP create surfaces change. Dashboard N/A (bex has no sandbox page).
