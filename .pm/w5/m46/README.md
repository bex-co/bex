# w5 · m46 — Automated PTY verification of official Render CLI `pgcli`

**Worker:** worker5 **Goal:** Make the unmodified official Render CLI's interactive-only `pgcli <dpg-id|name>` command reproducibly verifiable from automation: a bounded pseudo-terminal run must resolve a live bex Postgres by opaque id and by name, obtain its external connection information, invoke `pgcli`, execute a harmless SQL probe, and exit without leaking credentials. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Build a bounded, redaction-safe pseudo-terminal runner for interactive Render CLI commands | 45m | — |
| t002 | Verify `pgcli` id/name resolution and child-process handoff against non-production bex | 45m | t001 |
| t003 | Drive a real `pgcli` SQL session through the PTY and clean up the disposable database | 45m | t002 |
| t004 | Publish durable evidence and re-grade the CLI compatibility checklist | 30m | t002, t003 |
| t005 | Simplify the PTY and `pgcli` verification code (standing) | 20m | t004 |
| t006 | Add regression coverage for TTY detection, timeouts, handoff, and secret redaction (standing) | 30m | t004 |
| t007 | Closeout (standing) | 15m | t005, t006 |

## Definition of done

The repository has a reusable pseudo-terminal driver that gives the unmodified `render` process TTY-backed stdin, stdout, and stderr, uses a non-dumb terminal with `CI`/forced non-interactive output disabled, bounds every wait, and emits only named pass/fail markers rather than a raw ANSI transcript. A piped control run still produces the official CLI's `` `render pgcli` can only be used in interactive mode `` error, proving the PTY path crosses the real client guard instead of bypassing it by patching or forking the CLI.

A non-production verification provisions or selects a disposable public bex Postgres whose source-IP allow list admits the verifier and whose `GET /v1/postgres/{id}/connection-info` response has a non-empty external URI. The pinned unmodified Render CLI resolves the same database once by its `dpg-…` id and once by its display name, requests sensitive connection info only after resolution, preserves arguments passed after `--`, and invokes a redaction-safe `pgcli` shim with a syntactically valid external PostgreSQL URI. The shim may validate scheme, host, database, TLS mode, and passthrough flags, but it must never persist or print the URI, password, bearer token, or raw process arguments.

A second run replaces the shim with an installed real `pgcli`, sends `SELECT 1 AS bex_pgcli_probe;` followed by `\q` through the PTY, observes the expected value and a zero exit by both id and name, and removes every disposable database, local config, shim, transcript, and credential-bearing temporary file on success or failure. The evidence artifact records the CLI and `pgcli` versions, non-production target, redacted request/child-process sequence, negative guard control, id/name results, and cleanup proof. Only then does `docs/cli-compatibility-checklist.md` change the `pgcli` row from `[ ]` to `[x]`; partial proof leaves the row and milestone open with the exact failed gate named.

The PTY runner and verifier have automated failure-path coverage for non-TTY execution, missing client binaries, timeout/hang, empty external connection information, malformed or unexpected child arguments, non-zero child exit, and planted-secret redaction. Markdown formatting, script syntax checks, focused tests, and `git diff --check` pass.

## Source + Goal linkage

- **Source:** user request 2026-07-18 to research and materialize the `docs/cli-compatibility-checklist.md` row ``[ ] `pgcli [id|name]` — interactive-only client guard; not verifiable headlessly`` as a w5 milestone. Research used the pinned ignored checkout `cli/` at v2.21.0: `cmd/pgcli.go` calls `command.ParseCommandInteractiveOnly`; `pkg/command/outputresolver.go` requires all three standard streams to be TTYs and rejects `CI`/`TERM=dumb`; `pkg/tui/views/psql.go` resolves `dpg-…` ids or exact names, checks the caller IP, fetches `/connection-info`, and execs `pgcli <externalConnectionString> ...`. `scripts/ssh-verify.sh` supplies the repository precedent for verifying an interactive official-CLI handoff with a redaction-safe child shim.
- **Goal linkage:** pillar 1 / Render compatibility (`docs/ADR008-vision.md`) and the official Render CLI fifth-surface contract tracked by `docs/cli-compatibility-checklist.md`. This closes a verification blind spot without violating `.pm/DO_NOT_DO.md`'s prohibition on building, forking, or vendoring a bex CLI.
- **Expected outcome:** `render pgcli <dpg-id>` and `render pgcli <name>` are both repeatably proven against bex from a headless runner, including the actual interactive guard, external connection-info handoff, a harmless real SQL round trip, passthrough arguments, cleanup, and credential redaction.
- **Why now:** typed Postgres ids and name resolution already shipped (`w9/m3`), the connection-info contract and public SNI route already exist (`docs/ADR009-postgresql-management.md`), and the 2026-07-18 full CLI pass left `pgcli` unchecked only because its TTY guard fires before any request in the current harness. The remaining risk is therefore bounded verification plumbing, and the adjacent `psql` path plus SSH verifier provide working precedents.
- **Render parity closing task: omitted** — this milestone changes verification scripts and documentation only; it does not change REST, GraphQL, MCP, dashboard, operator, or CLI behavior. The milestone itself is the official-CLI parity proof. Simplify, Test coverage, and Closeout remain required.
- **Explicit exclusions:** no edits to the ignored `cli/` checkout; no first-party/forked CLI; no weakening of Postgres IP allow lists or TLS; no production database; no long-lived public fixture; no logging of connection strings, passwords, access tokens, or raw PTY output; no expansion to `kv-cli`, `psql`, or `ssh` beyond reusable PTY plumbing.
