# w4 · m95 — Complete the trust setup for external PostgreSQL connections

**Worker:** worker4 **Goal:** a tenant can obtain its database's server CA through bex and use the supplied external URL/PSQL instructions with full certificate and hostname verification. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Expose the database server CA through authenticated connection info | 50m | — — **DONE** |
| t002 | Provide CA download and complete external client instructions | 45m | t001 — **DONE** |
| t003 | Cover shared connection variants, authorization, and CA lifecycle | 50m | t001, t002 — **DONE** |
| t004 | Render parity | 20m | t003 — **DONE** |
| t005 | Simplify | 20m | t004 — **DONE** |
| t006 | Test coverage | 40m | t004 — **DONE** |
| t007 | Closeout | 20m | t005, t006 — **DONE** |

## Outcome (2026-09-07)

Shipped. Connection-info now delivers the CNPG cluster's private **server CA** so an external `sslmode=verify-full` client can actually connect — closing the gap where the external URL/PSQL command pinned full verification against a private CA the product never handed out.

- **t001 — CA on the authenticated read.** `PostgresConnectionInfo` gains an additive `serverCaCertificate` PEM field (REST struct + GraphQL `PostgresConnectionInfo.serverCaCertificate`). `Service.serverCACertificate` reads the CNPG-default `<cluster>-ca` Secret in the Database's own namespace (`cluster.SetName(db.Name)`, so the name is `db.Name+"-ca"`; the operator leaves CNPG's default CA in place and only adds public SNI SANs, so one CA covers the primary, pooler, and replica public endpoints alike). `certificateOnlyPEM` re-encodes **only** `CERTIFICATE` blocks — a private key, headers, or trailing bytes can never leak, and any non-certificate block fails the whole bundle. Only populated for `spec.public` databases; a missing CA is an actionable `ErrUnavailable` (503) and malformed material is refused rather than downgrading TLS. Existing `bex-tenant-api` RBAC already grants unrestricted `secrets` `get` in tenant namespaces — no RBAC change needed.
- **t002 — download + trust instructions.** The dashboard Connections panel renders a **Server CA certificate** field with a per-database `<id>-ca.pem` download (new shared `downloadTextFile` helper) and `PGSSLROOTCERT="/path/to/<id>-ca.pem"` guidance that keeps `sslmode=verify-full` and never suggests a weaker mode. Internal-only databases (empty CA) keep a coherent panel with no CA section. en/zh locales added.
- **t003/t006 — shared scope + failure-sensitive tests.** `server_ca_test.go` proves REST and GraphQL serve the identical bundle, the response never carries `PRIVATE KEY`, a missing CA → 503, a smuggled key is refused, and internal-only DBs omit the field; `certificateOnlyPEM` unit table covers empty/garbage/key-block/mixed-bundle. Pooler/replica seeding threads the CA. `connection-info-panel.test.tsx` drives the real download (Blob/anchor/revoke) and asserts the verify-full instructions.
- **t004 — Render parity.** ADR009 external-URL row and ADR018 Postgres row record this as a **deliberate bex extension**: Render's public DB hosts present publicly-trusted certificates, so it needs no counterpart; bex pins verify-full against each cluster's private CNPG CA, so completing that trust setup in-product is required. MCP deliberately still has no connection-info tool (not silently invented).
- **t005 — simplify.** The Blob/anchor/revoke download dance is now one shared `@/common/lib/download-file` helper; env-export and Generate-Blueprint download call it too (no per-feature drift).

**Verification:** backend `go test ./...` green (61 packages, EXIT 0) incl. the new `postgres` cases; backend `golangci-lint` 0 issues; dashboard `yarn typecheck` + `yarn lint` clean; dashboard tests green (connection-info-panel + blueprint + services suites). **t007 live re-probe deferred to the next QA pass** — no cluster access this session, the same close-out constraint recorded for m91–m94; the deployed change should repeat the README's create → reveal → external psql `verify-full` → cleanup walk.

## Definition of done

