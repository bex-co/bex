# Official Render CLI `psql` verification

**Verified:** 2026-07-18 PDT **Target:** local CAPD app cluster, `dev-7` (non-production) **Clients:** checksum-verified, unmodified Render CLI v2.21.0 and PostgreSQL `psql` 18.4

## Result

The official CLI's non-interactive `psql` command passed end to end against one disposable public bex Postgres by both immutable `dpg-…` id and exact display name. Both paths executed `SELECT 1 AS bex_psql_probe;` through the real local `psql` process and returned the probe column and value `1` through the external TLS/SNI route. The verifier also proved the CLI's client-side source-IP denial and deleted the Database and credential-bearing local state.

| gate | result | durable evidence |
| --- | --- | --- |
| Official release provenance | PASS | Render's published `cli_2.21.0_darwin_arm64.zip` SHA-256 was `3d721f8e5f26e8d920eec899c28b200e74901529ad5d964b180c5d09c7ad3546`; the manifest matched, and the extracted executable used by the run had SHA-256 `b936020f083a83f170b1eeae1b7e739ee533812f794c463e4cbae18ba8b550a8` and reported `render v2.21.0`. |
| Source-IP allow list | PASS | The create request contained `{cidrBlock:"[redacted IPv4]/32",description:"psql compatibility verifier"}` for the address returned by `api.ipify.org`. Loopback `/32` and `/128` entries separately admitted the local Kubernetes port-forward transport; they cannot satisfy the CLI's public-IP comparison. |
| External connection | PASS | The resolved host was `dpg-….db.localtest.me`, the database was the matching `dpg_…`, and the credential-bearing URI required `sslmode=require`; username, password, bearer, and full URI were never emitted. |
| Opaque-id probe | PASS | `render psql <dpg-…> -c 'SELECT 1 AS bex_psql_probe;' -o text` crossed the public-IP gate and returned the asserted probe through real `psql` 18.4. One bounded retry covered the expected interval between CNPG health and the service accepting connections. |
| Exact-name probe | PASS | The same command using the fixture's exact display name resolved the same immutable database and returned the asserted probe on its first attempt. |
| Unknown name | PASS | A unique missing name failed with `No Postgres instance found` before connection. |
| Absent caller IP | PASS | Replacing the allow list with `192.0.2.0/24` made the unmodified CLI exit nonzero with `not in allow list` before spawning `psql`. |
| Cleanup | PASS | The verifier-created Database returned not-found after DELETE; its temporary CLI configs, pinned-client shim directory, and other local artifacts were removed. |

## Redacted live capture

The completion run used the ignored dev-7 API-key file and release binary; no credential was present in the command or output capture:

```sh
BEX_API_URL=http://localhost:54070 \
  HYDRA_PUBLIC_URL=http://localhost:58070 \
  CLI_KEY_ENV=.pm/w7/dev-7/.cli-key.env \
  RENDER_BIN=.pm/w7/dev-7/bin/render \
  BEX_PSQL_TARGET_CLASS='local CAPD dev-7 via pg-sni-proxy' \
  BEX_PSQL_NON_PRODUCTION=1 \
  BEX_PSQL_REAL_BIN=/opt/homebrew/Cellar/libpq/18.4/bin/psql \
  BEX_PSQL_ADDITIONAL_ALLOW_CIDRS='127.0.0.1/32,::1/128' \
  bash scripts/cli-compat.sh psql-verify
```

The verifier emitted only these safe result markers for the successful fixture:

```text
INFO psql target=local CAPD dev-7 via pg-sni-proxy
INFO official-cli=render v2.21.0
PASS psql disposable-database-created id=dpg-…
PASS psql external-connection-precondition host=dpg-….db.localtest.me database=dpg_… tls=require
INFO psql probe-id ready-after-attempt=2
PASS psql probe-id
PASS psql probe-name
PASS psql unknown-name rejected
PASS psql allow-list-deny rejected-before-connect
PASS psql compatibility verification complete
PASS psql cleanup disposable-database
PASS psql cleanup local-artifacts
```

## Why this row had been `[~]`

`render psql [id|name]` (source: `cmd/psql.go` → `pkg/tui/views/psql.go`) does three things before it ever connects:

1. **resolves** the target — `GetPostgres(id)` first, else `ListPostgres(name=…)` (ambiguous name ⇒ error);
2. **checks the allow list client-side** — `getUserIP()` fetches the caller's public IP from `api.ipify.org`, then `hasAccessToPostgres` iterates `pg.ipAllowList[].cidrBlock` and refuses with `IP address (…) not in allow list for …` when no CIDR contains the caller (an **empty** list matches nothing ⇒ deny; the check is skipped only if ipify is unreachable);
3. **uses `connectionInfo.externalConnectionString`** — always the external string — and execs `psql <uri> -c <command>`.

