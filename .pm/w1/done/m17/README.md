# w1 · m17 — Managed Postgres advanced: data-protection + lifecycle + access

**Worker:** worker1 **Goal:** Close the credibility gap in "managed" Postgres — backups + point-in-time recovery, the suspend/resume/restart lifecycle, and access/connection controls (IP allowlist, PgBouncer pooler, Postgres users) — over CNPG, across REST/GraphQL/MCP and the database detail page. **Status:** done — 2026-07-09

## Tasks (in order)

| id   | title                                                                                  | est | depends_on             |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Backups + PITR/recovery — CNPG backups → object storage + recovery-info/recover/exports | 50m | — — **DONE**           |
| t002 | Lifecycle — suspend / resume / restart verbs (Core + REST/GraphQL/MCP)                   | 40m | — — **DONE**           |
| t003 | Access & connection — IP allowlist + PgBouncer pooler strings + Postgres users          | 45m | — — **DONE**           |
| t004 | Dashboard — Recovery tab + lifecycle actions + access-control on the database detail     | 45m | t001, t002, t003 — **DONE** |
| t005 | Render parity — advanced Postgres across REST/GraphQL/MCP/UI vs render.com               | 20m | t004 — **DONE**        |
| t006 | Simplify — `/simplify` over what this milestone changed                                  | 20m | t005 — **DONE**        |
| t007 | Test coverage — meaningful tests for recovery + lifecycle + access                       | 30m | t005 — **DONE**        |
| t008 | Closeout                                                                                 | 10m | t007 — **DONE**        |

## Definition of done

A managed Postgres instance supports backups + point-in-time recovery (restore to a NEW instance), suspend/resume/restart, an IP allowlist gating the external URL, PgBouncer pooler connection strings, and Postgres user management — exposed over REST/GraphQL/MCP and the database detail page (a Recovery section, lifecycle actions, and access-control), with parity checked vs render.com. `make test` + dashboard tests green. **HA / failover / read replicas are explicitly out of scope** (deferred to inbox note `w1/013` — they need more infra).

## Source + Goal linkage

- **Source:** inbox note `w1/011` (m13 audit), the ✖ "Managed Postgres" advanced rows in `docs/render-parity.md` (→ w1/m17). HA/replicas split to `w1/013`.
- **Goal linkage:** pillar 1 (Render parity — managed data).
- **Expected outcome:** "managed Postgres" becomes credible — a user can recover from mistakes (PITR), pause a DB, lock down access, and pool connections, as on Render.
- **Why now:** the audit ranked data-protection as the top managed-Postgres gap; the connection-info shape already stubs the pooler fields and the etcd/OpenBao backup runbooks are the object-storage precedent to reuse.
- **Render parity INCLUDED:** this milestone adds REST/GraphQL/MCP verbs + dashboard sections — the standing Render-parity task checks them against render.com's Postgres surfaces.
