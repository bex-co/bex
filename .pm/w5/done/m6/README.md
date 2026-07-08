# w5 · m6 — Live logs page (Render-consistent: historical query + SSE live-tail)

**Worker:** worker5 **Goal:** The service page's Logs tab shows an operator's App logs the way Render's Logs viewer does — a historical `logs(...)` query plus a live tail — sourced from bex-api's Render-shaped logs API, with the one deliberate divergence (bex live-tails over SSE where Render uses WebSocket) documented in the UI's data layer. **Status:** done (2026-07-08)

## Tasks (in order)

| id   | title                                                                                             | est | depends_on         |
| ---- | ------------------------------------------------------------------------------------------------- | --- | ------------------ |
| t001 | Capture Render's Logs page via Playwright (filter bar, live toggle, log-line layout) as the design source — **DONE** | 30m | —                  |
| t002 | `logs.graphql` query + codegen; historical log panel (message/timestamp/instance, text filter) in Render's `LogEntry` shape — **DONE** | 50m | w5/m6/t001         |
| t003 | Live-tail via `EventSource` against `/v1/logs/subscribe` (SSE, Kratos session): pause/resume, autoscroll, capped buffer; document the SSE-vs-WebSocket divergence — **DONE** | 75m | w5/m6/t002         |
| t004 | Wire as the Logs tab; empty / error / disconnected states; verify live against a running App; screenshot — **DONE** | 40m | w5/m6/t003         |
| t005 | Simplify — `/simplify` over the code this milestone changed — **DONE**                              | 30m | w5/m6/t004         |
| t006 | Test coverage — meaningful tests for log mapping + live-tail buffer/reconnect behavior — **DONE**   | 30m | w5/m6/t004         |

## Definition of done

- The service page's **Logs tab** shows historical App logs for the service from bex-api's `logs(resource, type, text, limit)` query, rendered in Render's `LogEntry` shape (`timestamp`, `message`, `type`, `instance`); a text filter narrows the results.
- With live-tail enabled, new lines append in real time from `GET /v1/logs/subscribe` (SSE) authenticated by the Kratos session; the viewer supports pause/resume, autoscroll, and a capped in-memory buffer.
- The layout matches the Render Logs reference captured in t001 (filter bar + live toggle + line list); the SSE-vs-WebSocket divergence (bex's deliberate choice, `docs/bex-api.md`) is documented in the data-layer code, not hidden.
- Disconnect/reconnect, empty (no logs), and error states are handled explicitly.
- Filters Render exposes that bex-api can't honor over raw pod logs (`level`, `statusCode`, `method`, and `type=request`/`build` → empty) are omitted or shown empty per bex-api's contract, not faked.
- `yarn lint && yarn typecheck && yarn test && yarn build` all pass; a live-tail screenshot is captured to `.playwright-mcp/`.

## Completion notes (2026-07-08)

- Shipped `dashboard/src/features/logs/**` (self-contained feature): `api/logs.graphql` + the hand-added `Logs` operation in `graphql/definitions.ts`; `lib/{map,format}.ts` (both wire shapes → one flat `LogLine`, dedupe key, `mergeLogLines`); `hooks/use-log-history.ts` (Apollo) + `hooks/use-live-logs.ts` (SSE `EventSource`, capped/deduped ring buffer, render-phase reset, documented SSE-vs-WebSocket divergence); `components/{log-viewer,log-filter-bar,log-line-list}.tsx`; `locales/{en,zh}.ts`. Wired into the Logs tab route; `logs` i18n namespace registered; `services.logsComingSoon*` placeholder strings removed.
- The SSE REST base lives in `config.ts` (`apiBaseUrl` = `apiUrl` minus `/graphql`), not derived inside the feature (t005 altitude fix).
- Tests (t006): `lib/__tests__/map.test.ts` (mapping, dedupe, merge, format) + `hooks/__tests__/use-live-logs.test.ts` (append, dedupe, buffer cap, transport-drop vs terminal-error, filter-change reset, unmount) — 20 new tests; suite 373 green. `yarn lint && yarn typecheck && yarn test && yarn build` all pass.
- **Live verification / local dev:** prod `api.bex.co` can't be driven from a localhost dashboard (CORS rejects the origin; the Kratos session cookie is `*.bex.co`-scoped and isn't sent cross-site → 401). Added `dashboard/scripts/local-bex.mjs` (`yarn local-bex` / `yarn dev:local`) — a zero-dep, no-auth, open-CORS dev stub speaking bex-api's logs wire protocol (GraphQL reads + `/v1/logs/subscribe` SSE) + Kratos `whoami`, streaming synthetic app logs. Verified end-to-end against it: historical page + live tail, text-filter narrowing (history **and** live), autoscroll, "Live — streaming" status. Screenshot: `.playwright-mcp/dashboard-logs-live.png`. Full-fidelity local bex still needs the mock cluster + Ory stack.

