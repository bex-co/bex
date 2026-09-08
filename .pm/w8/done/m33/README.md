# w8 · m33 — Upstream Render drift re-baseline round 3 + CLI pin v2.26.0

**Worker:** worker8 **Goal:** the Render-parity ledger, MCP-server tip, and CLI compatibility checklist are current at fresh upstream pins, with every newly surfaced gap filed as owned board work **Status:** done

## Tasks (in order)

| id   | title                                                                       | est | depends_on       |
| ---- | --------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Diff Render's live OpenAPI against the pinned 208-op baseline — **DONE**     | 45m | —                |
| t002 | Sweep Render changelog + render-mcp-server tip since the 2026-08-18/19 pins — **DONE** | 30m | —                |
| t003 | Move the CLI pin v2.24.0 → v2.26.0 and re-run the graded compat walk — **DONE** | 60m | —                |
| t004 | Re-baseline ADR018 + the CLI checklist with dated evidence; file new gaps — **DONE** | 45m | t001, t002, t003 |
| t005 | Simplify: `/simplify` over the code this milestone changed — **DONE**        | 30m | t004             |
| t006 | Test coverage: meaningful tests for the behavior this milestone shipped — **DONE** | 30m | t004             |
| t007 | Closeout: verify DoD, mark done, move milestone to done/ — **DONE**          | 15m | t006             |

## Definition of done

`docs/ADR018-render-parity.md` and `docs/cli-compatibility-checklist.md` carry a dated round-3 re-baseline: the live OpenAPI diff, the changelog/MCP-tip sweep, and the CLI pin at v2.26.0 with a green `scripts/cli-compat.sh verify` run and the pin-conditional checklist bullets re-checked. Every gap the round surfaces has an owned board item (note or milestone) — nothing silently accepted, no ADR018 `—` rows reopened. Minting the `bex-cli/v*` release is `/release`'s job and out of scope.

## Re-baseline summary (round 3, 2026-09-07)

- **CLI pin:** `render-oss/cli` **v2.24.0 → v2.26.0** (commit `6c0f561f8af9d4a6cfb88f4d1845ffd18cee181a`, module `v1.1.3-0.20260901190744-6c0f561f8af9`). Upstream moved to **Go 1.27** (`85c8c2c`), so `lego/cli/go.mod` + `lego/go.work` moved to `go 1.27.0` (CI-safe: workspace jobs derive Go from `lego/cli/go.mod`; platform suites run `GOWORK=off` on their own 1.26 go.mod). Updated `UPSTREAM_RENDER_CLI.md`, `scripts/bex-cli-{validate,build}.sh`, `lego/CLAUDE.md`, `.github/workflows/go-lint.yml`, README/`docs/bex-cli.md` version strings. `go build/vet/test ./...`, `bash scripts/bex-cli-validate.sh` all green. Seams re-verified: `RootCmd` exported, `CustomHelpTemplate` intact (branding tests green), `isRootVersionRequest` body unchanged, no new top-level commands (only hidden `analytics notice`), no Bex-native name collisions.
- **Launcher fix (in-scope, part of the pin move):** v2.26.0 made upstream usage analytics **opt-out (on by default)**, phoning a stable install id + active workspace to Render's telemetry endpoint. `internal/bridge` now defaults `RENDER_CLI_DISABLE_ANALYTICS=1` (unless the user set that or `DO_NOT_TRACK`) — process config, no fork. Four new hermetic bridge tests; the same change unified the "blank = unset" checks through a small `isSet` helper (t005 simplify).
- **OpenAPI diff (t001):** diffed the v2.26.0 regenerated REST client operation surface (generated from Render's current OpenAPI, upstream #648/#655) vs the w2/m79 208-op baseline. Net: `retrieve-service-outbound-ips` gains a client method (bex already serves it on REST/GraphQL/MCP — w2/023 — so it round-trips; dashboard cell pending w8/010); two early-access sandbox-exec ops + one env-group⇄artifact-source rename (outside target, `—`); request/response drift = Postgres `connectionPool` field (bex accepts on create+update via `resolvePooler`, w2/024) + 23 spec-based compute plans (divergence, filed) + two new pool event types.
- **Changelog + MCP-tip sweep (t002):** CLI v2.25.0/v2.26.0 changelog captured (connection-pool flags, spec plan names, blueprints exit-1, `RENDER_CLI_CONFIG_DIR` guidance, bounded OAuth refresh, analytics opt-out). `render-oss/render-mcp-server` tip `86a9c6f6 → 1cca753`: **contractual tool set unchanged** (`trigger_deploy` present); intervening commits are REST-client regens + Go 1.27 + a change to its own deployment `render.yaml` — none alter the MCP contract bex mirrors.
- **Docs re-baseline (t004):** dated round-3 blocks added to `docs/ADR018-render-parity.md` and `docs/cli-compatibility-checklist.md` (header/launcher record → v2.26.0, new `--connection-pool` rows graded, `--plan` divergence + analytics-disable under Intended divergences, blueprints exit-code note, Service-disk pin-conditional bullet re-checked at v2.26.0). No ADR018 `—` row reopened.
- **Filed gap:** `.pm/w8/011.md` — Render's spec-based compute plan names (`2c-8g` …) 400 against bex's tier catalog; open pricing decision (adopt as aliases vs keep divergence). Every other delta was already covered by prior work (`connectionPool` w2/024, outbound-ips w2/023).
- **Not run (environment limit):** live `scripts/cli-compat.sh verify` — no dev/cluster stack is up in this environment (the documented "dev-9 does not run" carve-out; w2/m79 itself only got a partial dev-2 verify). Round-3 grades are help-graded + hermetic launcher suite (green) + backend wire-shape reads + the `bex-cli-validate.sh` boundary gate (green). Minting the `bex-cli/v*` release is `/release`'s job, out of scope.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w8` 2026-09-07 #1, following the `w2/m79` re-baseline precedent (2026-08-19). Live release check: upstream `render-oss/cli` shipped v2.25.0 (2026-08-27) and v2.26.0 (2026-09-01) past the pinned v2.24.0.
- **Goal linkage:** Render parity is the core product promise (ADR006/ADR018); the checklist explicitly says to re-check its pin-conditional bullets "when the CLI pin moves."
- **Expected outcome:** parity claims are current instead of three weeks stale; the CLI launcher tracks the newest upstream release (also silencing the `bex login` TUI upgrade banner that appears whenever the pin lags).
- **Why now:** drift compounds — the previous round found four material row changes; the pin is measurably two releases behind.
- **Render parity closing task omitted:** this milestone _is_ the cross-surface parity audit; any drift it finds becomes filed follow-up work rather than a closing checkbox.
