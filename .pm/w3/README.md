# w3 — Observability (worker3)

**Worker:** worker3 The obs surface owned by no workstream today: w1 builds the platform/control-plane, w2 makes it AI-native, w3 makes it _observable_. Maps to `GOAL.md` #2 ("Basic obs for operation"). Ordered by value/dependency: logs first (highest operational value, pure pod-log backend, no metrics-server dependency), then metrics (needs metrics-server + Traefik metrics from w1 platform).

## Milestones

- [x] **m1** — Logs API: query + stream App logs (5 tasks) ← from `001`, brainstorm 2026-07-05
- [x] **m2** — Metrics API: resource + request metrics (5 tasks) ← from `001`, brainstorm 2026-07-05
- [x] **m3** — Metrics page PoC: beancount-cms, Render-style dashboard (5 tasks) ← from user request 2026-07-06, design learned live from Render's metrics page
- [x] **m4** — Resource-metrics history: Prometheus-backed CPU/memory/instances (6 tasks) ← from brainstorm 2026-07-06 (Render metrics-page parity)
- [x] **m4.5** — Metrics page parity: application charts, limits, network filters (6 tasks) ← from user request 2026-07-07, side-by-side gap check of Render's live metrics page vs dashboard.bex.co after m4
- [ ] **m5** — Durable logs: Loki-backed history behind the same API (9 tasks) ← from `/pm-brainstorm for w3` 2026-07-09 (w3/001's explicit v0 deferral)
- [x] **m6** — Platform alerting: Alertmanager + rules for bex's own health (8 tasks) ← from `/pm-brainstorm for w3` 2026-07-09 (alertmanager disabled; backup CronJobs unwatched)
- [x] **m7** — Service events feed (`GET /v1/services/{id}/events`) (8 tasks) — **done 2026-07-12** ([done/m7](done/m7/README.md)): a read-time VIEW over deploys + audit_events (one new column, no event table) across REST/GraphQL/MCP; Events-tab UI rides `w5/007`
- [ ] **m8** — Request/HTTP logs + structured filters over the Loki pipeline (9 tasks) ← promoted from `002` 2026-07-12 (its gate — the m5 log-backend decision — is settled; gated on m5 closeout)
- [ ] **m9** — Deploy notifications: email on deploy success/failure (9 tasks) ← from `/pm-brainstorm more` 2026-07-12 (unblocked by w2/m5 deploy events + w4/m7 SMTP courier, both done)
- [ ] **m10** — Extended resource metrics: autoscale target, disk, DB connections (9 tasks) ← from `/pm-brainstorm for more` 2026-07-12 (closes the "Extended metrics" parity row, unblocked by w1/008→w1/m20 + w1/m17; replication-lag scaffolded pending w1/m22)
- [ ] **m11** — Outbound event webhooks (Render `/webhooks` parity) (12 tasks) ← from `/pm-brainstorm for more` 2026-07-12 (last unowned row in the parity ledger's Platform events & integrations section; composes deploy lifecycle w2/m5 + SMTP courier w4/m7, sequenced after m7's in-flight service-events feed to share one instrumentation pass)

## Inbox

- `003.md` — Log Streams: forward logs to external observability tools (Render parity) — park until m5's shipper exists

> `002.md` (request logs + structured filters) promoted to **m8** 2026-07-12; note moved to `done/`.

> `001.md` (v0 observability backend strategy → docs/ADR010-observability.md) done — the doc shipped and is indexed; moved to `done/001.md` 2026-07-09.
