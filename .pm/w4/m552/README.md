# w4 · m552 — Clicking a log line opens its detail view in every log explorer, as on Render

**Worker:** worker4 **Goal:** in every bex log explorer, clicking a log line (or opening it from the keyboard) shows Render's detail view of that record: its message, its timestamps and every label it carries, with Render's actions, without breaking text selection or the w3/m509 shortcuts. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's log line detail view from its public dashboard bundle | 45m | — |
| [t002](t002.md) | Every explorer's log query selects the fields the detail view shows | 45m | t001 |
| [t003](t003.md) | The detail view in the shared log line list | 2h | t002 |
| [t004](t004.md) | Keyboard: open and close the detail view without breaking m509's shortcuts | 45m | t003 |
| [t005](t005.md) | Visual check on dev-4: widths, dark mode and zh | 30m | t004 |
| [t006](t006.md) | Render parity | 20m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 45m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 6h15m**.

## Definition of done

On dev-4, then on production once the pin carries the change:

- Clicking a request line opens the detail view with every field t001 records, in that order and with those labels. This holds in a service's Logs tab, a Postgres and a Key Value Logs tab, a deploy's log, and a project's Application Logs and Request Logs views.
- Dragging to select text in a line opens nothing; Esc closes the view and returns focus to the line; every w3/m509 shortcut still works.
- The view fits at 390 with no clipped field, in light, dark and high-contrast themes and in zh.
- ADR018's "Log explorer display and shortcuts" row names the detail view.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one more aspect of render parity for w4`, 2026-11-20, item 3 (dashboard-only Render features no API pin sees). Filed by `/pmx them to w4`.
- **Render:** changelog 2025-12-17, "Improved logs UX in the Render dashboard and CLI": "Click individual log lines to open a detail view."
- **bex today:** a line's only affordance is its row menu (`dashboard/src/features/logs/components/log-row-menu.tsx`: Copy log line, View in context, SSH into instance). w3/m509 and w3/m528 matched Render's log explorer from its bundle, and neither records a detail view, so this is not a duplicate.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)); [ADR018](../../../docs/ADR018-render-parity.md) "Log explorer display and shortcuts" (UI ✅; this completes it).
- **Expected outcome:** reading one request's instance, status, latency or client fields takes a click instead of copying the line and parsing it.
- **Why now:** w3/m528's bundle method and the shared line list (w3/m509) just landed, so the view is built once for every explorer at the lowest cost.
- **Render parity:** included (t006): a dashboard surface; Render's API and MCP have no log explorer.
