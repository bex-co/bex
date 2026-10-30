# w2 · m245 — Public Postgres URLs verify against system roots, as on Render

**Worker:** worker2 **Goal:** a tenant who copies a public database's external connection string into any stock Postgres client connects securely with no downloaded CA, and keeps connecting across certificate renewals, as with Render's `*-postgres.render.com` hosts. **Status:** done — every DoD bullet verified on production 2026-10-30

## Tasks (in order)

| id   | title                                                                                                   | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Prove on `dev-2` that CNPG serves a user-provided server certificate while its replication still verifies — **DONE** | 45m | —          |
| t002 | Operator: a publicly trusted certificate per externally reachable database, wired into CNPG and renewed   — **DONE** | 1h  | t001       |
| t003 | External connection string connects as-is from a stock client, Render-shaped                             — **DONE** | 40m | t002       |
| t004 | `serverCaCertificate` semantics on REST, GraphQL, MCP and the dashboard connect panel                     — **DONE** | 30m | t003       |
| t005 | Live acceptance on production: copy → connect, forced renewal → still connects                           — **DONE** | 40m | t004       |
| t006 | Correct ADR009, ADR018 and the connection docs                                                           — **DONE** | 20m | t005       |
| t007 | Render parity                                                                                           — **DONE** | 20m | t006       |
| t008 | Simplify                                                                                                — **DONE** | 15m | t007       |
| t009 | Test coverage                                                                                           — **DONE** | 30m | t007       |
| t010 | Closeout                                                                                                — **DONE** | 10m | t008, t009 |

## Definition of done

- On production, for a public database: `psql "<copied externalConnectionString>"` on a machine with no `~/.postgresql/root.crt` and no `PGSSLROOTCERT` connects with TLS, and the server certificate chains to a publicly trusted root (`openssl s_client -starttls postgres` → `Verify return code: 0`).
- After a forced renewal of that database's server certificate, the same command, unchanged, still connects.
- CNPG replication and internal (`-rw` service) connections keep working on an HA database with the new certificate.
- REST `GET /v1/postgres/{id}/connection-info`, GraphQL `databaseConnectionInfo`, MCP and the dashboard agree on the connection string and on what `serverCaCertificate` means.
- ADR009 / ADR018 describe what shipped.

## Source + Goal linkage

- **Source:** promoted from inbox note `w2/049` (2026-10-29, `/loopx w2`). The note's history is preserved in [source-w2-049.md](source-w2-049.md). Its `/unblock` decision (2026-10-29, Render parity rule): publicly trusted per-database certificates.
- **Evidence:** CNPG-generated tenant CAs rotate about every 90 days, which breaks `verify-full` clients that pinned the CA (w2/m170, Grafana). A copied production URL fails today with `root certificate file "~/.postgresql/root.crt" does not exist` (2026-10-24, w4 QA). Render's hosts present Let's Encrypt certificates (`oregon-postgres.render.com`, `frankfurt-postgres.render.com` → issuer `Let's Encrypt YR1`, verify OK; checked 2026-10-29).
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) pillar 1 (Render compatibility) and [ADR009](../../../../docs/ADR009-postgresql-management.md) managed Postgres external access.
- **Expected outcome:** teams migrating from Render paste their connection string and it works. No tenant connection breaks at a CA rotation.
- **Why now:** exposure grows with every public database. The CA cycle already broke a first-party consumer, and the parity decision is recorded.
- **Render parity included:** connection info is read on REST, GraphQL, MCP and the dashboard.

## Design notes (from the `/unblock` record, 2026-10-29)

- Production cert-manager has only HTTP-01 (traefik) ClusterIssuers, and `dpg-*.db.bex.co` answers ACME on port 80. Per-database HTTP-01 certificates therefore need no new credential. A `*.db.bex.co` wildcard would need a Cloudflare DNS-01 token, a user-supplied credential. Use it only if per-host HTTP-01 proves unworkable.
- Let's Encrypt allows 50 **new** certificates per registered domain (`bex.co`) per week, shared with every other `*.bex.co` certificate. Renewals are exempt. That caps new public databases at roughly 50/week. t002 records the budget, and if the wildcard is needed, file it as a follow-up with the exact token scope.
- libpq's `sslmode=verify-full` reads `~/.postgresql/root.crt` unless `sslrootcert=system` (libpq ≥ 16) is given. t003 must therefore choose the URL shape that connects as-is on stock clients, matching Render's exact external URL format.

## t001 findings — 2026-10-29 (`dev-2`, CNPG 1.27.0; production runs 1.30.1)

