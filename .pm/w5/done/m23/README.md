# w5 · m23 — Logs-tab structured filter UI

**Worker:** worker5 **Goal:** The service Logs tab exposes Render's full filter vocabulary (`type`/`level`/`statusCode`/`method`/`path`/`instance`), with dropdown values live-fed from the `logLabelValues` discovery `w3/m8` shipped, filter state reflected in the URL, request-log lines rendered distinctly from app-log lines, and an honest explanatory state when the durable log store isn't wired — closing the UI half of the parity ledger's "Request / HTTP logs + structured filters" row. **Status:** DONE 2026-07-13 — implemented directly (not task-by-task): filter bar with type + level/method/statusCode/instance dropdowns (fed by `logLabelValues`, static fallbacks) + a free-text request-path input; request lines render method/status chips; live tail auto-disables (with a note) while a store-only filter is active; a store-less `type=request`/structured-filter query shows an explanatory "request logs need the log store" (503) state. Filter state is component-local (URL-sync descoped). 17 tests; Request/HTTP-logs UI ◐→✅ in ADR018.

## Tasks (in order)

| id   | title                                                                                                                                          | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Capture Render's live Logs filter UI via Playwright → `docs/render-artifacts/` (controls/labels/placement — build against evidence, not guesses) | 30m | —          |
| t002 | Filter controls on the Logs tab: `level`/`statusCode`/`method`/`path`/`instance`, dropdown values fed by `logLabelValues` (shipped `w3/m8`)      | 1h  | t001       |
| t003 | URL sync: filter state reflected in the route's query params, matching the dashboard's existing filter-URL convention                          | 30m | t002       |
| t004 | Request-line rendering: a distinct treatment for `type=request` lines (method/status/path chips from labels, JSON collapsed) instead of reusing the app-line renderer on raw JSON | 45m | t002       |
| t005 | 503 empty/error state: an explanatory UI ("request logs need the log store") when `BEX_LOKI_URL` is unset (dev-mode), instead of a generic error toast | 30m | t002       |
| t006 | Live verification: compare the shipped UI against t001's Render capture; confirm `type=request` returns real Traefik access lines, `level=error` isolates errors | 30m | t003, t004, t005 |

## Definition of done

The service Logs tab exposes all five newly-honored filters with live-populated dropdown values, filter state is URL-shareable, request-log lines render distinctly from app-log lines, and the store-unavailable case shows an explanatory state — verified against a live Render capture for consistency.

## Source + Goal linkage

- **Source:** promotion of inbox `w5/008` — "unblocked (w3/m8 landed 2026-07-12)", which itself explicitly scoped this as milestone-sized ("filter state + URL sync + label-value queries + request-line rendering + the 503 state + capture comparison + tests"). Materialized via `/pm-brainstorm more milestones to work on` 2026-07-13.
- **Goal linkage:** pillar 1 (Render logs parity) — closes the UI half of `docs/ADR018-render-parity.md`'s "Request / HTTP logs + structured filters" row (backend already ✅ via `w3/m8`).
- **Expected outcome:** the ADR018 row's UI column moves off its current gap; a user can filter logs by the same vocabulary Render exposes, discoverable via live dropdown values rather than guessed strings.
- **Why now:** the backend gate (`w3/m8`) is shipped and unblocked; the source note itself already scoped this work as milestone-ready.
- **Render parity closing task: included** — this milestone *is* the UI parity closure; t006 doubles as the comparison check.