In the base `dev-9` environment there is no `BEX_DB_DOMAIN` (so no external route ⇒ `externalConnectionString` is omitted) and the fixture's allow list is empty (so the CLI's own client-side gate refuses). Both are the **same blocks Render imposes**, not bex defects — see the confirmation below.

## bex is already CLI-compatible (no code gap)

Confirmed against `lego/backend` HEAD:

| the CLI reads | bex-api emits | evidence |
| --- | --- | --- |
| `pg.ipAllowList[].cidrBlock` on the read shape | `PostgresView.IPAllowList` (`ipAllowList`, entries `{cidrBlock, description}`) | `internal/postgres/service.go` (`pgView` → `core.AllowListFromSpec`); `core.IPAllowListEntry.CIDRBlock` → `json:"cidrBlock"` (`internal/core/cidr.go`, "verified against the render-oss/cli generated client `client.CidrBlockAndDescription`") |
| resolve by id **and** name | `GET /v1/postgres/{id}` + `GET /v1/postgres?name=` (OR-matches `p.Name` and `p.ID`) | `internal/postgres/rest.go:118` |
| `externalConnectionString` | populated iff `d.Status.ExternalHost != ""`, itself set iff `db.Spec.Public && BEX_DB_DOMAIN != ""` | `internal/postgres/service.go:574`; `operator/internal/controller/database_controller.go:800` |

So the allow-list gate is enforced by the **CLI itself** on data bex already surfaces; the external string is gated exactly as Render gates a non-public database.

## Hermetic regression (run 2026-07-18, PASS)

`scripts/psql-compat-verify.test.sh` drives the **real, unmodified** Render CLI through its entire non-interactive `psql` path against a deterministic fake bex-api and a fake `psql` binary — no live cluster, database, network, or real psql client required. It proves the bex-side contract the live run depends on:

| gate | result | evidence |
| --- | --- | --- |
| Opaque-id resolution | PASS | `render psql <dpg-…> -c 'SELECT 1 AS bex_psql_probe;' -o text` resolved the id, crossed the allow-list gate, read `externalConnectionString`, exec'd `psql`, and returned the probe row. |
| Exact-name resolution | PASS | The same probe resolved the fixture by display name. |
| External connection precondition | PASS | A non-empty `postgresql://` URI carried the resolved external host/database and `sslmode=require`; username/password required but never emitted. |
| Client-side allow-list deny | PASS | With the fixture's allow list flipped to `192.0.2.0/24` (TEST-NET-1), the CLI refused with `not in allow list` **before** execing psql (fake psql invoked exactly twice — id + name — never for the deny leg). |
| Unknown name/id | PASS | `No Postgres instance found` before any connection. |
| Non-zero psql exit | PASS | An exit-42 psql surfaced as `Error: exit status 42`; the verifier failed the probe rather than swallowing it. |
| Production API guard | PASS | `api.bex.co` / `api.render.com` origins refused before any fixture work. |
| Credential safety | PASS | A planted password never reached any durable output, request log, or temporary artifact. |
| Cleanup | PASS | The verifier-created fixture returned not-found after DELETE; local artifacts removed. |

## Reproduction contract

[`scripts/cli-compat.sh`](../../scripts/cli-compat.sh) exposes the maintained entry point:

```sh
BEX_PSQL_TARGET_CLASS='<non-production target description>' \
  BEX_PSQL_NON_PRODUCTION=1 \
  BEX_PSQL_REAL_BIN=/path/to/psql \
  bash scripts/cli-compat.sh psql-verify
```

The harness supplies the pinned `RENDER_BIN`, exchanges its ignored local API-key credential for a short-lived bearer, and sets `RENDER_HOST` and `RENDER_WORKSPACE`. The target must have all of the following:

- a non-production bex API and App namespace;
- a configured `BEX_DB_DOMAIN` plus a reachable pg-sni-proxy edge;
- CNPG provisioning that reaches `available` before the bounded deadline;
- a verifier source CIDR admitted by the disposable database's IP allow list (the default discovers this host's public `/32` via `api.ipify.org` — the same address the CLI's client-side check reads);
- a real `psql` executable (`postgresql-client`).

When a local port-forward gives the proxy a transport source address different from the CLI's public address, `BEX_PSQL_ADDITIONAL_ALLOW_CIDRS` accepts a comma-separated list of those transport CIDRs. The public `/32` remains the first allow-list entry and is still what the CLI checks. The verifier retries the real endpoint for a bounded convergence window after control-plane status becomes available.

The default mode creates a uniquely named public Free database and unconditionally deletes it on success, failure, timeout, interruption, or child error. `BEX_PSQL_DATABASE_ID` plus `BEX_PSQL_EXISTING_DISPOSABLE=1` targets an explicitly disposable fixture without deleting it. Production API origins are refused. The interactive TTY session is proven for the sibling [`pgcli` row](pgcli-cli.md) and cross-referenced rather than re-driven here; `psql`'s `-o text` non-interactive path is what Render's own CLI uses for scripted access.
