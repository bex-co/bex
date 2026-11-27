# w1 · m685 — Moving resources into an environment is Render's Move Services modal

**Worker:** worker1 **Goal:** an environment's Move existing services opens Render's Move Services modal, and bex's Manage resources checklist is retired where Render's controls cover it. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Render's Move Services modal | 1h45m | w1/m682 t001 |
| [t002](t002.md) | Open it from the empty state and the New menu; move with the admin checks and refresh both sections | 1h | t001, w1/m682 t004, w1/m684 t001 |
| [t003](t003.md) | Retire Manage resources where each of its jobs has a Render home | 45m | t002 |
| [t004](t004.md) | Walk moving resources into an environment on dev-1 and production | 45m | t003 |
| [t005](t005.md) | Render parity | 20m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005 |
| [t008](t008.md) | Closeout | 10m | t006, t007 |

Total: **8 tasks, about 5h30m**.

## Definition of done

1. Move existing services, from an environment's empty state or its New menu, opens Render's Move Services modal with Render's words, grouping, search, tooltips and counted submit label.
2. Moving resources from several projects lands them in the target section and removes them from their source sections without a reload; a protected-environment refusal is shown per resource.
3. Manage resources is retired, or the README records which job keeps it and why; walked on production at 1440 and 390 px.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-26), filed to w1 by `/pm` the same day after re-reading Render's dashboard bundle. Render's public dashboard bundle, read 2026-11-26 without sign-in (entry `index-D5V4ZgbE.js`; chunk hashes change with every Render deploy, so find the code by the strings quoted here): the `Project` chunk's modal ("Move Services", "Move the following services into my", "Search services to move…", "Ungrouped services", "Move service", "Move N services").
- **bex today (checked 2026-11-26):** an environment's Manage resources is a full-replace checklist (`manage-resources-dialog.tsx`), which Render has no counterpart for.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** gathering existing resources into an environment works as on Render, from the places Render offers it.
- **Why now:** w1/m682's empty state and w1/m684's New menu both offer Move existing services; this gives them Render's destination.
- **Render parity task:** included (t005): dashboard UI over the existing membership operations.
