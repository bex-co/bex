# w1 · m161 — A WebSocket whose traffic is only client→server does not keep a free service awake

**Worker:** worker1 **Goal:** a free web service stays awake while any WebSocket on it is carrying traffic, in either direction, the way Render counts WebSocket messages as inbound activity. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The `websocketegress` plugin counts client→server frames as well as server→client | 45m | — |
| t002 | The operator's activity read sums both directions | 30m | t001 |
| t003 | Live: a client-only WebSocket holds a free service past its idle window | 45m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 40m | t004 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- **A client-only WebSocket keeps the service awake.** On a free web service whose idle window has elapsed, a WebSocket connection on which only the client sends frames (no server replies, no HTTP requests) leaves the service Running past the window, and `service_hibernated` is not recorded.
- **Control (must not regress).** A server→client WebSocket still keeps it awake (`w1/m151`), and a genuinely idle service still hibernates on schedule.
- **The counter is real.** The plugin exports a client→server byte counter per app, and the operator's activity query reads both counters.

## Root cause

- `deploy/gitops/charts/traefik-plugins/websocketegress/websocketegress.go` counts writes on the hijacked connection only — `bex_websocket_egress_bytes_total{app_id}` is server→client traffic.
- `lego/operator/internal/controller/activity.go` (`activityQueries`) reads that one counter, so `w1/m151`'s activity signal misses a connection the client alone is feeding.

## Blast radius

- **Who is hit.** Free web services whose WebSocket clients push without server replies — telemetry, log shippers, collaborative editors between server pushes.
- **Severity.** Minor: most WebSocket apps send server→client frames, and any HTTP request also counts. But when it bites, the service sleeps under real traffic.
- **Rollout risk.** The plugin ships through Argo with Traefik; a plugin version bump rolls the edge. The operator change is inert until the new counter exists, so the plugin lands first.

## Source + Goal linkage

- **Source:** `w1/102`, filed from the plugin source during `w1/m151` t002 (2026-09-14); never probed live. Promoted 2026-09-15 during the w1 triage, with the decision to fix rather than record it as a permanent divergence.
- **Goal linkage:** `docs/ADR018-render-parity.md` — Render counts "WebSocket messages from existing connections" as traffic that prevents a free service from spinning down; `w1/done/m151` owns bex's activity signal.
- **Expected outcome:** the idle clock reflects all WebSocket traffic, not half of it.
- **Why now:** it is the last open behavior note in w1's inbox, and it completes the activity signal `w1/m151` shipped.
- **Render parity is included** because the sleep behavior is visible through the service phase and events on every read surface.
