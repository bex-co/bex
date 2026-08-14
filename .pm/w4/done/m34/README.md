# w4 · m34 — bex CLI self-version + bex-owned update notice

**Worker:** worker4 **Goal:** `bex -v` reports bex's own release identity and checks bex's release channel — never Render's — and users of the growing Bex-native command surface (`bex glm`/`muse`/`kimi`/`deepseek`, `bex code`) get a gh-style passive update notice pointing at `bex-cli/v*` releases. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Intercept root version requests in `cli/main.go` (own the `-v`/`--version` path) — **DONE** | 45m | — |
| t002 | Inject a bex-native release version via ldflags (build script + release workflow) — **DONE** | 30m | t001 |
| t003 | gh-style passive update notice against bex's `bex-cli/v*` releases — **DONE** | 45m | t002 |
| t004 | Upstream PR: make `cfg.RepoURL` overridable; record the login-view limitation — **DONE** (PR opened, then closed + branch deleted by user decision 2026-08-15 — do not reopen without an explicit user decision; pin-bump is the standing mitigation) | 45m | t003 |
| t005 | Simplify — `/simplify` over the code this milestone changed — **DONE** | 30m | t004 |
| t006 | Test coverage — version interception, notice gating, cache behavior — **DONE** | 45m | t005 |
| t007 | Closeout — **DONE** | 15m | t006 |

## Definition of done

`bex -v` (and `bex --version`) prints bex's own release version (with the pinned upstream compatibility version alongside) and its update check queries this repo's `bex-cli/v*` releases — the upstream render-oss check never runs on the version path. `cli-release.yml`-built binaries carry the tag-derived bex version; `cfg.Version` (the User-Agent) stays at the pinned upstream release. The passive notice appears at most once per 24h, only on a TTY, never in CI, and is disabled by `BEX_NO_UPDATE_NOTIFIER`; a check failure is silent. `cli/UPSTREAM_RENDER_CLI.md` records the `pkg/tui/views/login.go` second call site with pin-bumping as the standing mitigation (the upstream PR was opened and then withdrawn by user decision 2026-08-15).

## Source + Goal linkage

- **Source:** 2026-08-14 session — CLI update-channel research (gh notice model, go-selfupdate landscape) after shipping the `bex glm`/`muse`/`kimi`/`deepseek` Claude Code launchers; local source inspection of the pinned upstream `cmd/root.go` (`printVersionWithUpdateCheck`, const `cfg.RepoURL` → render-oss/cli) and `pkg/tui/views/login.go` (second, non-interceptable call site).
- **Goal linkage:** the bex CLI is a first-class product surface (docs/bex-cli.md, docs/cli-compatibility-checklist.md); a CLI that mis-directs its own users to another vendor's upgrade page breaks product trust, and the new Bex-native coding launchers make CLI freshness (catalog + binary) matter.
- **Expected outcome:** the moment Render ships a release newer than the 2.22.0 pin, bex users see nothing wrong — `bex -v` reports bex's identity and bex's channel; bex releases become discoverable in-product via the passive notice.
- **Why now:** the misdirection is latent-but-armed — it fires on Render's next release, outside our control; and this session's launcher work will drive `bex-cli/v*` release cadence, which needs a working discovery channel. Render parity closing task omitted: CLI distribution infra only — no REST/GraphQL/MCP or dashboard surface change.