## Design source (t001) — Render Logs viewer

Captured from Render's dashboard (`.playwright-mcp/render-logs.png`, the `backend-v2`
service Logs tab). Information architecture, top → bottom:

- **Filter bar** (one horizontal row under the service header):
  - **Type dropdown** (left): `All logs` / `Application logs` (selected) / `Request logs`
    — a single-select radio menu. Maps to bex-api's `type` arg (`app`/`request`/`build`).
  - **Search box** (center, wide): "Search logs" with a magnifier icon → the `text` arg.
  - **Time-range dropdown** (right): "Last hour" with a clock icon.
  - **Fullscreen** expand icon + a **"…" overflow** menu.
- **Line list** (monospace, dense, newest at the bottom, autoscrolls):
  - Each line: `03:36:01 AM` timestamp (muted) · `[bv612]` instance in brackets (muted)
    · the raw message (here JSON stdout). Long lines wrap, indented under the message
    column. No level/status coloring — the message is rendered verbatim.
  - Render live-tails automatically when viewing recent logs; bex exposes this as an
    explicit **live toggle** (pause/resume) since the SSE stream is opened on demand.

### Filters bex-api honors vs. omits (over raw pod logs)

| Render control | bex-api GraphQL `logs(resource, type, text, limit)` | Decision |
| --- | --- | --- |
| Type: Application / Request / All | `type` = `app` / `request` / `build`; only `app` has a backend (`request`/`build` → empty) | **Honored** — offer All + Application; Request/Build resolve empty per contract, not faked |
| Search logs | `text` (case-insensitive substring) | **Honored** |
| (internal) result cap | `limit` (default 20, max 100) | Honored, not a user control |
| Time range ("Last hour", …) | GraphQL `logs()` takes no `start`/`end` (REST does; GraphQL does not) | **Omitted** — no time-range dropdown on the GraphQL panel |
| Level (info/warn/error) | not parsed from raw stdout | **Omitted** |
| Status code / Method / Path / Host | request-log filters, no backend | **Omitted** |
| Instance filter, direction | not wired (`docs/observability.md`) | **Omitted** |

The one deliberate transport divergence: bex live-tails over **SSE** (`GET /v1/logs/subscribe`)
where Render upgrades to a **WebSocket** — same "stream new lines live" contract, no extra
dependency (`docs/bex-api.md`, `docs/observability.md`). Documented in the data-layer hook.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5 to work on dashboard` (2026-07-06) + user directive "all apis and uis should be consistent with render.com". Backend live per `docs/observability.md` / `docs/bex-api.md`: GraphQL `logs(...)` + REST `GET /v1/logs` and SSE `GET /v1/logs/subscribe` (Render logs-API compatible; bex sources application logs only).
- **Goal linkage:** `docs/vision.md` dashboard pillar + `GOAL.md` #2 (obs) — logs are the other half of Render-parity observability alongside metrics (shipped w3/m3); pillar-1 API-first (logs already exposed via REST/GraphQL/MCP).
- **Expected outcome:** operators tail their App's logs in the dashboard, Render-style, without `kubectl` — completing the metrics + logs obs pair.
- **Why now:** the tab shell lands in m5, so logs slots in as a tab rather than a bare route; SSE live-tail is the one genuinely new client capability (a stream, not another Apollo query), worth isolating in its own milestone so its integration risk doesn't leak into the simpler query pages.
