# w3 · m5 — Durable logs: Loki-backed history behind the same API

**Worker:** worker3 **Goal:** Logs survive pod restarts. Today the logs feature reads live pod logs only (kubelet ring buffer) — a crash-looping app loses its history at the exact moment someone investigates. Deploy Loki (buy, not build — same pattern as Prometheus), ship pod logs into it, and make `QueryLogs` read Loki when `BEX_LOKI_URL` is set, with byte-identical fallback when unset. Surface shapes (REST/GraphQL/MCP/UI) do not change. **Status:** done — DoD met and verified live 2026-07-14, on the mock cluster via workstream w3's isolated dev environment (`.pm/w3/dev-3`), not prod: prod's `bex-controller-manager` was still unstable (rechecked live, still flapping `CrashLoopBackOff` after the `MachineHealthCheck`/Hetzner-server-limit incident that caused it had separately resolved), so rather than wait on an unrelated incident, proved the same code path by manually installing Loki + log-shipper on the mock cluster with the exact values ArgoCD would render (`kubectl kustomize` + `helm template`/`install`, no ArgoCD itself involved) and running the acceptance script against dev-3's own Hydra for auth. A restarted pod's pre-restart lines returned over REST, a pre-run time range excluded them (real bounds), and with `BEX_LOKI_URL` unset the same query gave the honest live-only answer while every store-only filter 503'd. Along the way, fixed four real bugs in `scripts/logs-verify.sh` itself (Go workspace resolution, an orphaned `go run` child silently invalidating the fallback assertion, a `set -e`-swallowed MCP soft-fail branch, and a hard `BEX_HYDRA_ADMIN_URL` requirement with no bearer minted — the last of which would have blocked the prod run too) and re-ran the real (patched) script end-to-end to confirm. Full history below.

## Tasks (in order)

| id   | title                                                                                             | est | depends_on | status |
| ---- | -------------------------------------------------------------------------------------------------- | --- | ---------- | ------ |
| t001 | GitOps: Loki (single-binary, filesystem PVC) + log-shipper DaemonSet (Alloy/Promtail), k8s labels  | 30m | —          | — **DONE** (`deploy/gitops/base/loki.yaml` + `log-shipper.yaml`, Alloy chosen; both overlays render) |
| t002 | bex-api: `BEX_LOKI_URL` client — `QueryLogs` via LogQL from the existing `LogQuery` filters        | 30m | t001       | — **DONE** (`internal/logs/loki.go` + `LogHistorySource` seam; wired in `main.go`/`server.go`) |
| t003 | Live tail decision + wiring: SSE stays on pod logs vs Loki tail — pick, implement, document        | 25m | t002       | — **DONE** (tail stays on pod logs; rationale in docs/ADR010-observability.md) |
| t004 | Retention (match Render's window) + env-template name mirrors + docs/ADR010-observability.md caveat swap  | 20m | t002       | — **DONE** (7d = Render Hobby window, tiered note; no `.env` var — non-secret URL like `BEX_PROM_URL`) |
| t005 | Acceptance: pod restart → pre-restart lines still served over REST and MCP `list_logs`             | 25m | t003, t004 | — **DONE** (live-verified 2026-07-14 on the mock cluster via dev-3; REST durable, bounds real, fallback honest — see `done/t005.md`) |
| t006 | Render parity — same log shapes/filters across REST/GraphQL/MCP/UI; compare Render's logs behavior | 20m | t005       | — **DONE** (no drift — pure `QueryLogs` backend swap; parity ledger row refreshed) |
| t007 | Simplify — `/simplify` over the code this milestone changed                                        | 20m | t006       | — **DONE** (diff reviewed; reuse via `lokiLimit`/`labelOr`; gofmt/lint clean) |
| t008 | Test coverage — meaningful tests for the Loki path + fallback                                      | 30m | t006       | — **DONE** (`internal/logs/loki_test.go`: LogQL builder, injection escaping, parser, bounds, Loki-down, routing) |
| t009 | Closeout — DoD met → move milestone to `done/`                                                     | 10m | t008       | — **DONE** (DoD verified live; see `done/t009.md`) |

## Definition of done

Restarting an app's pod does not lose its logs: a query afterwards returns pre-restart lines, bounded by a real time range, through all existing surfaces with unchanged shapes; retention covers at least Render's searchable window; with `BEX_LOKI_URL` unset, behavior is byte-identical to today's pod-log path (omitted, not faked).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-07-09; w3/001's explicit v0 deferral ("Defer a durable log/metric store … to a later milestone"); `lego/backend/internal/logs/service.go` (pod-log source, Render-shaped `LogQuery`); Render keeps ~7 days of searchable logs.
- **Goal linkage:** GOAL.md #2 ("Basic obs for operation"); Render logs parity (pillar 1).
- **Expected outcome:** crash investigations stop racing log rotation; time-range queries become real instead of best-effort over a live stream.
- **Why now:** the deferral was explicit and its precondition is met — the logs API/UI shapes are shipped and stable (w3/m1, w5/m6), so this is a pure backend swap with no surface churn.
- **Render parity closing task: included** — the logs surface spans REST/GraphQL/MCP/UI.
