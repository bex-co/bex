# w3 · m5 — Durable logs: Loki-backed history behind the same API

**Worker:** worker3 **Goal:** Logs survive pod restarts. Today the logs feature reads live pod logs only (kubelet ring buffer) — a crash-looping app loses its history at the exact moment someone investigates. Deploy Loki (buy, not build — same pattern as Prometheus), ship pod logs into it, and make `QueryLogs` read Loki when `BEX_LOKI_URL` is set, with byte-identical fallback when unset. Surface shapes (REST/GraphQL/MCP/UI) do not change. **Status:** implemented — code/gitops/docs/tests shipped and verified hermetically (fake-Loki restart-survival, fallback equivalence, real bounds, Loki-down); the live run of `scripts/logs-verify.sh` (t005) + the `done/` move (t009) remain. _2026-07-11: the "Loki-synced cluster" the gate waited on exists — it's prod (Loki + log-shipper live on the platform pool, `BEX_LOKI_URL` wired in `lego/operator/config/api/deployment.yaml`); the CAPD mock can never be it (`scripts/mock-cluster.sh` installs no Argo CD/GitOps). t005 retargeted to prod via `scripts/fetch-app-kubeconfig.sh`; t005 + t009 are unblocked today._

## Tasks (in order)

| id   | title                                                                                             | est | depends_on | status |
| ---- | -------------------------------------------------------------------------------------------------- | --- | ---------- | ------ |
| t001 | GitOps: Loki (single-binary, filesystem PVC) + log-shipper DaemonSet (Alloy/Promtail), k8s labels  | 30m | —          | — **DONE** (`deploy/gitops/base/loki.yaml` + `log-shipper.yaml`, Alloy chosen; both overlays render) |
| t002 | bex-api: `BEX_LOKI_URL` client — `QueryLogs` via LogQL from the existing `LogQuery` filters        | 30m | t001       | — **DONE** (`internal/logs/loki.go` + `LogHistorySource` seam; wired in `main.go`/`server.go`) |
| t003 | Live tail decision + wiring: SSE stays on pod logs vs Loki tail — pick, implement, document        | 25m | t002       | — **DONE** (tail stays on pod logs; rationale in docs/ADR010-observability.md) |
| t004 | Retention (match Render's window) + env-template name mirrors + docs/ADR010-observability.md caveat swap  | 20m | t002       | — **DONE** (7d = Render Hobby window, tiered note; no `.env` var — non-secret URL like `BEX_PROM_URL`) |
| t005 | Acceptance: pod restart → pre-restart lines still served over REST and MCP `list_logs`             | 25m | t003, t004 | — **PARTIAL** (`scripts/logs-verify.sh` written + hermetic restart-survival/fallback tests pass; live run retargeted to prod via `scripts/fetch-app-kubeconfig.sh` — unblocked 2026-07-11) |
| t006 | Render parity — same log shapes/filters across REST/GraphQL/MCP/UI; compare Render's logs behavior | 20m | t005       | — **DONE** (no drift — pure `QueryLogs` backend swap; parity ledger row refreshed) |
| t007 | Simplify — `/simplify` over the code this milestone changed                                        | 20m | t006       | — **DONE** (diff reviewed; reuse via `lokiLimit`/`labelOr`; gofmt/lint clean) |
| t008 | Test coverage — meaningful tests for the Loki path + fallback                                      | 30m | t006       | — **DONE** (`internal/logs/loki_test.go`: LogQL builder, injection escaping, parser, bounds, Loki-down, routing) |
| t009 | Closeout — DoD met → move milestone to `done/`                                                     | 10m | t008       | — **PENDING** (blocked on t005 live run per its own gate) |

## Definition of done

Restarting an app's pod does not lose its logs: a query afterwards returns pre-restart lines, bounded by a real time range, through all existing surfaces with unchanged shapes; retention covers at least Render's searchable window; with `BEX_LOKI_URL` unset, behavior is byte-identical to today's pod-log path (omitted, not faked).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-07-09; w3/001's explicit v0 deferral ("Defer a durable log/metric store … to a later milestone"); `lego/backend/internal/logs/service.go` (pod-log source, Render-shaped `LogQuery`); Render keeps ~7 days of searchable logs.
- **Goal linkage:** GOAL.md #2 ("Basic obs for operation"); Render logs parity (pillar 1).
- **Expected outcome:** crash investigations stop racing log rotation; time-range queries become real instead of best-effort over a live stream.
- **Why now:** the deferral was explicit and its precondition is met — the logs API/UI shapes are shipped and stable (w3/m1, w5/m6), so this is a pure backend swap with no surface churn.
- **Render parity closing task: included** — the logs surface spans REST/GraphQL/MCP/UI.
