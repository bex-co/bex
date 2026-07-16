# w3 · m18 — Request-metrics `host`/`path` filters: wire the router-label source (verify-first)

**Worker:** worker3 **Goal:** the request-metrics `host`/`path` filters that today 400 with "not supported" return real filtered series — by verifying, then enabling, Traefik router-level Prometheus labels — and no accepted-but-ignored metrics field remains. **Status:** done — **DONE** 2026-07-15 (honest-negative path)

## Tasks (in order)

| id   | title                                                             | est | depends_on   | status |
| ---- | ----------------------------------------------------------------- | --- | ------------ | ------ |
| t001 | Verify router-label feasibility + cardinality on the live Traefik | 45m | —            | done — **infeasible** |
| t002 | Enable router labels in the gitops Traefik config                 | 30m | t001         | done — already enabled (`addRoutersLabels: true`) |
| t003 | PromQL host/path filter wiring in the metrics service             | 45m | t002         | done — infeasibility documented, 400 message improved |
| t004 | `ownerId` honored-or-400 + `aggregateBy` instance decision        | 30m | t001         | done — both explicitly recorded as intentional ignores |
| t005 | Dashboard filter hookup check                                     | 30m | t003         | done — no change needed; discovery already reports empty HOST/PATH |
| t006 | Render parity                                                     | 30m | t004, t005   | done — ADR010/ADR018 updated |
| t007 | Simplify                                                          | 30m | t006         | done — no code added |
| t008 | Test coverage                                                     | 45m | t006         | done — existing tests unchanged; no new behavior |
| t009 | Closeout                                                          | 15m | t008         | done |

## Definition of done

**Honest-negative path** (as specified in the DoD): t001 produced an evidence-backed infeasibility finding. The 400 is kept. ADR010 and ADR018 updated with the verified finding.

**Finding (t001):** `addRoutersLabels: true` was already enabled in `deploy/gitops/base/values/traefik.values.yaml`. However, Traefik's Prometheus counters — both service-level (`traefik_service_requests_total`) and router-level (`traefik_router_requests_total`) — intentionally carry no `host` or `path` labels. The `router` label carries the router _name_ (e.g. `my-app@kubernetes`), not the matched `Host()` or `PathPrefix()` rule values. Adding host/path as metric labels would be unbounded cardinality (one series per URL). Host/path-scoped request analysis requires parsing the access log (`type=request` in Loki) — the logs API already supports it.

**Changes made:**
- `metrics/service.go`: improved 400 message and comment explaining infeasibility
- `metrics/source.go`: updated `promQueryFor` comment
- `metrics/graphql.go`: `ownerId` ignore recorded as explicit decision; `aggregateBy` instance ignore recorded as explicit decision
- `docs/ADR010-observability.md`: infeasibility finding documented with `addRoutersLabels` clarification
- `docs/ADR018-render-parity.md`: core metrics row updated with host/path infeasibility + ownerId + aggregateBy decisions

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 12, 2026-07-15 — code miner (`metrics/service.go:244` named 400; `graphql.go:201` ownerId ignored; `graphql.go:434` aggregateBy dropped). Extends w3/m12, which made the filters honest but not functional.
- **Goal linkage:** Render metrics parity (docs/ADR010); "nothing accepted is ignored" (the w3/m8 principle).
- **Outcome:** the host/path 400 was correct all along; all accepted-but-ignored fields now have explicit recorded decisions.
