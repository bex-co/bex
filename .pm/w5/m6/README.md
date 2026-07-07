# w5 · m6 — Live logs page (Render-consistent: historical query + SSE live-tail)

**Worker:** worker5 **Goal:** The service page's Logs tab shows an operator's App logs the way Render's Logs viewer does — a historical `logs(...)` query plus a live tail — sourced from bex-api's Render-shaped logs API, with the one deliberate divergence (bex live-tails over SSE where Render uses WebSocket) documented in the UI's data layer. **Status:** todo

## Tasks (in order)

| id   | title                                                                                             | est | depends_on         |
| ---- | ------------------------------------------------------------------------------------------------- | --- | ------------------ |
| t001 | Capture Render's Logs page via Playwright (filter bar, live toggle, log-line layout) as the design source | 30m | —                  |
| t002 | `logs.graphql` query + codegen; historical log panel (message/timestamp/instance, text filter) in Render's `LogEntry` shape | 50m | w5/m6/t001         |
| t003 | Live-tail via `EventSource` against `/v1/logs/subscribe` (SSE, Kratos session): pause/resume, autoscroll, capped buffer; document the SSE-vs-WebSocket divergence | 75m | w5/m6/t002         |
| t004 | Wire as the Logs tab; empty / error / disconnected states; verify live against a running App; screenshot | 40m | w5/m6/t003         |
| t005 | Simplify — `/simplify` over the code this milestone changed                                         | 30m | w5/m6/t004         |
| t006 | Test coverage — meaningful tests for log mapping + live-tail buffer/reconnect behavior              | 30m | w5/m6/t004         |

## Definition of done

- The service page's **Logs tab** shows historical App logs for the service from bex-api's `logs(resource, type, text, limit)` query, rendered in Render's `LogEntry` shape (`timestamp`, `message`, `type`, `instance`); a text filter narrows the results.
- With live-tail enabled, new lines append in real time from `GET /v1/logs/subscribe` (SSE) authenticated by the Kratos session; the viewer supports pause/resume, autoscroll, and a capped in-memory buffer.
- The layout matches the Render Logs reference captured in t001 (filter bar + live toggle + line list); the SSE-vs-WebSocket divergence (bex's deliberate choice, `docs/bex-api.md`) is documented in the data-layer code, not hidden.
- Disconnect/reconnect, empty (no logs), and error states are handled explicitly.
- Filters Render exposes that bex-api can't honor over raw pod logs (`level`, `statusCode`, `method`, and `type=request`/`build` → empty) are omitted or shown empty per bex-api's contract, not faked.
- `yarn lint && yarn typecheck && yarn test && yarn build` all pass; a live-tail screenshot is captured to `.playwright-mcp/`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5 to work on dashboard` (2026-07-06) + user directive "all apis and uis should be consistent with render.com". Backend live per `docs/observability.md` / `docs/bex-api.md`: GraphQL `logs(...)` + REST `GET /v1/logs` and SSE `GET /v1/logs/subscribe` (Render logs-API compatible; bex sources application logs only).
- **Goal linkage:** `docs/vision.md` dashboard pillar + `GOAL.md` #2 (obs) — logs are the other half of Render-parity observability alongside metrics (shipped w3/m3); pillar-1 API-first (logs already exposed via REST/GraphQL/MCP).
- **Expected outcome:** operators tail their App's logs in the dashboard, Render-style, without `kubectl` — completing the metrics + logs obs pair.
- **Why now:** the tab shell lands in m5, so logs slots in as a tab rather than a bare route; SSE live-tail is the one genuinely new client capability (a stream, not another Apollo query), worth isolating in its own milestone so its integration risk doesn't leak into the simpler query pages.