A two-instance CNPG `Cluster` in namespace `dev-2` used `spec.certificates.serverTLSSecret` (a cert-manager `Certificate` from a local stand-in CA) and `serverCASecret` (a secret holding **only** `ca.crt`, no key: the shape a Let's Encrypt chain will have). Afterwards every test object was deleted.

- **CNPG accepts it.** It keeps generating and managing its own client CA (`<cluster>-ca`) and `streaming_replica` certificate alongside. `status.certificates` listed all four secrets with their expirations.
- **Replication works.** The joining replica's `pg_basebackup` verified the primary against `server-ca.crt` from the provided `serverCASecret` (`host=<cluster>-rw … sslmode=verify-ca`), so **the server certificate must also cover the in-cluster names** (`<cluster>-rw`, `-r`, `-ro` and their namespace forms). For Let's Encrypt this means a CA secret whose `ca.crt` holds the chain's root (ISRG Root X1/X2). Internal names cannot appear in a public certificate, so t002 must check whether replication verifies only the CA (`verify-ca`, as the join log shows) or also the hostname, and keep in-cluster replication working (e.g. CNPG's `serverAltDNSNames` only on a CNPG-managed cert, or a separate internal path).
- **Served certificate:** `openssl s_client -starttls postgres` returned the issued certificate, `Verify return code: 0`. Writes replicated (`pg_stat_replication`: `streaming`; replica row count followed).
- **Renewal needs the `cnpg.io/reload` label.** Without it, after cert-manager re-issued the secret, the old certificate stayed served for more than 85 seconds. With `spec.secretTemplate.labels: {cnpg.io/reload: ""}` on the `Certificate`, the new serial was served within about 40 seconds after renewal, with no Pod restart and replication still streaming. **t002 must set that label.**

## t002–t004 — shipped 2026-10-30 (`19bfc60d1`, `1d5212871`)

- **Operator:** new `BEX_DB_CLUSTER_ISSUER` (production overlay: `letsencrypt-prod`; unset everywhere else, which keeps CNPG's own certificate). Each public database gets a `<db>-public-tls` cert-manager Certificate (primary, pooler and replica SNI names; Secret labeled `cnpg.io/reload`, stamped with the DB UID). It is wired into CNPG (`serverTLSSecret` + `serverCASecret` = a root-only Secret holding the root that Go's system-pool verification anchors the chain to, which is future-proof for Let's Encrypt's Y roots) only once the Certificate is Ready for its generation and the chain verifies (trusted, right host, unexpired). CNPG refuses `serverAltDNSNames` alongside `serverTLSSecret`, so it is dropped when wired. A served certificate stays served across re-issues while valid. `status.externalCertificate` goes `Pending` → `PubliclyTrusted` only once CNPG reports serving it. Turning public access off reverts to CNPG's certificate and cleans up safely.
- **Why internal names are not needed** (from CNPG source): replication and PgBouncer use `verify-ca`; the CNPG status client checks the chain and ignores DNS; pg-sni-proxy never terminates TLS. The NetworkPolicy needs no change, because `allow-traefik-ingress` admits the HTTP-01 solver.
- **URL (t003):** once publicly trusted, connection-info returns Render's exact external URL shape with **no `sslmode`** (libpq's default still negotiates TLS, and the proxy accepts nothing else). `verify-full` alone would demand `root.crt`, and `sslrootcert=system` breaks libpq < 16, node-postgres, pgjdbc and Prisma. `psqlCommand` takes Render's `PGPASSWORD=… psql -h … -U … db` form. While pending, or with the feature off, the URL is unchanged (`verify-full` + CA).
- **`serverCaCertificate` (t004):** present only while CNPG's own certificate is served, omitted (REST) or empty (GraphQL) once public. MCP has no connection-info tool. The dashboard already hides the CA block when it is empty, and a test pins that no `root.crt` / `PGSSLROOTCERT` copy appears.
- **Let's Encrypt budget:** 50 new certificates per week for `bex.co`; renewals are exempt; a name-set change counts as a new order. Recorded in `database_public_tls.go`.
- **Exposure checked before shipping:** the only two public databases (`dpg-db34l9bor55s73cqpq9g` in `bex`, `dpg-db9r654flioc739mcpbg` in `bex-canary`) are QA fixtures with no active client and no in-cluster config referencing their URLs, so the certificate switch breaks no pinned client.
- **Known limit, same as Render:** a client machine with a stale `~/.postgresql/root.crt` still verifies against that file.

## t005 — production acceptance, 2026-10-30 (PASS)

Deployed with pin `ef993f254` (images `dc80c29bf2bc`, which contains `19bfc60d1`/`1d5212871`). The earlier run failed only on a Sigstore Rekor timeout (first occurrence, rerun).

