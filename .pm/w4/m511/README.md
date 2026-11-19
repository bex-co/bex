# w4 · m511 — A read replica that stops syncing with its primary records Render's `postgres_read_replica_stale`

**Worker:** worker4 **Goal:** a named read replica whose streaming from its primary stops records one `postgres_read_replica_stale` per episode on REST, GraphQL, MCP, webhooks and the dashboard, as Render records it. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Choose and prove the stale-replica signal on dev-4 | 50m | — |
| t002 | The operator reports a stale read replica on its status | 60m | t001 |
| t003 | Record each stale episode as a datastore fact and serve it | 50m | t002 |
| t004 | Show the event in the dashboard | 25m | t003 |
| t005 | Correct the docs that say the event has no producer | 20m | t003 |
| t006 | Verify a stale replica on dev-4 | 40m | t004, t005 |
| t007 | Render parity | 20m | t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 30m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

1. On dev-4, with a paid Postgres and one named read replica, breaking the replica's streaming for longer than the recorded threshold records exactly one `postgres_read_replica_stale` for that database on REST list and retrieve, GraphQL and MCP, with `details: {}`, and a subscribed webhook receives it.
2. Restoring streaming records nothing more; breaking it again records a second event; creating a replica (initial sync) and a primary failover record none.
3. The signal, threshold and recovery rule are in ADR009; `docs/render-artifacts/webhooks-api.md` and ADR018 describe the type as produced.
4. `cd lego/backend && go test ./...`, `make test` and `make lint` in `lego/operator/`, and dashboard `yarn lint` + `yarn test` pass.

## Source + Goal linkage

- **Renumbered:** from `w4/m506` on 2026-11-19 (`w3` filed `m505–m509` first, and milestone numbers are global).
- **Source:** `/pm-brainstorm one aspect of render parity for w4` (the service events and outbound webhooks aspect), 2026-11-18, item 5; filed by `/pm them to w4`, 2026-11-19.
- **Render reference:** render.com/docs/webhooks, read 2026-11-18: `postgres_read_replica_stale` "A Render Postgres read replica has stopped syncing with its primary instance" (Render adds: contact support). The pinned `retrieve-event` member has no fields. render.com/docs/notifications sends no email for it.
- **bex today:** read replicas since w1/m22, each a one-instance CNPG replica cluster streaming from the primary (`lego/types/v1alpha1/database_types.go:282-292`); replication lag charted from `cnpg_pg_replication_lag` (`lego/backend/internal/metrics/datastore.go:67`). `w4/done/m379/README.md:123` and `w2/done/m171/done/t004.md:40` record "no producer", and `docs/render-artifacts/webhooks-api.md` files the type under "No truthful producer".
- **Goal linkage:** ADR008 Render parity first; ADR009 managed Postgres; ADR018's HA · failover · read replicas and Outbound event webhooks rows.
- **Expected outcome:** a replica that silently stops syncing, and keeps serving stale reads, is reported the way Render reports it.
- **Why now:** the signal already exists; this is the last Postgres type in Render's enum with a source bex can observe. Lowest of the five filed items, since the event is rare.
- **Render parity:** included: the change touches REST, GraphQL, MCP, webhooks and the dashboard.
