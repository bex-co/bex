# w5 · m92 — CLI analytics parity with render.com

**Worker:** worker5 **Goal:** bex-api accepts, attributes, and stores the CLI telemetry events the imported Render CLI already sends, so bex can understand real CLI usage the way render.com does — without forking the CLI. **Status:** done (2026-09-10)

## Tasks (in order)

| id                         | title                                                            | est | depends_on  |
| -------------------------- | ---------------------------------------------------------------- | --- | ----------- |
| [t001](done/t001.md) — **DONE** | Accept `POST /v1/cli-telemetry-events` with Render's schema      | 1h  | —           |
| [t002](done/t002.md) — **DONE** | Auth scope + rate limit + abuse caps for the telemetry endpoint  | 45m | w5/m92/t001 |
| [t003](done/t003.md) — **DONE** | Dedicated telemetry table with attribution and retention         | 1h  | w5/m92/t002 |
| [t004](done/t004.md) — **DONE** | Launcher consent flip + disclosure docs                          | 45m | w5/m92/t003 |
| [t005](done/t005.md) — **DONE** | End-to-end verification against dev-5 bex-api                    | 1h  | w5/m92/t004 |
| [t006](done/t006.md) — **DONE** | Render parity cross-surface check                                | 30m | w5/m92/t005 |
| [t007](done/t007.md) — **DONE** | Simplify                                                         | 30m | w5/m92/t006 |
| [t008](done/t008.md) — **DONE** | Test coverage                                                    | 45m | w5/m92/t007 |
| [t009](done/t009.md) — **DONE** | Closeout                                                         | 15m | w5/m92/t008 |

## Definition of done

POSTing Render's `CliTelemetryEventPOSTInput` shape (`command`, `duration_ms`, `exit_code`, `installation_id`, `current_workspace_id`, …) to a local bex-api returns 2xx and persists an attributed row; a tree-built `bex` with telemetry enabled emits one event per invocation (confirmed via `RENDER_LOG_ANALYTICS=1` plus a table read); `DO_NOT_TRACK` / `BEX_CLI_DISABLE_ANALYTICS` suppress sending; `docs/bex-cli.md` documents the disclosure; the compatibility checklist gains a telemetry row.

## Source + Goal linkage

- **Source:** user request 2026-09-10 ("hand off to /pm for w5 to achieve cli analytics parity with render.com") following session research: the pinned `render-oss/cli` `pkg/analytics` sender POSTs `/cli-telemetry-events` relative to the API base, which our bridge already repoints at bex-api — the client half exists, the server half does not (`lego/backend` has zero telemetry ingestion).
- **Goal linkage:** Render parity ([docs/ADR018-render-parity.md](../../docs/ADR018-render-parity.md)) plus product understanding — per-command usage, failure rates, CI-vs-human split, and version adoption for CLI decisions. This is bex-api work surfaced by the imported CLI, the sanctioned pattern (not a CLI fork — `.pm/DO_NOT_DO.md` stays satisfied).
- **Expected outcome:** bex-api stores attributable CLI usage events; the launcher can later flip consent with a same-shape client already verified.
- **Why now:** `bex-cli/v0.2.1` ships with Render telemetry force-disabled and a hostile one-time notice inviting users to re-enable phoning Render; the server endpoint must exist before any consent flip, and the schema must be pinned before the next upstream re-baseline moves it. Render parity task included because this ships a new REST surface (GraphQL/MCP intentionally out of scope — upstream exposes telemetry over REST only).
