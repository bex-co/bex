# w4 · m116 — CLI hunt sweep 1: psql trust, KV publish, one-off jobs, resume path

**Worker:** worker4 **Goal:** every CLI journey the 2026-09-17 hunt proved broken works end to end from a stock `bex` install **Status:** todo

## Tasks (in order)

| id   | title                                                                 | est | depends_on |
| ---- | --------------------------------------------------------------------- | --- | ---------- |
| t001 | Make `bex psql`/`pgcli` connect without manual CA provisioning        | 60m | —          |
| t002 | Let a private Key Value become public after create                    | 90m | —          |
| t003 | Decide one-off jobs: grant the RBAC or gate the endpoint, and surface why they fail | 60m | — |
| t004 | Resume must restore the Postgres data path, not just the status       | 60m | —          |
| t005 | Device login asks for the password twice; refresh the e2e helper      | 45m | —          |
| t006 | Empty CLI lists print `[]`, not `null`                                | 30m | —          |
| t007 | Render parity across the touched surfaces                             | 45m | t001–t006  |
| t008 | Simplify the code this milestone changed                              | 30m | t007       |
| t009 | Test coverage for the shipped behavior                                | 45m | t007       |
| t010 | Closeout                                                              | 30m | t009       |

## Definition of done

- On a machine with no `~/.postgresql/root.crt` and no `PGSSLROOTCERT`, `bex psql <new-free-pg> -c 'SELECT 1'` exits 0 and returns the probe row (exact mechanism per t001's decision: launcher CA provisioning or a public-CA edge — never a TLS downgrade).
- A free KV created private, then `bex keyvalues update <id> --ip-allow-list cidr=<caller>/32,description=qa` (or the explicit-publish equivalent t002 ships): `connection-info` carries an external `rediss://` string, `cliCommand` targets it, and `bex kv-cli <id> -- PING` answers `PONG` from outside the cluster.
- `bex jobs create <live-free-web> --start-command 'echo <marker>'` either reaches `succeeded` (marker in the job's logs) or is refused with a named error — never a bare `failed` with no reason. The DO_NOT_DO one-off-jobs tension is resolved on the record (fix or gate, with the file updated if the surface stays).
- Suspend then resume a free Postgres: once status reads `available`, `bex psql -c 'SELECT 1'` succeeds within 5 minutes; until the data path is up the status must not claim `available`.
- A fresh `bex login` device approval completes with a single password entry (or the second entry is proven required and the reason is documented); `scripts/render-cli-auth-browser.cjs` drives the current `/auth/device` page end to end.
- `bex ea sandboxes list` and `bex workflows list` on an empty workspace print the Render-matching empty shape t006 establishes.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs-cli` infinite loop, sweep 1, 2026-09-17 (~06:30–07:05 UTC). Workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`), fixtures `qa-20260917-9ecd0a-*` (2 web, 1 static, 1 cron, 1 pg, 1 kv — all created and deleted in-sweep; baseline of 1 pre-existing sample service restored and verified). Installed `bex v0.2.1` (compatible with Render CLI v2.27.0, pin `a764810a7682`) plus a HEAD checkout build at `5d8bcc896` where a release-vs-HEAD comparison was needed. Human device-flow OAuth (granular `bex.read/write/sensitive`), never a machine token. Sanitized repro commands and redacted responses are inline in each task; no gitignored transcript carries evidence.
- **Goal linkage:** bex is the open-source Render alternative (ADR008); the pinned Render CLI is the wire-contract oracle, and every failed journey below is a place a Render user pointing `bex` at bex-api hits a wall Render does not have. Governing docs: ADR006 (bex-api surfaces), ADR009 (Postgres), ADR021 (Key Value), ADR012 (auth), ADR018 (parity ledger).
- **Expected outcome:** `bex psql`, `bex kv-cli`, `keyvalues update`, `jobs create`, datastore suspend/resume, and device login all complete from a stock install against production, and the empty-list wire shape matches Render.
- **Why now:** four of the six findings are regressions against previously green evidence (the Jul-18 psql supplement, the Jul-18 kv-cli acceptance, the `w7/m45` update tests, the checklist `[x]` rows); the longer the drift, the more users script around the breakage. The Render-parity closing task applies (REST/GraphQL/MCP/CLI surfaces all move).
