# w2 · m245 — Public Postgres URLs verify against system roots, as on Render

**Worker:** worker2 **Goal:** a tenant who copies a public database's external connection string into any stock Postgres client connects securely with no downloaded CA, and keeps connecting across certificate renewals, as with Render's `*-postgres.render.com` hosts. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                   | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Prove on `dev-2` that CNPG serves a user-provided server certificate while its replication still verifies | 45m | —          |
| t002 | Operator: a publicly trusted certificate per externally reachable database, wired into CNPG and renewed   | 1h  | t001       |
| t003 | External connection string connects as-is from a stock client, Render-shaped                             | 40m | t002       |
| t004 | `serverCaCertificate` semantics on REST, GraphQL, MCP and the dashboard connect panel                     | 30m | t003       |
| t005 | Live acceptance on production: copy → connect, forced renewal → still connects                           | 40m | t004       |
| t006 | Correct ADR009, ADR018 and the connection docs                                                           | 20m | t005       |
| t007 | Render parity                                                                                           | 20m | t006       |
| t008 | Simplify                                                                                                | 15m | t007       |
| t009 | Test coverage                                                                                           | 30m | t007       |
| t010 | Closeout                                                                                                | 10m | t008, t009 |

## Definition of done

- On production, for a public database: `psql "<copied externalConnectionString>"` on a machine with no `~/.postgresql/root.crt` and no `PGSSLROOTCERT` connects with TLS, and the server certificate chains to a publicly trusted root (`openssl s_client -starttls postgres` → `Verify return code: 0`).
- After a forced renewal of that database's server certificate, the same command, unchanged, still connects.
- CNPG replication and internal (`-rw` service) connections keep working on an HA database with the new certificate.
- REST `GET /v1/postgres/{id}/connection-info`, GraphQL `databaseConnectionInfo`, MCP and the dashboard agree on the connection string and on what `serverCaCertificate` means.
- ADR009 / ADR018 describe what shipped.

## Source + Goal linkage

- **Source:** promoted from inbox note `w2/049` (2026-10-29, `/loopx w2`). The note's history is preserved in [source-w2-049.md](source-w2-049.md). Its `/unblock` decision (2026-10-29, Render parity rule): publicly trusted per-database certificates.
- **Evidence:** CNPG-generated tenant CAs rotate about every 90 days, which breaks `verify-full` clients that pinned the CA (w2/m170, Grafana). A copied production URL fails today with `root certificate file "~/.postgresql/root.crt" does not exist` (2026-10-24, w4 QA). Render's hosts present Let's Encrypt certificates (`oregon-postgres.render.com`, `frankfurt-postgres.render.com` → issuer `Let's Encrypt YR1`, verify OK; checked 2026-10-29).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillar 1 (Render compatibility) and [ADR009](../../../docs/ADR009-postgresql-management.md) managed Postgres external access.
- **Expected outcome:** teams migrating from Render paste their connection string and it works. No tenant connection breaks at a CA rotation.
- **Why now:** exposure grows with every public database. The CA cycle already broke a first-party consumer, and the parity decision is recorded.
- **Render parity included:** connection info is read on REST, GraphQL, MCP and the dashboard.

## Design notes (from the `/unblock` record, 2026-10-29)

- Production cert-manager has only HTTP-01 (traefik) ClusterIssuers, and `dpg-*.db.bex.co` answers ACME on port 80. Per-database HTTP-01 certificates therefore need no new credential. A `*.db.bex.co` wildcard would need a Cloudflare DNS-01 token, a user-supplied credential. Use it only if per-host HTTP-01 proves unworkable.
- Let's Encrypt allows 50 **new** certificates per registered domain (`bex.co`) per week, shared with every other `*.bex.co` certificate. Renewals are exempt. That caps new public databases at roughly 50/week. t002 records the budget, and if the wildcard is needed, file it as a follow-up with the exact token scope.
- libpq's `sslmode=verify-full` reads `~/.postgresql/root.crt` unless `sslrootcert=system` (libpq ≥ 16) is given. t003 must therefore choose the URL shape that connects as-is on stock clients, matching Render's exact external URL format.