- Create a free public PostgreSQL database, wait for Available, open Connections, and Reveal. Obtain its server CA through the authenticated product flow, follow the displayed client setup, and run `SELECT current_database(), current_user` from an external psql client. It returns the created database/user with `sslmode=verify-full`; it requires no Kubernetes access or CA harvested from an unverified socket. Repeat after a fresh dashboard load.
- From a machine without `~/.postgresql/root.crt`, the panel explains the required setup and supplies the correct CA and an explicit client trust-file path. Following those instructions succeeds. Merely appending `sslrootcert=system` is not the implementation: that control failed against the live private CA.
- A wrong CA or wrong hostname still fails validation. No fallback to `require`, `prefer`, disabled verification, or trust-on-first-use is introduced.
- The existing SQL-console control still returns the configured database and owner; database ID, database name, user and public hostname survive a display rename.
- Delete the QA database and verify its REST lookup is 404 and it disappears from the dashboard list.

## Source + Goal linkage

- **Source:** continuous live `$qa-find-bugs`, requested destination w4, 2026-09-06 PDT / 2026-09-07 04:55–05:10 UTC, pass 4. Workspace `tian-personal` (`tea-da2isimlm39c739m4ofg`), Hobby. Fixture `qa-20260906-postgres`, renamed `qa-20260906-postgres-renamed`, `dpg-daf47n7co25s73fkr360`, free PostgreSQL 18, public access on, database `qa_20260906_data`, user `qa_20260906_owner`.
- **Goal linkage:** ADR009 managed PostgreSQL and ADR006 API hosting contract. A new database must be connectable from the customer's machine without platform-admin credentials.
- **Expected outcome:** complete the CA installation lifecycle already required by ADR009 while preserving the security hardening in `9081fbdb8`.
- **Why now:** a new Available database's copied connection string fails twice, including after reload; the system-trust control also fails. The authenticated UI offers no way to obtain the necessary CA. The SQL console works, so the underlying database is usable.
- **Severity:** blocker for the external connection journey. Internal SQL-console operation succeeds; this is not a claim that PostgreSQL itself is down.
- **Render parity:** included. [Render's connection guide](https://render.com/docs/postgresql-creating-connecting) describes using the external URL and directly running the supplied PSQL command. Bex deliberately requires full verification, so its private-CA setup must be complete rather than adopting weaker TLS semantics.

## Reproduction and durable evidence

1. New → New Postgres; create the free public fixture above and wait for Available.
2. Connections → Reveal connection info → copy External connection string. Feed it to psql without exposing its password. This run used an in-memory one-shot loopback probe: parsed URI values became `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD`, `PGSSLMODE`; psql ran with `-X -A -t -v ON_ERROR_STOP=1`, a 10-second connect timeout and a fixed transaction creating a temporary table, reading it, then rolling back. No connection reached SQL because TLS verification failed.
3. Reload the page, reveal/copy again, and repeat. The URI's non-secret shape was `postgresql://<redacted-user>:<redacted-password>@dpg-daf47n7co25s73fkr360.db.bex.co:5432/qa_20260906_data?sslmode=verify-full`.

First psql result (complete sanitized probe response):

```json
{
  "ok": false,
  "exit": 2,
  "stdout": "",
  "stderr": "psql: error: connection to server at \"dpg-daf47n7co25s73fkr360.db.bex.co\" (2a01:4f8:c01e:3d1f::1), port 5432 failed: root certificate file \"/Users/tianpan/.postgresql/root.crt\" does not exist\nEither provide the file, use the system's trusted roots with sslrootcert=system, or change sslmode to disable server certificate verification.\n"
}
```

Fresh-page repeat returned the same missing-root error through IPv4 `49.12.20.236`. The diagnostic `PGSSLROOTCERT=system` control retained `verify-full` and returned:

```json
{
  "ok": false,
  "exit": 2,
  "stdout": "",
  "stderr": "psql: error: connection to server at \"dpg-daf47n7co25s73fkr360.db.bex.co\" (2a01:4f8:c01e:3d1f::1), port 5432 failed: SSL error: certificate verify failed\n"
}
```

A credential-free TLS inspection, `/opt/homebrew/opt/openssl@3/bin/openssl s_client -starttls postgres -connect dpg-daf47n7co25s73fkr360.db.bex.co:5432 -servername dpg-daf47n7co25s73fkr360.db.bex.co -showcerts`, returned subject `CN=dpg-daf47n7co25s73fkr360-rw`, issuer `OU=tea-da2isimlm39c739m4ofg, CN=dpg-daf47n7co25s73fkr360`, and verification code 19, self-signed certificate in chain. This was inspection only: that unauthenticated chain was not installed as a trust anchor.

The UI SQL console ran `SELECT current_database(), current_user, version();` successfully: one row, `qa_20260906_data`, `qa_20260906_owner`, `PostgreSQL 18.4 (Debian 18.4-1.pgdg13+1) on x86_64-pc-linux-gnu, compiled by gcc (Debian 14.2.0-19) 14.2.0, 64-bit`.

Authenticated `POST https://api.bex.co/graphql`, `Content-Type: application/json`, body:

```json
{
  "query": "{database(id:\"dpg-daf47n7co25s73fkr360\"){id name status databaseName databaseUser externalHost}}"
}
```

HTTP 200, complete response after rename:

```json
{
  "data": {
    "database": {
      "databaseName": "qa_20260906_data",
      "databaseUser": "qa_20260906_owner",
      "externalHost": "dpg-daf47n7co25s73fkr360.db.bex.co",
      "id": "dpg-daf47n7co25s73fkr360",
      "name": "qa-20260906-postgres-renamed",
      "status": "available"
    }
  }
}
```

The schema inventory was queried in the same authenticated session after clicking Suspend:

```json
{
  "query": "{database(id:\"dpg-daf47n7co25s73fkr360\"){id name status suspended} __type(name:\"PostgresConnectionInfo\"){fields{name}}}"
}
```

HTTP 200, complete response:

```json
{
  "data": {
    "__type": {
      "fields": [
        { "name": "externalConnectionPoolString" },
        { "name": "externalConnectionString" },
        { "name": "internalConnectionPoolString" },
        { "name": "internalConnectionString" },
        { "name": "password" },
        { "name": "psqlCommand" },
        { "name": "readReplicaConnectionStrings" }
      ]
    },
    "database": {
      "id": "dpg-daf47n7co25s73fkr360",
      "name": "qa-20260906-postgres-renamed",
      "status": "available",
      "suspended": "suspended"
    }
  }
}
```

This last request proves the connection-info schema has no CA field; its lifecycle fields are recorded verbatim, not evidence of connection success while suspended. Local auxiliary files `.playwright-mcp/qa-postgres-probe.py`, `.playwright-mcp/qa-20260906-postgres-console.txt`, `.playwright-mcp/qa-20260906-postgres-network.txt` were checked on disk. Console had zero errors at capture. These ignored artifacts supplement the durable probes above. No credentials are retained.

## Root cause, consumers, and fix contract

- `lego/backend/internal/postgres/service.go:695–702` emits `verify-full` in the external URL and PSQL command but neither a CA nor a trust-file parameter/setup contract. `PostgresConnectionInfo` at `:192–205` contains no CA representation. The security requirement is correct; changing that mode is not the fix.
- `lego/operator/internal/controller/database_controller.go:453–459` supplies `serverAltDNSNames` only, leaving CNPG's default private server CA. Its comment at `:210–214` explicitly assumes clients already trust that CA. ADR009:48 and :264 require clients to install it but describe no tenant delivery path.
- Checked the pinned dependency, not just the assumption: `deploy/helm-artifacts.lock:20` pins chart 0.29.0 (CNPG 1.30.0). [CNPG v1.30.0 cluster_pki.go](https://github.com/cloudnative-pg/cloudnative-pg/blob/v1.30.0/internal/controller/cluster_pki.go#L116) defaults to an operator-generated root CA and signs the server leaf from it. `ensureCASecret` creates/renews that CA. The live issuer agrees with this mechanism.
- `dashboard/src/features/databases/components/connection-info-panel.tsx:90–123` renders password, internal URL, external URL and PSQL command. It provides no CA download or trust setup. The underlying `ConnectionField` copies the exact supplied value; it cannot supply a missing CA through formatting.
- Add a narrowly scoped authenticated server-CA certificate field/download via the existing connection-info service and dashboard. Read the current server CA referenced by this database's CNPG Cluster in its own namespace, verify ownership, expose certificate bytes only (never CA private key), and explain a per-database local file path. Provide psql instructions that explicitly reference that downloaded file while retaining `verify-full`.
- Shared producer has **two production entrypoints** from exhaustive `rg PostgresConnectionInfo lego/backend/internal --glob '*.go' --glob '!**/*_test.go'`: REST `GET /v1/postgres/{id}/connection-info` (`rest.go:205`) and GraphQL `databaseConnectionInfo` (`graphql.go:385`). No current MCP connection-info wrapper was found; do not invent one silently. REST struct and GraphQL fields (`graphql.go:172–180`) plus dashboard query/types must evolve together.
- Shared outputs: direct external URL, external PSQL command, pooler external URI (`service.go:714`), and per-replica external URIs (`:728`). Internal direct, pooler, replica paths must preserve current behavior. t003 owns this shared scope and its tests.
- Before CA fetch settles, show loading/setup state rather than claiming the external setup is complete. Missing/not-yet-provisioned CA must be actionable unavailable/retry; malformed CA or store failure must not silently downgrade TLS. Maintain existing absent-resource, unauthenticated, forbidden and fresh `can_view_sensitive` behavior; do not reveal a cross-workspace CA/resource existence through an unauthorized path.
- CA renewal/rotation must fetch the current authenticated bundle and explain re-download when needed. Avoid a generic deployment-wide CA assumption: the observed issuer is database-specific.

## Dedupe and limits

- Searched open and done PM for `verify-full`, `sslrootcert`, `CA distribution`, and server-CA references; scanned every open milestone title. No scheduled CA-delivery work matched. ADR077 finding 4 explicitly recorded the need for public-host certificate plus CA distribution before enabling `verify-full`; `9081fbdb8` later enabled it and amended ADR009, but omitted delivery. This filing is the missing lifecycle follow-through, not a request to undo hardening.
- `w1/done/m29` implements the PostgreSQL SSLRequest/SNI transport; this run reached the intended database certificate. `w6/done/m93` canonicalizes internal hosts, unrelated to missing trust roots. Current `origin/main` at filing prep is `6fc830c5e`; fetch/pull and targeted history found no already-landed delivery fix.
- No anti-goal applies. No paid plans, poolers, replicas, additional database-user credentials, CA rotation, or correct-CA external connection were live-tested. Those variants are implementation verification work in t003, not observed failures. The full PSQL command was source-traced but not separately executed; the copied external URL was the live client probe.
- Initial probe harness mistakes (URI passed as PGDATABASE without parsing, and unsupported VM URL global) were corrected and excluded. No TLS verification was disabled. No production Kubernetes Secret was read.

## QA pass cleanup and controls

Resume eventually restored SQL access: `SELECT marker FROM qa_lifecycle;` returned one row, `qa-preserved`, after the test table was created before suspend. The first post-resume attempt returned `internal error`; the settled retry after reload passed, so this transient is not filed as a separate defect. The suspended header and raw metadata status differed; that presentation candidate is not part of this CA filing.

Deleted the renamed QA database through its typed-confirm dialog. Authenticated `GET https://api.bex.co/v1/postgres/dpg-daf47n7co25s73fkr360` returned HTTP 404 with complete body `{"error":"app not found","id":"not_found","message":"app not found"}`. Dashboard Overview no longer contained the QA name and retained pre-existing `tianpan-v4-web`. No QA resource from this pass remains in the product. Physical PVC/Secret cleanup was not independently inspected.

[PostgreSQL 18 SSL documentation](https://www.postgresql.org/docs/18/libpq-ssl.html) describes the trust-root file requirement for verification and the `sslrootcert` / `PGSSLROOTCERT` configuration. The proposed setup follows that client contract; the runtime error's suggestion to disable verification is not a recommended workaround.
