# w7 · m42 — Logs UI: URL-shareable filters + Render-simple filter bar

**Worker:** worker7 **Goal:** bex's Logs tab matches Render's two logs-UX strengths — (1) the full filter state lives in the URL query string so a filtered view is shareable, bookmarkable, and reload-stable (today only `range` is URL-backed; the other 8 filters are lost on reload), and (2) the filter bar reads as simple as Render's by keeping a minimal primary toolbar (type · search · range · live) and moving the structured filters (level/method/statusCode/instance/path) behind a progressive-disclosure "Filters" affordance without losing any capability. **Status:** done

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- |
| t001 | URL-back the full log-filter set (type/level/method/statusCode/instance/path/text/live) — **DONE** | 45m | —          |
| t002 | Render-simple filter bar: minimal primary toolbar + "Filters" popover + active chips — **DONE** | 45m | t001       |
| t003 | Render deep-link prefill: accept `t`/`r` query aliases, normalize to canonical keys — **DONE** | 20m | t001       |
| t004 | Render parity — verify Logs filter/URL behavior vs Render across surfaces — **DONE** | 20m | t002, t003 |
| t005 | Simplify — `/simplify` over the milestone's diff — **DONE** | 15m | t004       |
| t006 | Test coverage — meaningful tests for URL round-trip + filter-bar disclosure — **DONE** | 30m | t004       |
| t007 | Closeout — move to `done/` when the DoD holds — **DONE** | 15m | t006       |

## Definition of done

On the Logs tab (`/services/<name>/logs`):

1. **Every filter round-trips through the URL.** Setting type, level, method, statusCode, instance, path, text, or the live toggle updates the query string (e.g. `?range=1h&type=request&level=error&statusCode=5xx&path=/api&text=boom&live=0`); reloading the page restores the exact filter state; copying the URL to a new tab reproduces the same filtered view. Empty/default filters are omitted from the URL (no `?type=all` noise), byte-identical to today's clean URL when nothing is filtered.
2. **The primary toolbar is minimal** — log type + search + time range + live are always visible; the five structured filters (level, method, statusCode, instance, path) are collapsed behind a single **Filters** control (popover/sheet) carrying an active-filter count badge. Active structured filters render as removable chips in the bar so nothing is hidden when set.
3. **No capability is lost** — every filter still reaches the same GraphQL/SSE query it does today; the store-only-filter → live-tail-auto-disable behavior is preserved and its explanation still shows.
4. **A Render-shaped logs deep link prefills the view** — visiting the bex Logs route with Render's `?t=app&r=1h` keys resolves to type=app + range=1h (aliases normalized to bex's canonical keys), mirroring the w7/m39 Render-route-compat precedent.
5. Existing behavior unchanged where not in scope: range presets, debounced text/path, live/history merge+dedupe, empty/403/503 states.

## Source + Goal linkage

- **Source:** User request 2026-07-16 — live Playwright comparison of Render's logs (`.../logs?t=app&r=1h`) vs bex's (`/services/<name>/logs`). Render encodes filter state in the URL (`t`=type, `r`=range) and keeps a 3-control primary toolbar (type · search · range) with structured filters tucked away; bex URL-backs only `range` (verified live: selecting "Request logs" left the URL unchanged) and shows 9 always-visible controls in one wrapped row. Backend honors every filter already ([docs/ADR010-observability.md](../../../docs/ADR010-observability.md)); the `parseLogSearch`/`validateSearch` route seam already URL-backs `range` (`dashboard/src/features/logs/lib/log-search.ts`, `src/routes/services.$serviceId.logs.tsx`).
- **Goal linkage:** Render dashboard parity + AI-native operability ([docs/ADR008-vision.md](../../../docs/ADR008-vision.md) #2 "basic obs for operation") — a shareable, link-prefillable filtered log view is how an operator or agent hands off "here's the failing request stream" without re-driving the UI. ADR018 already marks logs query + structured filters ✅ on all surfaces; this milestone closes the UI-consistency gap (URL-shareability + default simplicity), not a missing capability.
- **Expected outcome:** A bex user can filter logs, copy the URL, and share/bookmark the exact view; the reload-safe URL survives refreshes; and the Logs tab reads as clean as Render's by default while keeping bex's richer filter set one click away.
- **Why now:** w7 is the polish workstream for already-shipped features (m34/m38/m41 precedent), and the logs filter surface is fully built on the backend — this is pure dashboard UX polish with no backend change, low-risk and self-contained. The `range`-only URL seam is a half-finished pattern; completing it now avoids the filter state silently resetting on every reload, the top friction point the live comparison surfaced.
- **Render parity:** included (t004) — UI-surface work touching the Logs tab; the check confirms the URL/filter model stays consistent with Render's `t`/`r` shape and that no REST/GraphQL/MCP drift is introduced (this is UI-only, so the closing check is expected to confirm the backend surface is untouched).
