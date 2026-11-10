# w2 · m316 — Pooled Postgres connection strings use port 6432, as on Render

**Worker:** worker2 **Goal:** every pooled connection string bex hands out (connection info on REST/GraphQL/MCP, the operator's pooler Secret behind Blueprint `connectionPoolString`, the dashboard) names PgBouncer's port 6432, as Render's do. Internal and external connections on 6432 reach the pooler, and every URI already handed out on 5432 keeps working. **Status:** todo (t001–t008 done; t009 waits on the production deploy)

## Tasks (in order)

| id   | title | est | depends_on |
| ---- | ----- | --- | ---------- |
| t001 | The pooler Service serves 6432 beside CNPG's default 5432 — **DONE** | 40m | — |
| t002 | Pooled URIs name 6432: backend connection info and the operator's pooler Secret — **DONE** | 40m | t001 |
| t003 | The edge accepts external pooled connections on 6432 — **DONE** | 30m | — |
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
