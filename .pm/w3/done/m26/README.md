# w3 · m26 — Datastore outbound-webhook event types (Postgres / Key Value)

**Worker:** worker3 **Goal:** Outbound webhook consumers see sourceable managed-datastore lifecycle, not just service lifecycle: Render's Postgres/KeyValue-specific event types — explicitly deferred out of w3/m11's initial set and owned nowhere since — are captured from Render's vocabulary and emitted from successful datastore lifecycle writes. **Status:** done — DONE 2026-07-15

## Tasks (in order)

| id   | title                                                                    | est | depends_on | status      |
| ---- | ------------------------------------------------------------------------ | --- | ---------- | ----------- |
| t001 | Capture Render's datastore webhook event-type vocabulary                 | 25m | —          | done — DONE |
| t002 | Instrument Postgres/KeyValue lifecycle writes to emit the events         | 75m | t001       | done — DONE |
| t003 | Docs: ADR006 outbound-webhooks section + ADR018 row evidence             | 20m | t002       | done — DONE |
| t004 | Render parity                                                            | 25m | t003       | done — DONE |
| t005 | Simplify                                                                 | 20m | t004       | done — DONE |
| t006 | Test coverage                                                            | 30m | t004       | done — DONE |
| t007 | Closeout                                                                 | 15m | t006       | done — DONE |

## Definition of done

Every managed-datastore transition for which Render publishes an exact type and bex has a truthful durable source fires one post-success webhook effect: Postgres create/restart/credential create/credential delete/backup start, plus plan changes for services/Postgres/Key Value. Registry, write-effect, store-feed, and thin-payload tests cover the chain; ADR006 and ADR018 list the shipped set and honest omissions. Render publishes no datastore suspend/resume/delete event type and no Key Value create type, so the original create/suspend/resume/delete premise was corrected rather than satisfied with invented names.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 13, 2026-07-15 — closeout residual: `.pm/w3/done/m11/done/t002.md:41` "Render's Postgres/KeyValue-specific event types — deferred, not in this milestone's initial set"; verified still true in code — `lego/backend/internal/webhooks/service.go:86-94` registers only deploy/server/service/cron types. Distinct from open w3/m16 (the events _feed_'s from/to details).
- **Goal linkage:** Render parity (outbound webhooks row, docs/ADR018-render-parity.md) + w3's observability charter (docs/ADR006-bex-api.md § Outbound event webhooks).
- **Expected outcome:** webhook consumers can react to datastore lifecycle (e.g. provision-complete automation) exactly as on Render.
- **Why now:** the only unowned explicit deferral left in w3's shipped surface; the m11 delivery machinery (retries, auto-disable, `BEX_WEBHOOK_BACKOFF` verification path) is all in place — this is vocabulary + emit points only.
- **Render parity:** included — user-facing event surface change (webhook payloads).

## Completion

Completed 2026-07-15. The current official Render webhook page was captured in [`docs/render-artifacts/datastore-webhook-events.md`](../../../../docs/render-artifacts/datastore-webhook-events.md). Successful datastore writes append fixed audit effects carrying immutable `dpg-…`/`red-…` ids and transition-time display names; the existing durable worker projects them into Render's thin payload. `go build ./... && go test ./...` passes from `lego/backend`.
