# w2 · m263 — Sandbox flags the Render CLI v2.35 sends: honor or refuse each by name

**Worker:** worker2 **Goal:** every sandbox flag the pinned Render CLI v2.35.0 sends behaves as on render.com, or is refused by name, instead of a generic `unknown field` 400 or a silently ignored filter. **Status:** todo

## Tasks (in order)

| id   | title                                                              | est | depends_on                   |
| ---- | ------------------------------------------------------------------ | --- | ---------------------------- |
| t001 | `idleTimeoutSeconds` on sandbox create and detail                  | 1h  | —                            |
| t002 | `--timeout=0` creates a sandbox with no timeout                    | 30m | —                            |
| t003 | Sandbox plan names `1c-2g` … `8c-16g`                              | 45m | —                            |
| t004 | Named refusals for sandbox groups and images on create and list    | 45m | —                            |
| t005 | `ea blueprints` next-step hints name `bex`, not `render`           | 45m | —                            |
| t006 | Render parity                                                      | 15m | t001, t002, t003, t004, t005 |
| t007 | Simplify                                                           | 15m | t006                         |
| t008 | Test coverage                                                      | 30m | t006                         |
| t009 | Closeout                                                           | 10m | t007, t008                   |

## Definition of done

- With the bex CLI at v2.35.0 (w2/m256), `bex sandboxes create --idle-timeout N`, `--timeout=0`, and `--plan 1c-2g` each do what Render documents, and detail output shows the real idle timeout.
- `--group`/`--sandbox-group` on create and list, `--image` on create, and `sandbox-groups create` / `images import|get` answer a 400 or 404 that names the unsupported feature, unless sandbox groups ship. A list never silently ignores a filter.
- `bex ea blueprints create|plan|approve|update` next steps, in both text output and JSON/YAML `meta.next`, point at `bex`. Otherwise ADR018 records the residual with the user's sign-off.
- REST, GraphQL, MCP and ADR018 agree.

## Source + Goal linkage

- **Source:** the w2/m256 pin bump re-diff (2026-11-04), which compared the v2.29.0 and v2.35.0 upstream sources and spot-checked the built launcher. Upstream CHANGELOG entries: v2.31.0 `--idle-timeout`; v2.32.0 `--group`/`--sandbox-group`, `sandbox-groups create`, `--wait`; v2.34.0 `--image`, `images import|get`, `--timeout=0` = no timeout. Server side: `lego/backend/internal/sandbox/rest.go` decodes strictly, so these fields 400 as `unknown field`; `sandboxGroupId` on list is ignored; plans accept only `starter|standard|pro`; timeout 0 maps to the 86400 s cap (w5/m99).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first). The Render CLI is bex's CLI (DO_NOT_DO: no first-party CLI), so gaps the CLI exposes are bex-api work.
- **Expected outcome:** customers on the v2.35 bex CLI never see an opaque `unknown field` error. Each sandbox flag works or explains itself, and agents following `meta.next` stay on bex.
- **Why now:** shipping m256 puts these flags in every customer's `bex sandboxes --help`. Before the bump the pinned CLI (v2.29.0) could not send them.
- **Render parity included:** REST/MCP sandbox surface and CLI output change.
