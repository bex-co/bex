# 049 — Do tenants' `verify-full` Postgres connections break at every CNPG CA renewal?

Why: ADR009 tells tenants to connect to a public Postgres URL with `sslmode=verify-full` using the server CA bex hands out (`serverCaCertificate`, w4/m95), but that CA is CNPG-generated and renews about every 90 days — if a renewal swaps the CA, every customer who followed the docs loses their database connection until they re-download it (exactly what broke Grafana's analytics readers, `w2/m170`).

**Status:** unverified inference — test before any fix.

## Evidence so far

- `docs/ADR009-postgresql-management.md` (External URL row): `verify-full` against CNPG's generated server certificate; clients must install the database's server CA, available through REST `GET /v1/postgres/{id}/connection-info` / GraphQL `databaseConnectionInfo` (`serverCaCertificate`).
- `bex-db` (also CNPG) showed CA, server and replication certificates all expiring together (2026-12-30), and Grafana's pinned copy of the previous CA failed `verify-full` with "certificate has expired" (2026-10-19).

## Do

1. On a dev stack: create a public tenant Postgres, connect with `verify-full` + the downloaded CA, force a CNPG CA renewal, reconnect. Record whether the old CA still verifies.
2. Check Render's behavior for external Postgres URLs (which CA signs them; whether `verify-full` needs a downloaded CA).
3. If it breaks: propose the fix as a milestone (likely publicly trusted certificates for `*.db.bex.co`, so tenants verify against system roots and never pin a CA) — Render parity applies.

## Acceptance

- [ ] Dev result recorded either way; a milestone filed only if the break is real.

## Triage 2026-10-20 — premise confirmed; BLOCKED on a design decision

Read-only on `hetzner-prod`: every tenant CNPG cluster (5) has a CNPG-generated CA whose CA, server and replication certificates expire together on the same ~90-day cycle as `bex-db` (e.g. the two public databases, with `spec.certificates.serverAltDNSNames`: `dpg-db34l9bor55s73cqpq9g` → 2027-01-05, `dpg-db9r654flioc739mcpbg` → 2027-01-15). That is the mechanism under which Grafana's pinned copy of `bex-db`'s previous CA failed `verify-full` with "certificate has expired" (w2/m170), so a tenant who follows ADR009 (`verify-full` + downloaded `serverCaCertificate`) loses the connection at the next renewal and must re-download the CA. Exposure today is small (two public databases, in `bex-canary` and the first-party workspace), but grows with every public database.

The dev experiment is no longer needed to establish the break. What remains is a product decision, with these options:

1. **Publicly trusted certificates for `*.db.bex.co`** (cert-manager DNS-01, already used for `*.onbex.co`): tenants verify against system roots and never pin a CA. Needs proof that CNPG accepts a user-provided server certificate while its own replication still verifies — the strongest fix, likely closest to Render (unverified).
2. **Long-lived per-database CA** (cert-manager CA via `spec.certificates`): tenants pin once for years.
3. **Change the documented guidance** to `sslmode=require` and stop handing out a CA — cheapest, weakest.

Recommendation: (1), as its own milestone with Render parity, after confirming what Render's external Postgres URLs present.

## Live evidence 2026-10-24 (w4 `/qa-find-bugs` loop run p2c6)

- On production, the copied `externalConnectionString` of the public free database `dpg-db9r654flioc739mcpbg` (`qa-20261017-64552e-pg14`, read-only probe, no writes) fails with a stock client. `psql "<copied URL>"` → `connection to server at "dpg-db9r654flioc739mcpbg.db.bex.co" … failed: root certificate file "~/.postgresql/root.crt" does not exist`. The URL ends `?sslmode=verify-full` (`lego/backend/internal/postgres/service.go:944`), so a user who copies it into any Render-targeted tool cannot connect until they download `serverCaCertificate` and set `sslrootcert`/`PGSSLROOTCERT`.
- Render's copied external URL connects as is, because its hosts present publicly trusted certificates (ADR018 Postgres row, w4/m95 note). So the CA-renewal question is not the only cost. Every first connection from outside the dashboard instructions already diverges from Render.
- Under AGENTS.md's parity rule ("never park parity work on an optional decision; ship the parity-sufficient choice"), option 1 (publicly trusted `*.db.bex.co` certificates) is the parity-sufficient choice. Options 2–3 would be stricter or looser follow-ups.

## Unblock 2026-10-29 — decided by existing rule; resumed

- **Affected:** `w2/049`. **Source:** the existing rule (AGENTS.md Render parity: "never park parity work on an optional decision; ship the parity-sufficient choice") plus evidence verified by `/unblock` (main `7fd6c9c4f`, read-only `hetzner-prod`). No new user decision was needed.
- **Decision:** option 1, **publicly trusted server certificates for tenant Postgres hosts (`<dpg-id>.db.bex.co`)**, so the copied `externalConnectionString` (`sslmode=verify-full`) works with a stock client, no downloaded CA needed, as on Render. Options 2 (long-lived per-database CA) and 3 (`sslmode=require`) are not adopted. If they are ever wanted, file them as separate follow-ups.
- **Feasibility facts (verified 2026-10-29):** prod cert-manager has only `letsencrypt-prod`/`-staging` ClusterIssuers with an **HTTP-01** (traefik) solver. There is no DNS-01 solver, and no Cloudflare API token in `.env`. `dpg-db9r654flioc739mcpbg.db.bex.co` resolves directly (`49.12.20.236`, not Cloudflare-proxied), and `http://<host>/.well-known/acme-challenge/…` answers on port 80 (404 from the edge). So per-database HTTP-01 certificates need no new credential. A `*.db.bex.co` wildcard would need a Cloudflare DNS-01 token, which is a user-supplied credential. Choose it only if per-host HTTP-01 proves unworkable.
- **Remaining gate:** none known. If the design needs a DNS-01 token, park that sub-step only, and say exactly which token and scope are needed.
- **Worker next steps:** (1) Confirm what Render's external Postgres hosts present (publicly trusted chain, which CA). (2) Prove on `dev-2` that CNPG accepts a user-provided server certificate (`spec.certificates.serverTLSSecret` + `serverCASecret`) while replication still verifies (CNPG's own client CA). (3) File the implementation as a w2 milestone under the `/pm` sizing rule, with the Render parity task. It should cover the operator issuing/renewing the cert per public database, the connection string keeping `verify-full` with no `sslrootcert`, `serverCaCertificate` semantics on REST/GraphQL/MCP/dashboard, and ADR009/ADR018 amendments. Then close this note as superseded by that milestone.
- **Acceptance:** on `dev-2`, a fresh public database's copied URL connects with `psql "<url>"` on a machine with no `~/.postgresql/root.crt`. After a forced CNPG certificate renewal it still connects with no client change.

**Promoted 2026-10-29 → [`w2/m245`](README.md)** (`/loopx w2`). Larger than an hour across several tasks. Render check done: its hosts present Let's Encrypt certificates.
