# w1 · m22 — Managed Postgres HA (Render `enableHighAvailability` + failover + read replicas)

**Worker:** worker1 **Goal:** Ship Render-parity high availability for managed Postgres — a replicated CNPG cluster from the `Database` CR, a `failover` path, and Render-shaped `readReplicas` (an array of **named replica resources**, independent of the HA toggle), plus the REST/GraphQL/MCP + dashboard surfaces (so `highAvailabilityEnabled` can finally report `true`). **Status:** done — **DONE 2026-07-12**

> **Parity reference (2026-07-12):** field names verified against Render's live API (`api-docs.render.com`). Create body: `enableHighAvailability` (bool, default `false`) and `readReplicas` (array of objects) — **two independent fields**, not one toggle. Failover: `POST /v1/postgres/{postgresId}/failover` → `202` (async; no `promote` verb exists — that was a phantom). Read model reports `highAvailabilityEnabled`. t001 captures the Postgres OpenAPI into `docs/render-artifacts/` so this stays evidence-backed.

## Tasks (in order)

| id   | title                                                                                 | est | depends_on                          |
| ---- | ------------------------------------------------------------------------------------- | --- | ----------------------------------- |
| t001 | HA replicated CNPG cluster from the `Database` CR (`spec.highAvailability` → instances) + capture Render Postgres OpenAPI | 90m | —                                   | — **DONE** |
| t002 | Failover path (`POST /v1/postgres/{id}/failover` → CNPG switchover; report current primary) | 60m | w1/m22/t001                         | — **DONE** |
| t003 | Read replicas: Render `readReplicas` named array → per-replica endpoints/URLs (internal + external SNI, read-only) | 60m | w1/m22/t001                         | — **DONE** |
| t004 | REST/GraphQL/MCP HA surface (`enableHighAvailability`, `failover`, `readReplicas`)       | 60m | w1/m22/t002,w1/m22/t003             | — **DONE** |
| t005 | Dashboard HA UI (HA toggle, failover button, replica connection info)                   | 60m | w1/m22/t004                         | — **DONE** |
| t006 | Render parity — verify HA surfaces against Render                                        | 30m | w1/m22/t004,w1/m22/t005             | — **DONE** |
| t007 | Simplify — `/simplify` over what this milestone changed                                  | 20m | w1/m22/t006                         | — **DONE** |
| t008 | Test coverage — HA reconcile + failover + replica URL tests                              | 40m | w1/m22/t006                         | — **DONE** |
| t009 | Closeout — verify DoD holds, then move the milestone to `done/`                          | 10m | w1/m22/t008                         | — **DONE** |

## Definition of done

- Render's Postgres OpenAPI (HA/failover/read-replica shapes) is captured into `docs/render-artifacts/` as the parity reference — field names verified against it, not asserted.
- `Database.spec.highAvailability` (or equivalent) provisions a replicated CNPG cluster (≥2 instances, primary + standby) with synchronous/asynchronous replication; a single-instance DB stays single-instance.
- A `failover` verb (Render `POST /v1/postgres/{id}/failover`, `202`/async) triggers a CNPG switchover and the reported primary changes; connections continue against the primary service. (No `promote` verb — Render has none.)
- Read replicas are modeled as Render's `readReplicas`: an array of **named replica resources**, each with its own connection URL (internal + external Traefik SNI hostname where `BEX_DB_DOMAIN` is set), distinct from the primary URL — and **independent of the HA toggle** (a DB can have read replicas without HA, matching Render's two-field model).
- REST/GraphQL/MCP expose `enableHighAvailability`, `failover`, and `readReplicas` with Render-identical shape; `highAvailabilityEnabled` reports the real state (no longer hardcoded `false`); the dashboard shows the HA toggle, failover action, and per-replica connection info.
- `docs/ADR018-render-parity.md` + `docs/ADR009-postgresql-management.md` updated.

## Source + Goal linkage

- **Source:** inbox note `w1/013` (split from m17 in the 2026-07-08 reorg; originally m13 audit note `w1/011`); moved to `w1/done/013.md` on promotion.
- **Goal linkage:** pillar 1 (Render parity — managed data).
- **Expected outcome:** tenants can run HA Postgres with failover and read replicas, matching Render's `enableHighAvailability` / `failover` / `readReplicas`.
- **Why now:** the two prerequisites are met — m17 shipped single-instance data-protection/lifecycle/access (2026-07-09), and m19's rebuild landed a real multi-node substrate that a replicated cluster can spread across (its README names this note as unblocked). This is the larger, riskier HA infra work deferred out of m17; sequence after the single-instance guarantees it builds on.
- **Render parity task included:** adds REST/GraphQL/MCP/UI surfaces, so cross-surface parity must be verified.
