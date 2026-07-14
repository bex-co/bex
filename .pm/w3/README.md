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
- [x] **m8** — Request/HTTP logs + structured filters over the Loki pipeline (9 tasks) — **done 2026-07-12** ([done/m8](done/m8/README.md)): Traefik access logs ship as `type=request`, app logs carry a parsed `level` (honest `unknown` bucket), REST/GraphQL/MCP honor Render's full filter set, and `list_log_label_values` ships under the official name/args. Nothing accepted is ignored (store-only filters 503, unknown values 400). Filter UI rides `w5/008`
- [x] **m9** — Deploy notifications: email on deploy success/failure (9 tasks) — **done 2026-07-13** ([done/m9](done/m9/README.md)): `backend/internal/notifications` (store/service/REST/GraphQL/MCP) + the reconciler's `DeployNotifier` hook + the dashboard Settings panel. `/simplify` caught and fixed the notify hook blocking the reconciler's hot path before it shipped.
- [x] **m10** — Extended resource metrics: autoscale target, disk, DB connections (9 tasks) — **done 2026-07-13** ([done/m10](done/m10/README.md)): `cpu_target`/`memory_target` (App-scoped, `Metrics`) + a new `DatastoreMetrics` verb for `disk`/`disk_capacity`/`db_connections`/`replication_lag` (Database/KeyValue-scoped), across REST/GraphQL/MCP + a dashboard `DatastoreMetricsPanel`. Replication-lag is wired but gated on `highAvailabilityEnabled` (omitted, not a fake zero) until `w1/m22` ships.
- [x] **m11** — Outbound event webhooks (Render `/webhooks` parity) (12 tasks) — **done 2026-07-14** ([done/m11](done/m11/README.md)): `backend/internal/webhooks` — endpoint CRUD (mint-once `whsec_` secret) across REST/GraphQL/MCP + dashboard Settings panel; the delivery worker tails the SAME deploys+audit_events composition m7 reads through a durable watermark (no new write path), signs per Standard Webhooks, retries 8× (~33h) with auto-disable + email notice; live-verified end to end by `scripts/webhooks-verify.sh`. Found + fixed en route: w2/m30's write-path consolidation made `core.callerVerb` record `apps.writeThroughStore`/`apps.patch` as the audited verb, silently un-mapping suspend/resume/restart/plan/scale/cron-run from m7's events feed — the helper-frame walk restores verb attribution for both features. ← from `/pm-brainstorm for more` 2026-07-12
- [ ] **m12** — Metrics `host`/`path` filter honesty fix (8 tasks) ← from `/pm-brainstorm more` 2026-07-13 (fourth pass; `docs/ADR006-bex-api.md:322` — host/path filters accepted but silently unapplied)
- [ ] **m13** — Fix log-shipper N× duplication (7 tasks) ← promotes `004` 2026-07-13 (node-scope Alloy's cluster-wide pod discovery; needs a live multi-node cluster to verify safely)

## Inbox

- `005.md` — "deploy started" notification toggle (w3/m9 residual, sub-hour) ← from `/pm-brainstorm more milestones for each worker` 2026-07-14; filed rather than scheduled — w3's queue is full

> `003.md` closed 2026-07-13 — conflicts with `.pm/DO_NOT_DO.md`'s "external log/metric drains — non-goal" entry; not built, moved to `done/003.md`.
> `004.md` promoted to **m13** 2026-07-13; note moved to `done/`.

> `002.md` (request logs + structured filters) promoted to **m8** 2026-07-12; note moved to `done/`.

> `001.md` (v0 observability backend strategy → docs/ADR010-observability.md) done — the doc shipped and is indexed; moved to `done/001.md` 2026-07-09.
