# w3 · m26 — Datastore outbound-webhook event types (Postgres / Key Value)

**Worker:** worker3 **Goal:** Outbound webhook consumers see managed-datastore lifecycle, not just service lifecycle: Render's Postgres/KeyValue-specific event types — explicitly deferred out of w3/m11's initial set and owned nowhere since — are captured from Render's vocabulary and emitted from the datastore lifecycle writes. **Status:** done — DONE 2026-07-15

## Tasks (in order)

| id   | title                                                                    | est | depends_on | status |
| ---- | -------------------------------------------------------------------------- | --- | ---------- | ------ |
| t001 | Capture Render's datastore webhook event-type vocabulary                 | 25m | —          | done — DONE |
| t002 | Instrument Postgres/KeyValue lifecycle writes to emit the events         | 75m | t001       | done — DONE |
| t003 | Docs: ADR006 outbound-webhooks section + ADR018 row evidence             | 20m | t002       | done — DONE |
| t004 | Render parity                                                              | 25m | t003       | done — DONE |
| t005 | Simplify                                                                   | 20m | t004       | done — DONE |
| t006 | Test coverage                                                              | 30m | t004       | done — DONE |
| t007 | Closeout                                                                   | 15m | t006       | done — DONE |

## Definition of done

Creating, suspending/resuming, and deleting a managed Postgres or Key Value fires the Render-named webhook event to registered endpoints (observed via `scripts/webhooks-verify.sh` or a test sink); the event-type registry test covers the new types; ADR006's outbound-webhooks section and the ADR018 webhooks row list the datastore vocabulary as shipped.

## Implementation (w3/m26, 2026-07-15)

**8 new webhook type constants** in `lego/backend/internal/webhooks/service.go`:
`postgres_created`, `postgres_deleted`, `postgres_suspended`, `postgres_resumed`,
`key_value_created`, `key_value_deleted`, `key_value_suspended`, `key_value_resumed`.

**8 new AuditVerb constants** in `lego/backend/internal/core/audit.go` (`AuditVerbCreate/Delete/Suspend/ResumePostgres` + `AuditVerbCreate/Delete/Suspend/ResumeKeyValue`) and 2 post-create recording helpers (`RecordDatabaseCreated`, `RecordKeyValueCreated`). Create uses workspace-level `s.Authorize` (no resource target), so the helpers emit an explicit second event after a successful create with a `DatabaseTarget`/`KeyValueTarget` for the webhook arm to pick up. Delete/Suspend/Resume already emit via `AuthorizeDatabase`/`AuthorizeKeyValue`.

**2 new UNION ALL arms** in `store.webhookEventsQuery` — one for `e.target LIKE 'database:%'`, one for `'keyvalue:%'` — no apps-table join needed (the database/keyvalue name IS the resource ID, returned as both `service_id` and `service_name`).

**Post-create emit** added in `postgres/service.go` (`s.RecordDatabaseCreated(ctx, d)`) and `keyvalue/service.go` (`s.RecordKeyValueCreated(ctx, kv)`).

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 13, 2026-07-15 — closeout residual: `.pm/w3/done/m11/done/t002.md:41` "Render's Postgres/KeyValue-specific event types — deferred, not in this milestone's initial set"; verified still true in code — `lego/backend/internal/webhooks/service.go:86-94` registers only deploy/server/service/cron types. Distinct from open w3/m16 (the events *feed*'s from/to details).
- **Goal linkage:** Render parity (outbound webhooks row, docs/ADR018-render-parity.md) + w3's observability charter (docs/ADR006-bex-api.md § Outbound event webhooks).
- **Expected outcome:** webhook consumers can react to datastore lifecycle (e.g. provision-complete automation) exactly as on Render.
- **Why now:** the only unowned explicit deferral left in w3's shipped surface; the m11 delivery machinery (retries, auto-disable, `BEX_WEBHOOK_BACKOFF` verification path) is all in place — this is vocabulary + emit points only.
- **Render parity:** included — user-facing event surface change (webhook payloads).
