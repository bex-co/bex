# w2 · m533 — Cross-surface parity: GraphQL, MCP tools & Dashboard UI for sandbox group credentials

**Worker:** worker2 **Goal:** Expose all 7 sandbox group credential operations consistently across GraphQL, MCP, and the Dashboard UI over `w2/m532`'s shared write-only core, with identical split Admin/Developer RBAC, scoped API key permissions, breakpoint-matched skeletons, and full localization. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add GraphQL types, queries, and mutations for sandbox group credentials and versions over `w2/m532`'s shared `sandbox.Service`, plus `key_permission_ops.go` entries — **DONE** | 1h | `w2/m532` |
| t002 | Add the 7 `Extension` MCP tools, behavior hints, `key_permission_ops.go`, and `TestMCPParityInventory` pin — **DONE** | 1h | t001 |
| t003 | Add cross-surface parity tests asserting identical fields, split RBAC, scoped-key enforcement, write-only secrets, and coded errors across REST, GraphQL, and MCP — **DONE** | 45m | t002 |
| t004 | Build Dashboard Sandbox Group Credentials management UI (list, create/edit form, enabled toggle, version history + restore, role-aware Admin vs Developer controls) — **DONE** | 1h30m | t001 |
| t005 | Add breakpoint-matched preloading skeletons (desktop + narrow mobile), all dashboard locale strings, and Vitest component/hook coverage — **DONE** | 45m | t004 |
| t006 | Render parity — **DONE** | 20m | t003, t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t006 |
| t009 | Closeout — **DONE** | 10m | t007, t008 |

## Definition of done

- GraphQL and MCP expose all 7 sandbox-group-credential operations (`list`, `get`, `versions`, `create`, `update`, `delete`, `restore`) over the same `sandbox.Service` core with identical fields, split Admin/Developer RBAC, `KeySandboxesHistory`/`KeySandboxesCredentials` scoped-key enforcement, and coded errors; `TestMCPParityInventory` and `TestMCPToolsNeverCallBexRender` pass.
- No GraphQL query, MCP tool output, or dashboard component ever returns, caches, or logs a stored secret value.
- In the dashboard, Admins can create, edit (including `domain` and replacement `value`), toggle `enabled`, inspect version history, restore a retained version, and delete credentials; Developers can edit `injection`/`methods`/`paths`/`enabled` only; Viewers see read-only metadata. Pending skeletons match the ready layout at desktop and narrow-mobile (`390px`) widths, and `dashboard` `yarn lint` + `yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w2` (2026-11-18), materialized via `/pmx them all to w2`. Follows `w2/m532` under [ADR006-bex-api.md](../../../../docs/ADR006-bex-api.md) ("one Core, three adapters") and [ADR104 D1](../../../../docs/ADR104-sandbox-render-parity.md#d1--the-parity-target-is-renders-documented-contract-graded-row-by-row).
- **Goal linkage:** Render parity + AI-native cross-surface discipline ([ADR008](../../../../docs/ADR008-vision.md)) — agents managing sandboxes over MCP and humans managing groups in the dashboard share the exact same capability and security posture as REST/CLI.
- **Expected outcome:** Users and agents can configure, inspect, roll back, and disable sandbox group credentials from GraphQL, MCP, and the dashboard without dropping to raw REST.
- **Why now:** Immediately follows `w2/m532` so sandbox group credentials never ship as a REST-only island that drifts across adapters.
- **Render parity:** Included — adds GraphQL, MCP, and Dashboard UI surfaces.
- **Depends on:** `w2/m532`.
- **Brainstorm recommendations, not user decisions:** Render's public docs cover REST/CLI credential management and do not publish a Dashboard UI layout or MCP tool set for sandbox group credentials; shipping the 7 `Extension` MCP tools and a role-aware Dashboard Credentials panel follows bex's established sandbox-group and snapshot precedent (`w22/m311`, `w32/m285`) under ADR006/ADR008.
