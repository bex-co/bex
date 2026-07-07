# w3 · m2 — Metrics API: resource + request metrics

**Worker:** worker3 **Goal:** Render-compatible read-only metrics over bex-api — CPU/memory/instance-count time series plus request rate, latency percentiles, status codes, and bandwidth for an App. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Resource-metrics backend: CPU/memory from metrics.k8s.io + instance count + RBAC — **DONE** | 45m | — |
| t002 | REST adapter for resource metrics: cpu/memory/instance_count, percentage vs total, time range — **DONE** | 40m | t001 |
| t003 | Request-metrics backend: request count, latency percentiles, status codes, bytes from Traefik — **DONE** | 45m | t001 |
| t004 | REST + GraphQL adapters for request metrics + filters (status/host/path, group-by) — **DONE** | 40m | t003 |
| t005 | Cluster enablement (metrics-server + Traefik metrics) + tests + docs — **DONE** | 40m | t001, t003 |

## Definition of done

REST + GraphQL return resource metrics (CPU/memory percentage-or-total, instance count) and request metrics (count, p50/p90/p99 latency, status codes, outbound bytes) for an App over a chosen time range, with status/host/path filters; verified end-to-end against the mock cluster with metrics-server present; `make test` green; documented in `docs/bex-api.md` + `docs/observability.md`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-07-05 (Render `/metrics` page) + inbox note `w3/001`.
- **Goal linkage:** `GOAL.md` #2 ("Basic obs for operation"); seeds `GOAL.md` #5 (usage metering builds on the request/bandwidth counters).
- **Expected outcome:** the data behind a Render-style metrics page is queryable through bex-api — resource pressure and request health per App without `kubectl top` / Prometheus access.
- **Why now:** completes "basic obs" alongside logs, and the request/bandwidth counters are the seed the later multi-tenant usage-metering milestone depends on, so building them now avoids a second pass over ingress metrics.
- **Risk/dependency:** needs **metrics-server** (resource) and **Traefik metrics enabled** (request) in-cluster — platform infra owned by w1/GitOps; t005 tracks it. If metrics-server is absent from the mock cluster, that is a blocking w1 prerequisite.
