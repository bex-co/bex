# w2 · m316 — Pooled Postgres connection strings use port 6432, as on Render

**Worker:** worker2 **Goal:** every pooled connection string bex hands out (connection info on REST/GraphQL/MCP, the operator's pooler Secret behind Blueprint `connectionPoolString`, the dashboard) names PgBouncer's port 6432, as Render's do. Internal and external connections on 6432 reach the pooler, and every URI already handed out on 5432 keeps working. **Status:** blocked — t001, t002, t004–t008 done; the internal half is shipped (`245a2eac9`, `3bc94d538`); t003 waits on the edge-LB cost decision below, then t009/t010

## Tasks (in order)

| id   | title | est | depends_on |
| ---- | ----- | --- | ---------- |
| t001 | The pooler Service serves 6432 beside CNPG's default 5432 — **DONE** | 40m | — |
| t002 | Pooled URIs name 6432: backend connection info and the operator's pooler Secret — **DONE** | 40m | t001 |
| t003 | The edge accepts external pooled connections on 6432 — **BLOCKED** (the edge LB is at its listener limit) | 30m | — |
| t004 | dev-2 walk: internal pooled connections on 6432 and 5432; the Secret and connection info name 6432 — **DONE** | 30m | t001, t002 |
| t005 | ADR009 and ADR018 record the port — **DONE** | 15m | t002, t003 |
| t006 | Render parity — **DONE** | 20m | t004, t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t007 |
| t009 | Production check after deploy: an external pooled connection on 6432 | 20m | t008 |
| t010 | Closeout | 10m | t009 |

## Definition of done

- REST `GET /v1/postgres/{id}/connection-info`, GraphQL and MCP return `internalConnectionPoolString` and `externalConnectionPoolString` on port 6432.
- The operator's `<db>-pooler-app` Secret `uri`, which Blueprint `fromDatabase: connectionPoolString` resolves to, names 6432.
- On dev-2, `psql` through the pooler connects on 6432 and still on 5432.
- On production, an external pooled URL on `<db>-pool.<domain>:6432` connects through the edge, and the old `:5432` form still does.
- ADR009 and ADR018 state the port.

## Source + Goal linkage

- **Source:** promoted from [w2/079](../done/079.md) (w2/m307's Render parity task, 2026-11-10).
- **Render:** "The pooled URLs are almost identical to your direct connection URLs, except they use PgBouncer's port 6432 instead of the standard PostgreSQL port 5432" ([connection pooling](https://render.com/docs/postgresql-connection-pooling)). This applies to both the internal and external pooled URLs.
- **bex today:**
  - Both URI builders write `:5432`: `lego/backend/internal/postgres/service.go` connection info, and `lego/operator/internal/controller/database_controller.go` `reconcilePoolerConnectionSecret`.
  - `poolerSpec` sets no `serviceTemplate`, so CNPG's Pooler Service exposes only its default `pgbouncer` port 5432.
  - Externally, the Hetzner LB forwards only `:5432` to pg-sni-proxy's NodePort 31056 (`infra/terraform/main.tf` `hcloud_load_balancer_service.postgres`), and pg-sni-proxy routes `<db>-pool` hosts by SNI.
- **Design (parity-sufficient, no open decision):**
  - Serve both ports and emit 6432, so nothing already handed out breaks.
  - CNPG adds its default `pgbouncer`/5432 ServicePort unless the template declares one with that name or port. A template port named `pgbouncer-6432`, port 6432 → `targetPort: pgbouncer`, therefore yields both ([CNPG connection pooling](https://cloudnative-pg.io/docs/1.28/connection_pooling/)).
  - PgBouncer keeps listening on container port 5432, so NetworkPolicies, which match the destination pod port, are unchanged.
  - The edge gets a second LB service, listen 6432 → the same NodePort 31056. The proxy is port-agnostic, routing by SNI behind proxy protocol.
  - Retiring `:5432` for pooled traffic is a separate, later decision.
- **Goal linkage:** Render parity first (ADR008), V0 roadmap item 4 (PostgreSQL), ADR018 Postgres access row.
- **Expected outcome:** a pooled URL copied from bex looks exactly like Render's, and Render's guidance ("swap the port to 6432") works on bex.
- **Why now:** w2/m307 just put the pooler toggle in every paid database's dashboard, so pooled URLs are now one click away for every customer. The port difference is visible on the same page as the direct URL.
- **Render parity included:** REST, GraphQL, MCP, the dashboard and Blueprint strings change.

## Blocked: the edge load balancer has no free listener (2026-11-10, `/loopx w2`)

- **What shipped:**
  - The pooler Service serves 6432 beside 5432.
  - The internal pooled string and the `<db>-pooler-app` Secret (Blueprint `connectionPoolString`) name 6432.
  - The **external** pooled string stays on 5432, so nothing handed out points at a port the edge does not serve.
- **What blocks the rest:** Hetzner refused the 6432 listener. The edge LB is `lb11` (5 services), already serving 22/80/443/5432/6379.
- **Decision needed (the user's, because it changes the monthly bill).** Prices are from [Hetzner's load balancer page](https://www.hetzner.com/cloud/load-balancer), 2026-11-10, excluding VAT.

  | Option | Cost | What it takes |
  | --- | --- | --- |
  | **(A, recommended)** Resize `bex-traefik` lb11 → lb21 | €7.49 → €21.49/month (+€14.00) | Hetzner changes the type in place, keeping both IPs. One line (`load_balancer_type = "lb21"`) plus the 6432 listener; no DNS change. Leaves 15 services of headroom for future edge ports. |
  | (B) A second `lb11` for the datastore listeners (5432, 6432, 6379) | +€7.49/month | `*.db.bex.co` and the Key Value hosts must move to the new LB's IP. More Terraform, a DNS cutover, and validator changes. |
  | (C) Keep external pooled URLs on 5432 | €0 | An ADR018 divergence that needs your sign-off. Render's external pooled URLs use 6432. |

- **Render basis:** "The pooled URLs are almost identical to your direct connection URLs, except they use PgBouncer's port 6432" ([connection pooling](https://render.com/docs/postgresql-connection-pooling)), external included.
- **Next steps once decided:**
  - A or B: set the LB type (or add the LB), re-add `hcloud_load_balancer_service.postgres_pooled` (listen 6432 → NodePort 31056), and restore the edge-validator entries. Switch the external pooled string to `appv1alpha1.PoolerPort`. Then run t009 on production: an external pooled connection on 6432 and on 5432, plus connection info.
  - C: record the divergence in ADR018 with your sign-off, drop t003, and run t009 for the internal half only.
- **Unblock:** `/unblock w2`.
