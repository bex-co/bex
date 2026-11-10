# w3 · m317 — Disk, maintenance-mode and auto-deploy events carry the details Render requires

**Worker:** worker3 **Goal:** `disk_created`, `disk_updated`, `disk_deleted`, `maintenance_mode_enabled`, `maintenance_mode_uri_updated` and `auto_deploy_disabled` events carry every field their Render member requires, on REST, MCP and GraphQL **Status:** done 2026-11-10

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Audit rows store the disk, maintenance-URI and auto-deploy facts — **DONE** | 1h | — |
| t002 | The write paths record them — **DONE** | 1h | t001 |
| t003 | Events project them in Render's keys; retire the allowlist lines — **DONE** | 1h | t002 |
| t004 | Render parity — **DONE** | 30m | t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 40m | t004 |
| t007 | Closeout — **DONE** | 10m | t005, t006 |

## Definition of done

- A disk added, resized and deleted through the API projects `disk_created {diskId, sizeGB}`, `disk_updated {diskId, fromSizeGB, toSizeGB}` and `disk_deleted {diskId}`; a maintenance-mode toggle projects `maintenance_mode_enabled {enabled}`, a URI change `maintenance_mode_uri_updated {fromURI, toURI}`; turning auto-deploy off in settings projects `auto_deploy_disabled {reason: "setting_change", fromTrigger}` and a dashboard rollback `{reason: "rollback", fromTrigger}`; turning it on projects `auto_deploy_enabled {newTrigger}`
- `TestRenderEventDetailsMatchTheirMember` lists none of those fields as missing; `rendertest.Allowlist` has no `disk_created`/`disk_deleted` entry, and `TestAllowlistOnlyShrinks` pins the lower size
- Rows written before the migration keep answering as they do today: a field the row never recorded is omitted, never invented
- GraphQL `serviceEvent(s)` and MCP `get_service_event`/`list_service_events` expose the same facts as REST

## Source + Goal linkage

- **Source:** promoted from inbox note `061` (2026-11-10), found by w3/m302 while matching every event type to its Render member; Render's pinned `/events/{eventId}` members "Disk Created", "Disk Updated", "Disk Deleted", "Maintenance Mode Enabled", "Maintenance Mode URI Updated", "Auto Deploy Disabled" (`reason`: manual_deploy, rollback or setting_change; optional `fromTrigger`) and "Auto Deploy Enabled" (optional `newTrigger`)
- **Goal linkage:** pillar 1, Render-compatible REST (ADR006); Render parity first (ADR008); ADR018 Events row
- **Expected outcome:** a Render SDK decoder (`AsDiskUpdatedDetails()` and the rest) reads these events whole, and a webhook consumer hydrating one through `retrieve-event` sees what changed
- **Why now:** m302 just made every other event type match its member, so these six are the last event types whose required fields bex drops; each day without the columns is another day of rows that can never carry the facts
- **Render parity:** included — the change is on the REST, MCP and GraphQL event surfaces. bex has no path that disables auto-deploy on a manual deploy, so `manual_deploy` is never written.

## Outcome (2026-11-10)

- Migration 0222 adds `disk_size_from_gb`, `maintenance_uri_from`/`_to`, `auto_deploy_reason` and `auto_deploy_trigger` to `audit_events`; `core.RecordDiskChanged` (now also the snapshot restore's recorder), `RecordMaintenanceModeEffects` and `RecordAutoDeployEnabled`/`RecordAutoDeployDisabled` write them, and an unchanged auto-deploy re-save records nothing.
- The feed attaches them in one read per page (`store.attachAuditChanges`); the by-id lookup selects them off the audit row it already joins. `events.view`/`toRenderEvent` send Render's `diskId`, `sizeGB`, `fromSizeGB`/`toSizeGB`, `enabled`, `fromURI`/`toURI`, `reason` + `fromTrigger`, `newTrigger`; GraphQL `ServiceEventDetails` gains `sizeGB`, `fromSizeGB`, `maintenanceEnabled`, `fromURI`, `toURI`, `autoDeployTrigger`, `autoDeployDisabledReason`.
- `TestRenderEventDetailsMatchTheirMember` has no missing entry for the six types; the `disk_created`/`disk_deleted` allowlist lines are gone (`allowlistSize` 112 → 109; `disk_snapshot_restored` stays for pre-w2/064 rows). ADR018's events row and `docs/render-artifacts/service-events.md` record it.
- Render parity (t004): REST and MCP share `toRenderEvent`; `TestServiceEventSurfaceCarriesDiskAndBexNamedTypes` reads the same disk details on REST, GraphQL and MCP. The dashboard Events tab renders no per-type details for any audit event, so it is unchanged. Not filed: a rollback of a legacy image-backed service with `spec.autoDeploy` still set reports `fromTrigger: commit` where the service view says `off`.
- Tests: `TestPGAuditChangeDetailsRoundTrip` (real Postgres, feed and by-id), `TestDiskVerbsRecordRendersDiskDetails`, `TestAutoDeployAuditCarriesRendersTriggerAndReason`, the rollback and maintenance-URI audit assertions, `TestChangeEventDetailsCarryTheRecordedFacts` (values and older rows).
