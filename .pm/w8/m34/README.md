# w8 · m34 — Disk event wire vocabulary: rename to Render's `disk_created`/`disk_deleted`

**Worker:** worker8 **Goal:** every disk event bex emits is selectable through the Render-shaped `?type=` filter — bex stops rejecting its own vocabulary with 400 **Status:** todo

## Tasks (in order)

| id   | title                                                                        | est | depends_on |
| ---- | ---------------------------------------------------------------------------- | --- | ---------- |
| t001 | Rename the emitted types + all backend references                            | 45m | —          |
| t002 | History + subscriptions: old spellings keep rendering, filtering, delivering | 60m | t001       |
| t003 | Dashboard catalog: labels, icons, locales, vocabulary test                   | 30m | t001       |
| t004 | Parity-positive filter guard + docs closure                                  | 30m | t002, t003 |
| t005 | Render parity: cross-surface consistency check (REST/GraphQL/MCP/UI)         | 30m | t004       |
| t006 | Simplify: `/simplify` over the code this milestone changed                   | 30m | t005       |
| t007 | Test coverage: meaningful tests for the behavior this milestone shipped      | 30m | t005       |
| t008 | Closeout: verify DoD, mark done, move milestone to done/                     | 15m | t007       |

## Definition of done

`GET /v1/services/{id}/events?type=disk_created` and `?type=disk_deleted` return 200 and narrow correctly (they are in Render's pinned 39-value enum), and no event type bex emits in the disk family is unfilterable through the Render-shaped parameter. Historical `service_event_facts`/audit rows recorded under the old spellings still render on all surfaces and still match the renamed filter, and webhook endpoints whose subscribed-type filters carry `disk_attached`/`disk_detached` keep receiving the renamed events (no silently orphaned subscription). `disk_restored` stays a labeled bex extension. `docs/render-artifacts/service-events.md` and the ADR018 events row record the closure.

## Source + Goal linkage

- **Source:** `w6/068` (filed 2026-08-27 by the w6/m122/t004 parity probe, triaged 2026-09-03 with option 1 recommended; retired to `w6/done/068.md` pointing here). Ratified as `/pm-brainstorm for w8` 2026-09-07 #2, approved same day.
- **Goal linkage:** Render parity (ADR006/ADR018) and correctness of bex's own event surface — Render already named these concepts (`disk_created`/`disk_updated`/`disk_deleted` in its events enum); bex chose different words for the same things and its own validator now rejects them.
- **Expected outcome:** the proven 400-on-own-vocabulary defect (`TestServiceEventSurfaceCarriesDriftedTypes`) is gone; the drift test becomes a parity-positive guard.
- **Why now:** the note was blocked only on ratifying the recorded recommendation, and every day of emission grows the old-spelling row set the compatibility layer must cover.
- **Render parity closing task included:** the rename is wire-visible across REST, GraphQL, MCP, webhooks, and the dashboard Events tab.
