# w1 · m2 — Control plane: Go service + Postgres source of truth

**Worker:** worker1 **Goal:** Stand up the bex control plane — a Go service backed by Postgres that owns the product's source of truth (tenants/apps/domains/plans) and projects `apps` rows into `App` CRs for the operator to execute. Fixes business data living only in single-node etcd and gives a product API. **Status:** todo (t001 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Provision Postgres (CNPG `bex-db`) — **DONE** | — | — |
| t002 | Scaffold the control-plane Go binary + Deployment + DB conn | 30m | t001 |
| t003 | Core schema + migrations (tenants, apps, domains) | 30m | t002 |
| t004 | Accounts/auth + usage/billing tables | 25m | t003 |
| t005 | Reconciler: project `apps` rows → `App` CRs | 30m | t003 |
| t006 | Minimal API: create tenant / app / domain | 30m | t003 |
| t007 | GitOps-deploy the service + end-to-end acceptance | 30m | t002,t005,t006 |

## Definition of done

Create a tenant + app via the API → an `App` CR appears → the operator deploys it → `status.url` flows back. Business logic lives in this service, not in the operator or in Postgres procedures.

## Source

Converted from `.tmp/005-control-plane-service.md` (see `docs/control-plane.md` for data flow, schema, one-Postgres, tiers).
