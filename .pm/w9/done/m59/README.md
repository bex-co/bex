# w9 · m59 — Native `bex` CLI launcher

**Worker:** worker9 **Goal:** Ship a `bex` executable that imports the pinned upstream Render CLI, targets bex-api by default, and keeps its authenticated local state isolated under `~/.bex/cli.yaml`. **Status:** done (released `bex-cli/v0.1.0` 2026-08-15 — 4 platform binaries + checksums + sigstore signature)

## Tasks (in order)

| id   | title | est | depends_on |
| ---- | ----- | --- | ---------- |
| t001 | Scaffold the import-based `bex` CLI module and configuration bridge — **DONE** | 45m | — |
| t002 | Prove browser device login, refresh, logout, and Bex config isolation — **DONE** | 45m | t001 |
| t003 | Package and document installation plus automation credentials — **DONE** | 45m | t001 |
| t004 | Add an upstream-version and end-to-end compatibility release gate — **DONE** | 45m | t001, t002, t003 |
| t005 | Simplify — **DONE** | 30m | t004 |
| t006 | Test coverage — **DONE** | 45m | t005 |
| t007 | Closeout — **DONE** | 15m | t006 |

## Definition of done

A released `bex` binary imports a pinned `render-oss/cli` dependency rather than copying or forking its command implementation. With no `RENDER_*` configuration, `bex login` targets the configured Bex API/device-verification flow, stores its access/refresh-token state only in `~/.bex/cli.yaml` (mode `0600`), refreshes and revokes successfully, and `bex services -o json` reaches bex-api. Installation, CI authentication, supported BEX environment variables, and the intentionally retained upstream Render-branding limitations are documented. CI rejects an unreviewed upstream CLI bump or a regression in the launcher/config/auth flow.

## Source + Goal linkage

- **Source:** User request 2026-08-02, following research of `render-oss/cli` and bex's completed `w9/m2` Render-CLI compatibility work.
- **Goal linkage:** Advances ADR008's familiar Render-compatible control-plane surface while giving users a Bex-native command and isolated credentials.
- **Expected outcome:** Users install and invoke `bex` with the upstream Render CLI's command/flag behavior, but it safely targets bex-api and never reads or writes a Render CLI config.
- **Why now:** The server-side Render REST/device-flow compatibility and live verification already exist; importing the upstream CLI converts that evidence into a low-maintenance product entrypoint without taking on a downstream command fork.
- **Render parity:** Omitted as a standing task because this milestone changes no REST, GraphQL, MCP, or dashboard contract. `w9/m2`/`m4`'s existing official-client suite remains the backend parity oracle; t004 extends it to the new launcher.
