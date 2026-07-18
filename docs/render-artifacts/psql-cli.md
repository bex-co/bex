# Official Render CLI `psql` verification

**Harness authored + hermetically verified:** 2026-07-18 PDT **Client:** unmodified Render CLI (`render-oss/cli`, checkout `v2.21.0-8-gc23438e`, live acceptance pins the checksum-verified `v2.21.0` release) **Status:** bex-side wire contract proven CLI-compatible; live end-to-end run in a `BEX_DB_DOMAIN` + pg-sni-proxy environment is the remaining step to earn ✅ in [the CLI compatibility checklist](../cli-compatibility-checklist.md).

## Why this row was `[~]`

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

## Reproduction contract (live run — earns ✅)

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

The default mode creates a uniquely named public Free database and unconditionally deletes it on success, failure, timeout, interruption, or child error. `BEX_PSQL_DATABASE_ID` plus `BEX_PSQL_EXISTING_DISPOSABLE=1` targets an explicitly disposable fixture without deleting it. Production API origins are refused. The interactive TTY session is proven for the sibling [`pgcli` row](pgcli-cli.md) and cross-referenced rather than re-driven here; `psql`'s `-o text` non-interactive path is what Render's own CLI uses for scripted access.
