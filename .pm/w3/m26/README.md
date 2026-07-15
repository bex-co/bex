# w3 · m26 — Datastore outbound-webhook event types (Postgres / Key Value)

**Worker:** worker3 **Goal:** Outbound webhook consumers see managed-datastore lifecycle, not just service lifecycle: Render's Postgres/KeyValue-specific event types — explicitly deferred out of w3/m11's initial set and owned nowhere since — are captured from Render's vocabulary and emitted from the datastore lifecycle writes. **Status:** todo

## Tasks (in order)

| id   | title                                                                    | est | depends_on |
| ---- | -------------------------------------------------------------------------- | --- | ---------- |
| t001 | Capture Render's datastore webhook event-type vocabulary                 | 25m | —          |
| t002 | Instrument Postgres/KeyValue lifecycle writes to emit the events         | 75m | t001       |
| t003 | Docs: ADR006 outbound-webhooks section + ADR018 row evidence             | 20m | t002       |
| t004 | Render parity                                                              | 25m | t003       |
| t005 | Simplify                                                                   | 20m | t004       |
| t006 | Test coverage                                                              | 30m | t004       |
| t007 | Closeout                                                                   | 15m | t006       |

## Definition of done

Creating, suspending/resuming, and deleting a managed Postgres or Key Value fires the Render-named webhook event to registered endpoints (observed via `scripts/webhooks-verify.sh` or a test sink); the event-type registry test covers the new types; ADR006's outbound-webhooks section and the ADR018 webhooks row list the datastore vocabulary as shipped.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 13, 2026-07-15 — closeout residual: `.pm/w3/done/m11/done/t002.md:41` "Render's Postgres/KeyValue-specific event types — deferred, not in this milestone's initial set"; verified still true in code — `lego/backend/internal/webhooks/service.go:86-94` registers only deploy/server/service/cron types. Distinct from open w3/m16 (the events *feed*'s from/to details).
- **Goal linkage:** Render parity (outbound webhooks row, docs/ADR018-render-parity.md) + w3's observability charter (docs/ADR006-bex-api.md § Outbound event webhooks).
- **Expected outcome:** webhook consumers can react to datastore lifecycle (e.g. provision-complete automation) exactly as on Render.
- **Why now:** the only unowned explicit deferral left in w3's shipped surface; the m11 delivery machinery (retries, auto-disable, `BEX_WEBHOOK_BACKOFF` verification path) is all in place — this is vocabulary + emit points only.
- **Render parity:** included — user-facing event surface change (webhook payloads).
