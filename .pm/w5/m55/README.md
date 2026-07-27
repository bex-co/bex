# w5 · m55 — Datastore detail-page edit-in-place consistency (Postgres + Key Value)

**Worker:** worker5 **Goal:** The Databases and Key Value detail pages edit their single-value fields with the same always-rendered disabled-input/select + pencil → Cancel/"Save changes" pattern the services Settings page adopted in w5/m50 — matching Render's cross-resource design system and retiring the older bespoke always-editable "Input + Save" rows. **Status:** todo

## Tasks (in order)

| id   | title                                                                                        | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Live walk: Render Postgres + Key Value detail vs bex; inventory inline-editable fields + drift | 40m | —          |
| t002 | Migrate Databases detail fields (Name, Version, single-value settings) to EditableFieldRow    | 45m | t001       |
| t003 | Migrate Key Value detail fields (Name, Maxmemory Policy) to EditableFieldRow (text/select)     | 30m | t001       |
| t004 | Reconcile section-order / grouping drift surfaced by the walk (align to Render, or file notes) | 30m | t002, t003 |
| t005 | Render parity — cross-surface consistency check                                              | 30m | t004       |
| t006 | Simplify — run `/simplify` over the changed code                                             | 20m | t005       |
| t007 | Test coverage — behavior tests for the migrated datastore fields                             | 30m | t005       |
| t008 | Closeout — verify DoD, mark done, move milestone                                             | 15m | t007       |

## Definition of done

On dev-5, the Postgres and Key Value detail pages render their editable single-value fields (at minimum: database Name + Version; key-value Name + Maxmemory Policy) as visibly disabled inputs/selects with a pencil that enables and focuses the same control and swaps the pencil for Cancel + "Save changes" (Save disabled until the value differs), matching w5/m50's `EditableFieldRow` pattern and Render's own datastore-page interaction. No editable single-value field on these two pages still renders as an always-editable `Input + Save` row (or a save-on-change select). Destructive/rebuild-affecting fields keep their confirm dialog. Existing mutations are reused (no new backend verbs). Dashboard test suite green.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-07-27 (topic "还有哪些 inconsistency 值得做的？"), extending the w5/m50–m54 services-Settings edit-in-place work to the next-biggest management surfaces. `database-name-section.tsx` and `key-value-name-section.tsx` were confirmed to use the pre-m50 always-editable `Input + Save` pattern, and `EditableFieldRow` is currently used only under `features/services/`.
- **Goal linkage:** Render-parity pillar (`docs/ADR018-render-parity.md`) — cross-resource interaction consistency; Render's dashboard uses one edit-in-place pattern across services and datastores.
- **Expected outcome:** editing a Postgres/Key Value field feels identical to editing a service and to Render — no jarring pattern switch between resource types.
- **Why now:** the shared `EditableFieldRow` (three variants: text/number, select, combobox) shipped and proved in production across m50–m54; reusing it now, while fresh, is cheap, and the inconsistency is visible on every datastore detail page.
- **Render parity task included:** yes — pure-UI change over already-shipped setter verbs (`setDatabaseName`, `updatePlan`/`setMaxmemoryPolicy`, …); the parity check compares each migrated field interaction-for-interaction against Render's Postgres/Key Value pages.
