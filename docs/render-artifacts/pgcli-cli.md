# Official Render CLI `pgcli` PTY verification

**Verified:** 2026-07-18 PDT **Target:** local CAPD app cluster, `dev-5` (non-production) **Clients:** unmodified Render CLI v2.21.0 and pgcli 4.5.0

## Result

The official CLI's interactive-only `pgcli` command passed headlessly against a disposable public bex Postgres by both immutable `dpg-…` id and exact display name. Each path invoked a real pgcli process, executed `SELECT 1 AS bex_pgcli_probe;`, observed the column, value `1`, and successful `SELECT 1` status, sent `\q`, and returned zero. The verifier then deleted the Database and its credential-bearing local state.

| gate | result | durable evidence |
| --- | --- | --- |
| Piped official-CLI control | PASS | The unmodified process returned `` `render pgcli` can only be used in interactive mode `` before making a bex request. |
| PTY runtime contract | PASS | stdin, stdout, and stderr were TTYs; `TERM=xterm-256color`; `CI` and `RENDER_OUTPUT` absent; explicit terminal size; bounded marker waits. |
| Opaque-id resolution | PASS | One safe shim invocation and one real pgcli SQL session resolved `dpg-…`. |
| Exact-name resolution | PASS | One safe shim invocation and one real pgcli SQL session resolved the same immutable id, host, database, and TLS contract. |
| Passthrough arguments | PASS | The shim received `--csv -q` in that order after the external URI. |
| External connection | PASS | A non-empty `postgresql://` URI had the resolved external host/database and `sslmode=require`; username and password were required but never emitted. |
| Real query | PASS | Both sessions observed the ordered safe markers `bex_pgcli_probe`, `"1"`, and `SELECT 1`, then clean client/CLI exits. |
| Negative names | PASS | Unknown name failed before child invocation live; unknown and ambiguous names both failed before child invocation in the deterministic official-CLI test. |
| Cleanup | PASS | The verifier-created Database returned not-found after DELETE; PTY config, shim records, pgcli config/history, wrappers, credentials, and temporary transport state were removed. |

## Reproduction contract

[`scripts/cli-compat.sh`](../../scripts/cli-compat.sh) exposes the maintained entry point:

```sh
BEX_PGCLI_TARGET_CLASS='<non-production target description>' \
  BEX_PGCLI_NON_PRODUCTION=1 \
  BEX_PGCLI_REAL_BIN=/path/to/pgcli \
  bash scripts/cli-compat.sh pgcli-verify
```

The existing CLI harness supplies the pinned `RENDER_BIN`, exchanges its ignored local API-key credential for a short-lived bearer, and sets `RENDER_HOST` and `RENDER_WORKSPACE`. The target must have all of the following:

- a non-production bex API and App namespace;
- a configured `BEX_DB_DOMAIN` plus a reachable pg-sni-proxy edge;
- CNPG provisioning that reaches `available` before the bounded deadline;
- a verifier source CIDR admitted by the disposable database's IP allow list;
- a real pgcli executable. The recorded run used pgcli 4.5.0 with its binary psycopg distribution.

`BEX_PGCLI_DATABASE_ID` plus `BEX_PGCLI_EXISTING_DISPOSABLE=1` can target an explicitly disposable fixture. That mode does not delete the caller-owned fixture. The default mode is the acceptance mode: it creates a uniquely named public Free database and unconditionally deletes it on success, failure, timeout, interruption, or child error. Production API origins are refused.

## What is verified

The pinned CLI source establishes the execution contract:

1. `ParseCommandInteractiveOnly` accepts the command only when all three standard streams are TTYs and the runtime is neither CI nor a dumb/forced output mode.
2. An id selector performs `GET /v1/postgres/{id}`. A name selector performs `GET /v1/postgres?name=…` and requires exactly one match.
3. The CLI checks its observed public IP against `ipAllowList` before requesting sensitive connection information.
4. Only then does it request `GET /v1/postgres/{id}/connection-info` and execute `pgcli <externalConnectionString> <arguments after -->`.

The deterministic test runs this unmodified client against a safe fake API and records only method/path classes. It proves the id and name request order, exactly-once child handoff, ordered passthrough flags, empty/malformed external URI rejection, unknown/ambiguous name rejection before child invocation, missing binaries, timeout and descendant teardown, and nonzero real-client failure. The live run then proves the same handoff reaches a real TLS/database session rather than stopping at the shim contract.

## Redaction boundary

[`scripts/render-cli-pty.py`](../../scripts/render-cli-pty.py) owns the only PTY. It keeps terminal bytes in a bounded in-memory tail long enough to match named markers, never writes or prints the raw ANSI stream, and reports only marker, timeout, and numeric-exit facts. It gives the process group a deadline and terminates descendants on timeout.

[`scripts/pgcli-verify-client.sh`](../../scripts/pgcli-verify-client.sh) receives the same credential-bearing argv the real client would receive, parses the URI in-process, and records only scheme, host, database, TLS mode, immutable id, selector label, and allow-listed flags. It never records or prints the URI, username, password, bearer, or raw argv. The full verifier pipes the sensitive connection-info response directly into the same allow-listing parser and clears the in-memory shell values immediately.

The regression suite plants a known token/password in API responses, URIs, child output, and failure paths, then scans every captured output and temporary artifact for it. The planted value was absent. No terminal transcript exists to publish.

## Commands and safe markers

The completion run used:

```sh
bash scripts/render-cli-pty.test.sh
bash scripts/pgcli-compat-verify.test.sh
bash scripts/pgcli-compat-verify.sh # with the non-production variables above
```

The live verifier emitted only these result classes:

- `PASS pgcli official-non-tty-guard`
- `PASS pgcli disposable-database-created id=dpg-…`
- `PASS pgcli external-connection-precondition … tls=require`
- `PASS pgcli id-resolution-and-shim-handoff`
- `PASS pgcli unknown-name rejected-before-child`
- `PASS pgcli name-resolution-and-shim-handoff`
- ordered `PASS pty marker {id,name}-{ready,column,value,status,client-done}`
- `PASS pgcli real-sql-id` and `PASS pgcli real-sql-name`
- `PASS pgcli cleanup disposable-database`
- `PASS pgcli cleanup local-artifacts`

`docs/ADR018-render-parity.md` required no change: it points at the CLI compatibility checklist as the fifth-surface ledger and made no stale claim that `pgcli` remained unverified.
