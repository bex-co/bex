# w3 · m18 — Request-metrics `host`/`path` filters: wire the router-label source (verify-first)

**Worker:** worker3 **Goal:** the request-metrics `host`/`path` filters that today 400 with "not supported" return real filtered series — by verifying, then enabling, Traefik router-level Prometheus labels — and no accepted-but-ignored metrics field remains. **Status:** todo

## Tasks (in order)

| id   | title                                                             | est | depends_on   |
| ---- | ----------------------------------------------------------------- | --- | ------------ |
| t001 | Verify router-label feasibility + cardinality on the live Traefik | 45m | —            |
| t002 | Enable router labels in the gitops Traefik config                 | 30m | t001         |
| t003 | PromQL host/path filter wiring in the metrics service             | 45m | t002         |
| t004 | `ownerId` honored-or-400 + `aggregateBy` instance decision        | 30m | t001         |
| t005 | Dashboard filter hookup check                                     | 30m | t003         |
| t006 | Render parity                                                     | 30m | t004, t005   |
| t007 | Simplify                                                          | 30m | t006         |
| t008 | Test coverage                                                     | 45m | t006         |
| t009 | Closeout                                                          | 15m | t008         |

## Definition of done

A host- or path-filtered request-metrics query returns genuinely filtered series on REST, GraphQL, and MCP (dashboard filters produce visibly different charts), **or** t001 produces an evidence-backed infeasibility/cardinality finding and the milestone re-scopes to documenting it (the w7/m34 verify-first precedent — an honest negative also closes the milestone, with the 400 kept and ADR006:322's successor text updated). Either way, `ownerId` is validated-or-400 rather than silently ignored, and the instance-flavored `aggregateBy` behavior is an explicit recorded decision.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 12, 2026-07-15 — code miner (`metrics/service.go:244` named 400; `graphql.go:201` ownerId ignored; `graphql.go:434` aggregateBy dropped). Extends w3/m12, which made the filters honest but not functional.
- **Goal linkage:** Render metrics parity (docs/ADR010); "nothing accepted is ignored" (the w3/m8 principle).
- **Expected outcome:** the last metrics ◐ with a plausible mechanism closes, or its impossibility is pinned with evidence.
- **Why now:** Traefik's `addRoutersLabels` is a config flip away and w3's open milestones are small wiring; verify-first bounds the risk. Render parity closing task included — REST/GraphQL/MCP/UI behavior change.