- **Rollout:** both existing public databases (`dpg-db34l9bor55s73cqpq9g`, `dpg-db9r654flioc739mcpbg`) got Ready `<db>-public-tls` Certificates and `status.externalCertificate: PubliclyTrusted`. `openssl s_client -starttls postgres` on both showed `CN=<db>.db.bex.co`, issuer `Let's Encrypt YR2`, `Verify return code: 0 (ok)`.
- **DoD 1 (copy → connect, no CA):** REST connection-info for `dpg-db9r654flioc739mcpbg` returned `externalConnectionString` with host `<db>.db.bex.co:5432` and an **empty query**, no `serverCaCertificate`, and `psqlCommand` in Render's `PGPASSWORD=… psql -h …` form. On a machine with no `~/.postgresql/root.crt` and `PGSSLROOTCERT` unset, `psql "<url>"` connected (`pg_stat_ssl.ssl = t`). `psql "<url>?sslmode=verify-full&sslrootcert=system"` also connected, so the hostname verifies against system roots.
- **DoD 2 (forced renewal):** set `Issuing=True` (`ManuallyTriggered`, what `cmctl renew` does) on `dpg-db9r654flioc739mcpbg-public-tls`. It moved to revision 2, and the served serial changed `0699C8…` → `0588F3…` with **no Pod restart** (the instance had run 6d2h with 0 restarts; `cnpg.io/reload` worked on CNPG 1.30.1). The same unchanged URL connected again.
- **DoD 3 (HA + internal):** a temporary `1c-2g` HA public database in workspace `bex` (`dpg-dbi5pqg9vnuc73fiqpe0`) became `PubliclyTrusted`, with CNPG `serverTLSSecret`/`serverCASecret` set to `<db>-public-tls`/`<db>-public-ca`. Its replica streamed over TLS (`pg_stat_replication`: `streaming`, ssl `t`). `POST …/failover` → 202 promoted `-2` within about 5 s, and the former primary **rejoined as a streaming TLS replica** (it verified the new primary's Let's Encrypt certificate against the root-only CA). The external URL wrote after failover with no CA file, and the internal URL read from inside the namespace. The database was deleted afterwards (204). A first attempt in `bex-canary` hit a separate defect, the Hobby tenant quota refusing the second HA instance, now filed as `w2/054`. That database was deleted too.
- **DoD 4 (surfaces):** GraphQL `databaseConnectionInfo` matched REST (same host, empty query, `serverCaCertificate: ""`, `PGPASSWORD=` psql form). MCP has no connection-info tool. The dashboard hides the CA block when it is empty (t004 test).

## t006–t010 — 2026-10-30 (closed)

- **t006 docs** (`1a25490cc`, corrected by `ac14b1334`): ADR009 has a new § "Publicly trusted server certificates (w2/m245)" (issuance, CNPG wiring, status, URL shape, `serverCaCertificate`, renewal, cleanup order, Let's Encrypt budget, stale `root.crt` limit), and the External URL row is rewritten. ADR018's connection-info row is updated, and so are the ADR088 tenant line, the CLI checklist and `render-artifacts/psql-cli.md`.
- **t007 Render parity:** REST omits `serverCaCertificate`, matching the pinned `postgresConnectionInfo`. GraphQL returns `""`. MCP has no connection-info tool, as Render's MCP does not. The dashboard hides the CA block. The bex CLI provisions nothing for an empty CA and passes the URL to `psql` unchanged. **URL shape:** Render documents `postgresql://USER:PASSWORD@EXTERNAL_HOST:PORT/DATABASE` (render.com/docs/postgresql-creating-connecting, checked 2026-10-30), so bex keeps `host:5432/db` with no `sslmode`. A subagent commit that dropped the port based on ADR009's portless record was **not** taken. That record was corrected instead (`ac14b1334`).
- **t008 simplify** (`b671ee403`): public SNI names are computed once per reconcile, and a `stringsAsAny` helper replaces three loops.
- **t009 tests** (`8330d2168`): an expired served certificate reverts to CNPG's; a name-set re-issue keeps the database `PubliclyTrusted`; deletion order is covered; foreign same-named objects are never adopted. The backend falls back to `verify-full` + CA (503 if the CA is missing). A CLI end-to-end test checks that a public database gets no `PGSSLROOTCERT`. Mutation spot-checks confirmed the expiry and foreign-object tests catch regressions. Suites pass: `make test`/`make lint`, backend (real-deps rerun green after a known `internal/store` timing flake), CLI, and dashboard with 5693 tests.
- **t010 Closeout:** all five DoD bullets hold, with evidence in `## t005` (production) and the docs above.
