# w1 · m161 — A WebSocket whose traffic is only client→server does not keep a free service awake

**Worker:** worker1 **Goal:** a free web service stays awake while any WebSocket on it is carrying traffic, in either direction, the way Render counts WebSocket messages as inbound activity. **Status:** todo (t001, t002, t005 and t006 done; t003 live and t004 parity wait on the Traefik roll)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The `websocketegress` plugin counts client→server frames as well as server→client — **DONE** | 45m | — |
| t002 | The operator's activity read sums both directions — **DONE** | 30m | t001 |
| t003 | Live: a client-only WebSocket holds a free service past its idle window | 45m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
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

**Simplify (t005).** One combined reuse/quality/efficiency pass over the shipped diff.

- **Applied:**
  - dropped the `c.ingress != nil` guard from `downstreamConn.Read`. It existed only so the pre-existing `TestDownstreamDoesNotCountClientReads` — which hand-builds a `downstreamConn` — would not panic; production always populates both counters, and no other field in that file is nil-checked. Every hand-built `downstreamConn` in the test file now supplies both, so the production path adds unconditionally like `Write` does;
  - cited `w1/m161` on the new `ingress` field comment, matching the file's citation-everywhere convention.
- **Declined:**
  - **Merging `counters` and `ingress` into one map of a two-counter struct.** It would remove a `LoadOrStore` and the allocation-order comment, but `metricsBody` still needs a separate pass per metric because Prometheus exposition requires each series to carry its own HELP/TYPE block — so the merge buys map bookkeeping only. Worth a follow-up, not worth churning a shipped plugin.
- **Confirmed clean:** the billing invariant (the egress counter path is untouched, and its test still exercises the unmodified write accounting); efficiency (the read path takes one atomic add and no mutex, where the write path already takes `c.mu`; the third `or` operand is a bounded index lookup, not a scan); and the plugin's stdlib-only constraint, which rules out a `CounterVec` here.
- **Known limit of the tests:** the claim that `or` over a series Prometheus does not have yet contributes nothing is an externally-trusted PromQL semantic — the activity tests drive a fake HTTP server with canned JSON, so they pin the query's _shape_, not Prometheus's evaluation of it.

**Suites.**

- The plugin's own `go test ./...` passes.
- **`make test`** (operator) and **`make lint`** — recorded at the ship.

**Rollout.** The plugin ships as a ConfigMap generated from its source files (`disableNameSuffixHash: true`) and is loaded through Traefik's `localPlugins`, so there is no version to bump: Argo applies the ConfigMap and Traefik pods must roll to pick it up. The operator change is inert until that happens.

## Live verification (2026-09-16, production)

**Fixture.** `qa-20260916-m161ws` (`srv-dakvq2557grc738qmmh0`), a free web service from `examples/hello-python` whose Docker Command runs a **read-only** stdlib WebSocket server: it completes the RFC6455 handshake and then only reads, never sending a frame. That is the mirror of `w1/m151`'s fixture, which ticked server→client — and it is what makes the test honest, because server→client frames are exactly what the old egress counter already saw. Its idle window was set to the smallest real preset, 300 s (`setIdleTimeout`), so the probe is short and spends less time exposed to stray traffic.

**Before the fix** (nothing had rolled yet — images still pinned at `eb035151a`, the `w1/m158` build):

```text
02:46:09Z  service_woken — the client's upgrade request woke the pod through the activator
           (a hibernated service answers the first attempt with 503 + Retry-After; the
           client retries until it gets 101, or the hold would measure nothing)
02:46:18Z  phase Running; the client holds one WebSocket and sends a frame every 10 s
02:51:10Z  phase Hibernated — 301 s after the wake, with client frames still flowing
02:51:43Z  the server closed the socket: its pod had been scaled to zero
02:53:55Z  final phase Hibernated | 34 client frames sent, 0 bytes received from the server
```

- **The filed symptom, reproduced.** A WebSocket carrying only client→server traffic does **not** keep a free service awake: it slept on schedule, exactly as if the connection were not there.
- **Not confounded.** The request log for the window holds 2 records, both the client's own upgrade attempts (`ClientHost 10.10.0.7`, the load-balancer hop). Stray traffic is the risk that confounded `w1/m151`'s probe, but it could only have kept the service _awake_ — and it slept, so this direction of the claim is safe from it.
- **The 300 s window is confirmed applied**, independently of REST projecting `idleTTLSeconds` as absent: each wake was followed by a hibernate exactly 300 s later (02:28:58 → 02:33:58, 02:46:09 → 02:51:10).

**After the fix.** Pending the roll that pins the plugin and the operator; the same probe re-runs unchanged.

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
