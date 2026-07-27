# w5 · m55 — Datastore detail-page edit-in-place consistency (Postgres + Key Value)

**Worker:** worker5 **Goal:** The Databases and Key Value detail pages edit their single-value fields with the same always-rendered disabled-input/select + pencil → Cancel/"Save changes" pattern the services Settings page adopted in w5/m50 — matching Render's cross-resource design system and retiring the older bespoke always-editable "Input + Save" rows. **Status:** done

## Tasks (in order)

| id   | title                                                                                        | est | depends_on | status       |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------- | ------------ |
| t001 | Live walk: Render Postgres + Key Value detail vs bex; inventory inline-editable fields + drift | 40m | —          | — **DONE**   |
| t002 | Migrate Databases detail fields (Name, Version, single-value settings) to EditableFieldRow    | 45m | t001       | — **DONE**   |
| t003 | Migrate Key Value detail fields (Name, Maxmemory Policy) to EditableFieldRow (text/select)     | 30m | t001       | — **DONE**   |
| t004 | Reconcile section-order / grouping drift surfaced by the walk (align to Render, or file notes) | 30m | t002, t003 | — **DONE**   |
| t005 | Render parity — cross-surface consistency check                                              | 30m | t004       | — **DONE**   |
| t006 | Simplify — run `/simplify` over the changed code                                             | 20m | t005       | — **DONE**   |
| t007 | Test coverage — behavior tests for the migrated datastore fields                             | 30m | t005       | — **DONE**   |
| t008 | Closeout — verify DoD, mark done, move milestone                                             | 15m | t007       | — **DONE**   |

## Walk findings (t001, live Render walk 2026-07-27)

Walked Render's managed Postgres detail (`/d/dpg-…`, PostgreSQL 16) authenticated. Render's **Info → "General"** section groups the fields; the interaction is the design system m50 already adopted:

- **Name** — a visibly disabled input + a pencil button. Clicking the pencil makes the input `active` and swaps the pencil for **Cancel** + **"Save changes"** (Save disabled until the value differs). This is byte-for-byte `EditableFieldRow`. bex's `DatabaseNameSection`/`KeyValueNameSection` diverged (always-editable `Input + Save`) → migrated.
- **PostgreSQL Version** — a **read-only value + a link to a dedicated `/version-upgrade` page**, _not_ an inline editor. bex's `DatabaseVersionControl` already matches this (read-only value + "Upgrade" button opening a confirm dialog) → **no migration needed**; the milestone's original "migrate Version" assumption was corrected by the walk.
- Render places **Name as the editable lead row of the General group**, above the read-only facts (Created/Status/Version/Region/Storage). bex kept Name in a _separate_ card above the read-only `MetadataList` → reconciled (t004): Name now leads the Details card via a new `MetadataList` `lead` slot, matching Render's grouping.
- **Key Value** — no live KV instance was reachable in the walk account; Render's design system renders the KV Info page from the same components (confirmed identical on Postgres), so the KV Name + Maxmemory Policy migration follows the confirmed pattern.

## Definition of done

On dev-5, the Postgres and Key Value detail pages render their editable single-value fields (at minimum: database Name + Version; key-value Name + Maxmemory Policy) as visibly disabled inputs/selects with a pencil that enables and focuses the same control and swaps the pencil for Cancel + "Save changes" (Save disabled until the value differs), matching w5/m50's `EditableFieldRow` pattern and Render's own datastore-page interaction. No editable single-value field on these two pages still renders as an always-editable `Input + Save` row (or a save-on-change select). Destructive/rebuild-affecting fields keep their confirm dialog. Existing mutations are reused (no new backend verbs). Dashboard test suite green.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-07-27 (topic "还有哪些 inconsistency 值得做的？"), extending the w5/m50–m54 services-Settings edit-in-place work to the next-biggest management surfaces. `database-name-section.tsx` and `key-value-name-section.tsx` were confirmed to use the pre-m50 always-editable `Input + Save` pattern, and `EditableFieldRow` is currently used only under `features/services/`.
- **Goal linkage:** Render-parity pillar (`docs/ADR018-render-parity.md`) — cross-resource interaction consistency; Render's dashboard uses one edit-in-place pattern across services and datastores.
- **Expected outcome:** editing a Postgres/Key Value field feels identical to editing a service and to Render — no jarring pattern switch between resource types.
- **Why now:** the shared `EditableFieldRow` (three variants: text/number, select, combobox) shipped and proved in production across m50–m54; reusing it now, while fresh, is cheap, and the inconsistency is visible on every datastore detail page.
- **Render parity task included:** yes — pure-UI change over already-shipped setter verbs (`setDatabaseName`, `updatePlan`/`setMaxmemoryPolicy`, …); the parity check compares each migrated field interaction-for-interaction against Render's Postgres/Key Value pages.
