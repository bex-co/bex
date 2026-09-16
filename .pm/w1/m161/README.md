# w1 · m161 — A WebSocket whose traffic is only client→server does not keep a free service awake

**Worker:** worker1 **Goal:** a free web service stays awake while any WebSocket on it is carrying traffic, in either direction, the way Render counts WebSocket messages as inbound activity. **Status:** todo (t001, t002 and t006 done; t003 live, t004 parity, t005 simplify wait on the Traefik roll)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The `websocketegress` plugin counts client→server frames as well as server→client — **DONE** | 45m | — |
| t002 | The operator's activity read sums both directions — **DONE** | 30m | t001 |
| t003 | Live: a client-only WebSocket holds a free service past its idle window | 45m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage — **DONE** | 40m | t004 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- **A client-only WebSocket keeps the service awake.** On a free web service whose idle window has elapsed, a WebSocket connection on which only the client sends frames (no server replies, no HTTP requests) leaves the service Running past the window, and `service_hibernated` is not recorded.
- **Control (must not regress).** A server→client WebSocket still keeps it awake (`w1/m151`), and a genuinely idle service still hibernates on schedule.
- **The counter is real.** The plugin exports a client→server byte counter per app, and the operator's activity query reads both counters.

## Root cause

- `deploy/gitops/charts/traefik-plugins/websocketegress/websocketegress.go` counts writes on the hijacked connection only — `bex_websocket_egress_bytes_total{app_id}` is server→client traffic.
- `lego/operator/internal/controller/activity.go` (`activityQueries`) reads that one counter, so `w1/m151`'s activity signal misses a connection the client alone is feeding.

## Implementation (2026-09-15)

**The plugin counts both directions (t001).** `deploy/gitops/charts/traefik-plugins/websocketegress/websocketegress.go`:

- a second per-App counter map (`processState.ingress`), allocated only past the existing App-count cap so one cap still bounds both;
- `downstreamConn.Read` adds what the client sends to that counter. No handshake accounting is needed on this side — net/http consumes the upgrade request before `Hijack`, so everything read afterwards is frame bytes;
- `metricsBody` exports `bex_websocket_ingress_bytes_total{app_id}` beside the egress series.

**The egress counter is untouched on purpose.** It is the billable meter (`lego/backend/internal/egressquery/query.go` reads it as `Source: WebSocket`), and it still ignores client reads — the existing `TestDownstreamDoesNotCountClientReads` still passes unchanged. Ingress is a separate series that billing never reads.

**The operator treats either direction as activity (t002).** `activityQuery` (`lego/operator/internal/controller/activity.go`) adds a third `or` clause for the ingress series. PromQL `or` over a series Prometheus does not have yet contributes nothing, so an operator running ahead of the plugin roll behaves exactly as it does today — which is what makes the rollout order safe in either direction.

**Tests (t006).**

| Test | Pins | Fails under |
| --- | --- | --- |
| `TestDownstreamCountsClientReadsAsIngressOnly` (plugin) | Client bytes land in the ingress counter, the billing egress counter stays 0, and the bytes are forwarded unchanged | A plugin that ignores reads (today's behavior), or one that bills them as egress |
| `TestMetricsExposeBothDirections` (plugin) | The metrics body declares both counters | The ingress series missing from the exposition |
| `TestDownstreamDoesNotCountClientReads` (plugin, pre-existing) | The egress meter still ignores reads | Any change that bills client traffic |
| `TestPrometheusAppActivityReaderQueriesEverySignal` | One round trip whose query carries the Traefik request counter and both WebSocket series | An activity read that asks only about egress |

**Suites.**

- The plugin's own `go test ./...` passes.
- **`make test`** (operator) and **`make lint`** — recorded at the ship.

**Rollout.** The plugin ships as a ConfigMap generated from its source files (`disableNameSuffixHash: true`) and is loaded through Traefik's `localPlugins`, so there is no version to bump: Argo applies the ConfigMap and Traefik pods must roll to pick it up. The operator change is inert until that happens.

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
