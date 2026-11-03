# w2 · m252 — Failure alerts reach email, as on Render

**Worker:** worker2 **Goal:** every failure event Render emails about — a running service becoming unhealthy, a cron run failing, an image pull failing, a Blueprint sync failing — bex emails too, under the existing `notificationsToSend` and member settings. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Correct ADR018's Notifications row with a per-event matrix | 20m | — |
| t002 | Email for server_failed with a flap digest (and server_available per Render's level) | 1.5h | t001 |
| t003 | Email for cron_failed and image_pull_failed | 45m | t001 |
| t004 | Email for Blueprint sync failure | 45m | t001 |
| t005 | Apply notificationsToSend and member settings per event on every surface | 1h | t002, t003, t004 |
| t006 | dev-2 walk and production check | 45m | t005 |
| t007 | Render parity | 15m | t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 30m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

- On production, a workspace member with **no push device** receives an email for: a crashing (unhealthy) service, a failed cron run, a failed image pull, and a failed Blueprint sync.
- A service flapping failed/available 20 times in an hour produces a digest, not 20 emails; no failure is silently dropped (the digest lists them).
- With `notificationsToSend: none` on the service, none of these emails is sent; with `failure` they are; recovery (`server_available`) email follows Render's documented level.
- `docs/ADR018-render-parity.md` Notifications row carries a per-event matrix against render.com/docs/notifications and is `✅` only where true.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-11-02 #2. render.com/docs/notifications (fetched 2026-11-02) emails on: build/deploy failure, Docker image pull failure, cron job failure, a running service becoming unhealthy, persistent disk usage >80% (and back below), Blueprint sync failure, and suspension after repeated startup failures. bex email today covers deploy lifecycle, billing, free hours and database expiry only (`lego/backend/internal/notifications/service.go`); `server_failed`/`cron_failed` reach push only. Absorbs FUTURE-MAYBE "Availability-alert delivery + CPU-throttle visibility" part (a) (w2/028; eden-cms-v2 logged 1,399 failed/available pairs).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); V0 goal 2 (basic obs for operation).
- **Expected outcome:** tenants learn about outages without installing the mobile app or enabling web push.
- **Why now:** ADR018 currently claims `✅` for notifications while email is deploy-only; the old deferral trigger predates the parity-first rule (2026-10-17/20), under which an unsigned divergence does not stand. w2/m253 and w2/m255 reuse this email path.
- **Render parity included:** notification settings and email behavior are tenant-facing on REST, GraphQL, MCP and the dashboard.
