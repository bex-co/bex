# w1 · m22 — Managed Postgres HA (Render `enableHighAvailability` + failover + read replicas)

**Worker:** worker1 **Goal:** Ship Render-parity high availability for managed Postgres — a replicated CNPG cluster from the `Database` CR, a failover/promote path, and read-replica endpoints/URLs, plus the REST/GraphQL/MCP + dashboard surfaces (so `highAvailabilityEnabled` can finally report `true`). **Status:** todo

## Tasks (in order)

| id   | title                                                                                 | est | depends_on                          |
| ---- | ------------------------------------------------------------------------------------- | --- | ----------------------------------- |
| t001 | HA replicated CNPG cluster from the `Database` CR (`spec.highAvailability` → instances) | 90m | —                                   |
| t002 | Failover / promote path (switchover verb → CNPG; report current primary)               | 60m | w1/m22/t001                         |
| t003 | Read replicas: replica endpoints/URLs (internal + external SNI, read-only)              | 60m | w1/m22/t001                         |
| t004 | REST/GraphQL/MCP HA surface (`enableHighAvailability`, failover/promote, replica URLs)   | 60m | w1/m22/t002,w1/m22/t003             |
| t005 | Dashboard HA UI (HA toggle, failover button, replica connection info)                   | 60m | w1/m22/t004                         |
| t006 | Render parity — verify HA surfaces against Render                                        | 30m | w1/m22/t004,w1/m22/t005             |
| t007 | Simplify — `/simplify` over what this milestone changed                                  | 20m | w1/m22/t006                         |
| t008 | Test coverage — HA reconcile + failover + replica URL tests                              | 40m | w1/m22/t006                         |
| t009 | Closeout — verify DoD holds, then move the milestone to `done/`                          | 10m | w1/m22/t008                         |

## Definition of done

- `Database.spec.highAvailability` (or equivalent) provisions a replicated CNPG cluster (≥2 instances, primary + standby) with synchronous/asynchronous replication; a single-instance DB stays single-instance.
- A failover/promote verb triggers a CNPG switchover and the reported primary changes; connections continue against the primary service.
- Read-replica endpoints/URLs are issued (internal `-ro` service + external Traefik SNI hostname where `BEX_DB_DOMAIN` is set), separate from the primary URL.
- REST/GraphQL/MCP expose `enableHighAvailability`, failover/promote, and replica URLs with Render-identical shape; `highAvailabilityEnabled` reports the real state (no longer hardcoded `false`); the dashboard shows the HA toggle, failover action, and replica connection info.
- `docs/render-parity.md` + `docs/postgresql-management.md` updated.

## Source + Goal linkage

- **Source:** inbox note `w1/013` (split from m17 in the 2026-07-08 reorg; originally m13 audit note `w1/011`); moved to `w1/done/013.md` on promotion.
- **Goal linkage:** pillar 1 (Render parity — managed data).
- **Expected outcome:** tenants can run HA Postgres with failover and read replicas, matching Render's `enableHighAvailability`/`failover`/`replication`.
- **Why now:** the two prerequisites are met — m17 shipped single-instance data-protection/lifecycle/access (2026-07-09), and m19's rebuild landed a real multi-node substrate that a replicated cluster can spread across (its README names this note as unblocked). This is the larger, riskier HA infra work deferred out of m17; sequence after the single-instance guarantees it builds on.
- **Render parity task included:** adds REST/GraphQL/MCP/UI surfaces, so cross-surface parity must be verified.
