# w3 · m509 — Every log explorer reads and drives like Render's: level gutter, copy, order, expand, fullscreen and keyboard shortcuts

**Worker:** worker3 **Goal:** bex's shared log explorer (service, datastore, deploy and project views) gains what Render's has and bex lacks: the level gutter with tinted warning and error rows, full-timestamp hover, date separators, the Options menu (Copy logs, Order by, Expand everything), Maximize and Render's keyboard shortcuts. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Inventory: what each bex explorer already has against Render's list | 30m | — |
| [t002](t002.md) | Level gutter and tinted rows; request status chips | 1h | t001 |
| [t003](t003.md) | Full-timestamp hover and date separators | 45m | t001 |
| [t004](t004.md) | The Options menu: Copy logs, Order by, Expand everything | 1h | t001 |
| [t005](t005.md) | Maximize and the keyboard shortcuts | 1h | t004 |
| [t006](t006.md) | Verify at 1440 and 390, and on production | 30m | t002, t003, t005 |
| [t007](t007.md) | Render parity | 20m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 30m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

## Definition of done

- Every bex log explorer shows the level gutter with tinted warning and error rows, request status chips, full-timestamp hover and date separators.
- Options offers Copy logs (`⌘/Ctrl ⇧ C`), Order by and Expand everything; Maximize and the shortcuts `/`, `M`, `Home`/`End`, `Shift+L` work and never fire while typing.
- Verified at 1440 and 390 on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w3 2026-11-18/19 (topic: one aspect of Render parity, project and environment observability and the log explorer), filed by `/pmx them all to w3` 2026-11-19 after a read-only capture of Render's live project Observability views in the user's Render workspace (`docs/render-artifacts/project-observability.md`).
- **Goal linkage:** ADR008 § Render parity first: [render.com/docs/logging](https://render.com/docs/logging) documents the line metadata and shortcuts, and the capture shows the Options menu, Maximize and the gutter.
- **Expected outcome:** reading logs on bex works the way Render users expect: levels visible at a glance, copy and fullscreen at hand.
- **Why now:** w3/m505 adds the project views on the same shared components; adding these to the shared line list and toolbar now gives every explorer them once instead of retrofitting four.
- **Render parity included:** dashboard-only change; the parity task compares each explorer with Render's.
