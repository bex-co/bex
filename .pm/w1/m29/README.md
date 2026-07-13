# w1 · m29 — Managed Postgres external connectivity: SNI proxy for preamble-mode TLS clients

**Worker:** worker1 **Goal:** An external client using standard (non-`sslnegotiation=direct`) TLS negotiation can connect to a managed Postgres instance's public endpoint. Today the external endpoint only works for PG 17+ direct-TLS clients — Traefik's TCP/SNI routing can't read SNI from a preamble-mode client's cleartext `SSLRequest`, so most `psql`/driver defaults today can't connect externally at all. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                          | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design: a `pg_sni_proxy`-style component reading the cleartext `SSLRequest` preamble to extract routing info before Traefik's TCP/SNI layer, per `docs/ADR009-postgresql-management.md:53`'s named fix | 45m | —          |
| t002 | Implement and deploy the proxy in front of (or alongside) the existing Traefik TCP/SNI route for external managed-Postgres                             | 1h  | t001       |
| t003 | Wire any connection-info surface changes (host/port) through to REST/GraphQL/MCP if the external endpoint shape changes                               | 30m | t002       |
| t004 | Live verification: a default-mode `psql`/driver (preamble TLS negotiation) connects externally; a PG17+ direct-TLS client still works unchanged        | 30m | t002       |
| t005 | Docs: close the gap in `docs/ADR009-postgresql-management.md`                                                                                          | 15m | t004       |

## Definition of done

An external client using standard (non-`sslnegotiation=direct`) TLS negotiation can connect to a managed Postgres instance's public endpoint, verified live; existing direct-TLS clients are unaffected.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones to work on` 2026-07-13 — `docs/ADR009-postgresql-management.md:53` names the fix (a `pg_sni_proxy`-style proxy) but no milestone had ever tracked building it.
- **Goal linkage:** `GOAL.md` #4 (PostgreSQL) — managed Postgres is a core V0 feature; today its external endpoint silently fails for the majority of real-world client defaults.
- **Expected outcome:** external managed-Postgres connectivity works for standard client TLS negotiation, not just the PG 17+ direct-TLS opt-in.
- **Why now:** the fix is already named in the ADR, unblocked by nothing else, and is a present connectivity gap for anyone trying to connect from outside the cluster with a standard client — not covered by `w1/m22` (HA, DB-internal) or `w8/m5` (metering).
- **Render parity closing task: omitted** — this is bex's own TLS-termination mechanism underneath an already-✅ parity row ("Postgres CRUD + connection-info"), not a new REST/GraphQL/MCP/UI surface; flag for re-confirmation if t003 changes the connection-info shape.
