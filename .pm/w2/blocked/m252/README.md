# w2 · m252 — Failure alerts reach email, as on Render

**Worker:** worker2 **Goal:** every failure event Render emails about — a running service becoming unhealthy, a cron run failing, an image pull failing, a Blueprint sync failing — bex emails too, under the existing `notificationsToSend` and member settings. **Status:** blocked — t001–t005, t007–t009 done; t006 (dev-2 + production check) next, then t010

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Correct ADR018's Notifications row with a per-event matrix — **DONE** | 20m | — |
| t002 | Email for server_failed with a flap digest (and server_available per Render's level) — **DONE** | 1.5h | t001 |
| t003 | Email for cron_failed and image_pull_failed — **DONE** | 45m | t001 |
| t004 | Email for Blueprint sync failure — **DONE** | 45m | t001 |
| t005 | Apply notificationsToSend and member settings per event on every surface — **DONE** | 1h | t002, t003, t004 |
| t006 | dev-2 walk and production check | 45m | t005 |
| t007 | Render parity — **DONE** | 15m | t006 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage — **DONE** | 30m | t007 |
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

## Work log — 2026-11-03

- **Design:** `AlertEmailWorker` (`internal/notifications/alert_email_worker.go`) tails the composed event feed (its own `alert_email_watermarks` row, `store.TailFeed`) and failed Blueprint syncs (`blueprint_syncs.state = 'error'`, second watermark), projecting `server_failed`/`server_hardware_failure`, failed `cron_job_run_ended`, `image_pull_failed` and failed syncs into one `alert_email_streams` row per (workspace, member, resource, event). `alert_email_events` records each source event once per member, so a page re-read by a stale replica counts nothing twice. The first failure is due at once; failures within an hour after a mail are folded into one digest that counts them and lists the newest 20 times. Sends lease streams (`FOR UPDATE SKIP LOCKED`), retry transient SMTP/lookup failures with backoff (cap 8), and treat 550/551/553 and no-address as final. Both tables cascade from `tenant_members` (workspace and account-deletion censuses).
- **Policy (t005):** a service's `notificationsToSend` applies exactly as for a failed deploy (`failureNotificationPolicy`, now shared with `NotifyDeploy`); `default` follows each member's existing failure preference, so no new setting was needed. The Settings toggle is relabelled "Failures" / "故障" with a hint naming every event, and the MCP field description matches. Recovery is never emailed (Render: Slack only).
- **Found and fixed:** push "Cron job failed" was driven by `job_run_ended` (retired one-off jobs) and never fired for real cron runs; it now reads `cron_job_run_ended`.
- **t001:** ADR018 Notifications row is ◐ with the per-event matrix (disk 80% → w2/m255, crash-loop suspension → w2/m253).
- **Tests:** `TestAlertEmailWorkerMailsFailures` and `TestAlertEmailStreamLifecycle` (real Postgres), `alert_email_worker_test.go` (classification, policy matrix, copy, send outcomes), push vocabulary tests updated.

## Blocked (2026-11-03)

All implementation is done (t001–t005 and t007–t009 done; t006 (live walk) blocked). The remaining walk needs a gate this session cannot clear: user: (1) choose how the local CAPD cluster runs the egress nft rule (drop it locally, or rework it to bind in nested kind nodes) so dev-2 comes up for the crash / cron / image-pull / Blueprint-failure emails in Mailpit; and (2) approve a paid production canary (cron jobs are paid) plus an inbox to read the alert, for the production repeat. Run `/unblock w2` to decide. The milestone returns to the queue once the gate clears.
