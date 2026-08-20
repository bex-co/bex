# w8 · m29 — bex CLI help chrome: strip Render branding without forking

**Worker:** worker8 **Goal:** Make `bex --help` and related cobra chrome read as Bex while still importing the pinned `render-oss/cli` command tree — no fork, no vendoring, no OAuth/client-id change. **Status:** done

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Overlay `RootCmd` Use/examples + help template + `cliVersion` so usage says `bex` — **DONE**              | 45m | —          |
| t002 | Targeted Short/Long branding pass (safe replacements; preserve `render.yaml`) — **DONE**                  | 45m | t001       |
| t003 | Retarget `docs` + soften `bex -v` product line — **DONE**                                                 | 30m | t001       |
| t004 | Document branding overlay + pin-bump re-diff checklist; name residual runtime Render copy — **DONE**      | 30m | t002, t003 |
| t005 | Simplify — **DONE**                                                                                       | 30m | t004       |
| t006 | Test coverage — **DONE**                                                                                  | 45m | t005       |
| t007 | Closeout — **DONE**                                                                                       | 15m | t006       |

## Definition of done

With no `RENDER_*` override, `bex --help` (and representative subcommand `--help`) show `bex` as the command path, Bex-oriented Short/Long/examples where safely rewritten, and help chrome that no longer prints `Render CLI v…` as the product label. `bex docs` opens a Bex destination (not render.com/docs). `bex -v` leads with bex's release identity without leading as a Render binary. Hermetic launcher tests cover the overlay. `docs/bex-cli.md` and `lego/cli/UPSTREAM_RENDER_CLI.md` record what changed, what remains upstream-only (login TUI, `run \`render login\``, User-Agent prefix, skills state path), and that pin bumps must re-diff the overlay. No upstream command implementation is copied; no fork/`replace` of `render-oss/cli`; OAuth client id and `cfg.Version` User-Agent truthfulness stay unchanged.

## Source + Goal linkage

- **Source:** User research handoff 2026-08-19 — review of bex CLI customizations over Render CLI and how to further remove Render branding while keeping bex; explicit handoff to `/pm` for w8.
- **Goal linkage:** Product identity for the shipped `bex` binary (`docs/bex-cli.md`, ADR058, `lego/cli/`); users already install `bex` and authenticate through Bex-branded dashboard device pages (`w10/m7`), but help/TUI chrome still says Render.
- **Expected outcome:** Day-to-day `bex --help` / examples / docs / version feel like Bex. Residual hard-coded runtime strings remain documented as accepted until an explicit upstream-hook or fork decision.
- **Why now:** Research mapped a clear Layer-1 embedder seam (`cmd.RootCmd` is exported) that does not conflict with DO_NOT_DO's no-fork rule; Layer-2/3 (login TUI, `ErrLogin`, User-Agent prefix, RepoURL consts) need fork or a reopened upstream decision and stay out of this milestone.
- **Render parity:** Omitted — no REST/GraphQL/MCP or dashboard contract change; CLI help-chrome overlay and docs only. Server-side CLI compatibility remains `docs/cli-compatibility-checklist.md`.
