# w3 — Observability (worker3)

**Worker:** worker3 The obs surface owned by no workstream today: w1 builds the platform/control-plane, w2 makes it AI-native, w3 makes it _observable_. Maps to `GOAL.md` #2 ("Basic obs for operation"). Ordered by value/dependency: logs first (highest operational value, pure pod-log backend, no metrics-server dependency), then metrics (needs metrics-server + Traefik metrics from w1 platform).

## Milestones

- [x] **m1** — Logs API: query + stream App logs (5 tasks) ← from `001`, brainstorm 2026-07-05
- [x] **m2** — Metrics API: resource + request metrics (5 tasks) ← from `001`, brainstorm 2026-07-05
- [x] **m3** — Metrics page PoC: beancount-cms, Render-style dashboard (5 tasks) ← from user request 2026-07-06, design learned live from Render's metrics page
- [x] **m4** — Resource-metrics history: Prometheus-backed CPU/memory/instances (6 tasks) ← from brainstorm 2026-07-06 (Render metrics-page parity)
