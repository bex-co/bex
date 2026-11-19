# w3 · m507 — Project views can be created, edited, duplicated and deleted, and are shared by the whole project, as on Render

**Worker:** worker3 **Goal:** a project's members manage its observability views as Render's do: New View by type, Edit view (name and filter rows), Duplicate and Delete, every change visible to everyone on the project, recorded in the audit log under Render's event names. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | GraphQL create, update, duplicate and delete for saved searches, audited under Render's event names | 1h30m | w3/m505/t003 |
| [t002](t002.md) | New View: pick a type, name it, save it | 1h30m | t001, w3/m505/t006 |
| [t003](t003.md) | The Edit view modal: name and filter rows | 1h30m | t002 |
| [t004](t004.md) | Duplicate and Delete, defaults included | 45m | t003 |
| [t005](t005.md) | Role-aware controls | 30m | t004 |
| [t006](t006.md) | Verify on dev-3 and on production | 30m | t005 |
| [t007](t007.md) | ADR032 and ADR018 | 20m | t006 |
| [t008](t008.md) | Render parity | 20m | t007 |
| [t009](t009.md) | Simplify | 15m | t008 |
| [t010](t010.md) | Test coverage | 30m | t008 |
| [t011](t011.md) | Closeout | 10m | t009, t010 |

## Definition of done

- A project editor creates a view by type, edits its name and filters, duplicates it and deletes it (default views included); every change is what every other member of the project sees.
- A viewer sees the controls disabled with a reason, and the API refuses them.
- A duplicate name answers `duplicate_saved_search_name`; the audit log records `CreateSavedSearchEvent` and `DeleteSavedSearchEvent`.
- Verified with two members on dev-3 and on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w3 2026-11-18/19 (topic: one aspect of Render parity, project and environment observability and the log explorer), filed by `/pmx them all to w3` 2026-11-19 after a read-only capture of Render's live project Observability views in the user's Render workspace (`docs/render-artifacts/project-observability.md`).
- **Goal linkage:** ADR008 § Render parity first. [render.com/docs/projects](https://render.com/docs/projects) § Observability and the capture: views are named, saved filter sets shared by the project ("Editing filters will update the view for everyone on the project.").
- **Recommendation, not a user decision:** write access follows the project's edit permission, and New View creates on Done; the capture could not show either, so ADR032 records them as bex's choices to revisit if a capture shows otherwise.
- **Expected outcome:** teams keep the filtered views they use (one service's errors, one environment's requests) instead of re-filtering every visit.
- **Why now:** w3/m505 ships the store and the defaults; without this the defaults are fixed and the sidebar's New View has nothing behind it.
- **Render parity included:** new GraphQL mutations, audit events and dashboard controls.
