# w8 · m33 — Upstream Render drift re-baseline round 3 + CLI pin v2.26.0

**Worker:** worker8 **Goal:** the Render-parity ledger, MCP-server tip, and CLI compatibility checklist are current at fresh upstream pins, with every newly surfaced gap filed as owned board work **Status:** todo

## Tasks (in order)

| id   | title                                                                       | est | depends_on       |
| ---- | --------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Diff Render's live OpenAPI against the pinned 208-op baseline               | 45m | —                |
| t002 | Sweep Render changelog + render-mcp-server tip since the 2026-08-18/19 pins | 30m | —                |
| t003 | Move the CLI pin v2.24.0 → v2.26.0 and re-run the graded compat walk        | 60m | —                |
| t004 | Re-baseline ADR018 + the CLI checklist with dated evidence; file new gaps   | 45m | t001, t002, t003 |
| t005 | Simplify: `/simplify` over the code this milestone changed                  | 30m | t004             |
| t006 | Test coverage: meaningful tests for the behavior this milestone shipped     | 30m | t004             |
| t007 | Closeout: verify DoD, mark done, move milestone to done/                    | 15m | t006             |

## Definition of done

`docs/ADR018-render-parity.md` and `docs/cli-compatibility-checklist.md` carry a dated round-3 re-baseline: the live OpenAPI diff, the changelog/MCP-tip sweep, and the CLI pin at v2.26.0 with a green `scripts/cli-compat.sh verify` run and the pin-conditional checklist bullets re-checked. Every gap the round surfaces has an owned board item (note or milestone) — nothing silently accepted, no ADR018 `—` rows reopened. Minting the `bex-cli/v*` release is `/release`'s job and out of scope.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w8` 2026-09-07 #1, following the `w2/m79` re-baseline precedent (2026-08-19). Live release check: upstream `render-oss/cli` shipped v2.25.0 (2026-08-27) and v2.26.0 (2026-09-01) past the pinned v2.24.0.
- **Goal linkage:** Render parity is the core product promise (ADR006/ADR018); the checklist explicitly says to re-check its pin-conditional bullets "when the CLI pin moves."
- **Expected outcome:** parity claims are current instead of three weeks stale; the CLI launcher tracks the newest upstream release (also silencing the `bex login` TUI upgrade banner that appears whenever the pin lags).
- **Why now:** drift compounds — the previous round found four material row changes; the pin is measurably two releases behind.
- **Render parity closing task omitted:** this milestone _is_ the cross-surface parity audit; any drift it finds becomes filed follow-up work rather than a closing checkbox.
