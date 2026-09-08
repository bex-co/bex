# w8 · m34 — Disk event wire vocabulary: rename to Render's `disk_created`/`disk_deleted`

**Worker:** worker8 **Goal:** every disk event bex emits is selectable through the Render-shaped `?type=` filter — bex stops rejecting its own vocabulary with 400 **Status:** done

## Tasks (in order)

| id   | title                                                                        | est | depends_on |
| ---- | ---------------------------------------------------------------------------- | --- | ---------- |
| t001 | Rename the emitted types + all backend references — **DONE**                 | 45m | —          |
| t002 | History + subscriptions: old spellings keep rendering, filtering, delivering — **DONE** | 60m | t001       |
| t003 | Dashboard catalog: labels, icons, locales, vocabulary test — **DONE**        | 30m | t001       |
| t004 | Parity-positive filter guard + docs closure — **DONE**                       | 30m | t002, t003 |
| t005 | Render parity: cross-surface consistency check (REST/GraphQL/MCP/UI) — **DONE** | 30m | t004       |
| t006 | Simplify: `/simplify` over the code this milestone changed — **DONE**        | 30m | t005       |
| t007 | Test coverage: meaningful tests for the behavior this milestone shipped — **DONE** | 30m | t005       |
| t008 | Closeout: verify DoD, mark done, move milestone to done/ — **DONE**          | 15m | t007       |

## Definition of done

`GET /v1/services/{id}/events?type=disk_created` and `?type=disk_deleted` return 200 and narrow correctly (they are in Render's pinned 39-value enum), and no event type bex emits in the disk family is unfilterable through the Render-shaped parameter. Historical `service_event_facts`/audit rows recorded under the old spellings still render on all surfaces and still match the renamed filter, and webhook endpoints whose subscribed-type filters carry `disk_attached`/`disk_detached` keep receiving the renamed events (no silently orphaned subscription). `disk_restored` stays a labeled bex extension. `docs/render-artifacts/service-events.md` and the ADR018 events row record the closure.

## Compatibility mechanism (t002)

**Chosen: one-time migration + thin write/match aliases (no permanent stored dual vocabulary).**

- Service-feed history needs **no row rewrite**: disk types are projected from audit verbs (`apps.AddDisk` / `apps.DeleteDisk`) at read time, so renaming the constants is enough for historical audit rows.
- Live webhook `event_types` filters are rewritten by migration `0106_disk_event_type_rename` (`disk_attached`→`disk_created`, `disk_detached`→`disk_deleted`, de-duped). Immutable attempt/delivery payloads stay byte-unchanged.
- Create/Update still accept the legacy spellings and store the canonical names; dispatch also matches a not-yet-migrated filter so subscriptions are never silently orphaned across the rollout window.
- Persistent disks (ADR082) are no longer an outbound-webhook anti-goal: `disk_created`/`disk_updated`/`disk_deleted` are advertised and delivered. `disk_restored` stays service-feed-only.

## Source + Goal linkage

- **Source:** `w6/068` (filed 2026-08-27 by the w6/m122/t004 parity probe, triaged 2026-09-03 with option 1 recommended; retired to `w6/done/068.md` pointing here). Ratified as `/pm-brainstorm for w8` 2026-09-07 #2, approved same day.
- **Goal linkage:** Render parity (ADR006/ADR018) and correctness of bex's own event surface — Render already named these concepts (`disk_created`/`disk_updated`/`disk_deleted` in its events enum); bex chose different words for the same things and its own validator now rejects them.
- **Expected outcome:** the proven 400-on-own-vocabulary defect (`TestServiceEventSurfaceCarriesDriftedTypes`) is gone; the drift test becomes a parity-positive guard.
- **Why now:** the note was blocked only on ratifying the recorded recommendation, and every day of emission grows the old-spelling row set the compatibility layer must cover.
- **Render parity closing task included:** the rename is wire-visible across REST, GraphQL, MCP, webhooks, and the dashboard Events tab.

## Closeout evidence (t005 / t008)

- REST/GraphQL/MCP list the same `disk_created`/`disk_deleted`/`disk_restored` strings for the same fixture (`TestServiceEventSurfaceCarriesDiskAndBexNamedTypes`).
- `?type=disk_created|disk_deleted|disk_updated` → 200 + verb push-down; `disk_restored` and legacy `disk_attached`/`disk_detached` → 400.
- Webhook vocabulary fixture + `TestDiskEventTypesAreSubscribableAndLegacyAliasesNormalize` + migration `TestDiskEventTypeRenameMigration`.
- Dashboard catalog, icons, en/zh locales, and `backend-vocabulary.test.ts` assert the new set.
