# w3 — Observability (worker3)

**Worker:** worker3 The obs surface owned by no workstream today: w1 builds the platform/control-plane, w2 makes it AI-native, w3 makes it _observable_. Maps to `GOAL.md` #2 ("Basic obs for operation"). Ordered by value/dependency: logs first (highest operational value, pure pod-log backend, no metrics-server dependency), then metrics (needs metrics-server + Traefik metrics from w1 platform).

## Milestones

- [x] **m1** — Logs API: query + stream App logs (5 tasks) ← from `001`, brainstorm 2026-07-05
- [x] **m2** — Metrics API: resource + request metrics (5 tasks) ← from `001`, brainstorm 2026-07-05
- [x] **m3** — Metrics page PoC: beancount-cms, Render-style dashboard (5 tasks) ← from user request 2026-07-06, design learned live from Render's metrics page
- [x] **m4** — Resource-metrics history: Prometheus-backed CPU/memory/instances (6 tasks) ← from brainstorm 2026-07-06 (Render metrics-page parity)
- [x] **m4.5** — Metrics page parity: application charts, limits, network filters (6 tasks) ← from user request 2026-07-07, side-by-side gap check of Render's live metrics page vs dashboard.bex.co after m4
- [ ] **m5** — Durable logs: Loki-backed history behind the same API (9 tasks) ← from `/pm-brainstorm for w3` 2026-07-09 (w3/001's explicit v0 deferral)
- [ ] **m6** — Platform alerting: Alertmanager + rules for bex's own health (8 tasks) ← from `/pm-brainstorm for w3` 2026-07-09 (alertmanager disabled; backup CronJobs unwatched)

## Inbox

- `002.md` — Request/HTTP logs + structured log filters (level · status · method · path · instance) — from the w1/m13 parity audit; request logs likely ride m5's pipeline
- `003.md` — Log Streams: forward logs to external observability tools (Render parity) — park until m5's shipper exists

> `001.md` (v0 observability backend strategy → docs/observability.md) done — the doc shipped and is indexed; moved to `done/001.md` 2026-07-09.
