# w1 · m47 — Events/Deploys consolidation for Render parity

**Worker:** worker1 **Goal:** Unify the Deploys and Events dashboard pages into a single, Render-compatible Events view that shows deployment history + service activity in one timeline. Achieve full API surface parity (REST/GraphQL/MCP all expose events correctly). **Status:** todo

## Tasks (in order)

| id   | title                                                                                    | est   | depends_on |
| ---- | ---------------------------------------------------------------------------------------- | ----- | ---------- |
| t001 | Audit Render's events UX: review render.com/web/srv-…/events, capture exact behavior    | 45m   | —          |
| t002 | Verify MCP has events tool; add if missing                                              | 30m   | —          |
| t003 | Consolidate dashboard: enrich Events GraphQL query to carry full deploy details          | 2h    | t001       |
| t004 | Merge Deploys and Events pages: unified tab showing both deploy history + config events | 2h    | t003       |
| t005 | Remove or hide the separate Deploys tab; update navigation                              | 30m   | t004       |
| t006 | Render parity: verify REST/GraphQL/MCP consistency across surfaces                      | 1h    | t005       |
| t007 | Simplify: reuse patterns, clean up dead page/component code from old Deploys path       | 1h    | t006       |
| t008 | Test coverage: add tests for merged Events page with deploy + audit event interleaving  | 1h    | t007       |
| t009 | Closeout: verify DoD met, move milestone to done                                         | 15m   | t008       |

## Definition of done

- Events page displays deploy transitions (started/ended) **and** audit events (config changes) in one newest-first timeline
- Dashboard navigation shows one unified Events tab (Deploys tab removed or links to filtered Events)
- REST `GET /v1/services/{id}/events`, GraphQL `serviceEvents`, and MCP events tool (if it exists) all return the same event stream
- Both deploy-specific details (commit, image, trigger) and audit metadata (actor, target change) are visible in the Events view
- Existing deploy detail page (`/services/{id}/deploys/{deployId}`) remains or is aliased into Events detail
- No dead code from the old separate Deploys page remains
- Full test coverage for the merged view including pagination, filtering, and interleaved event types

## Source + Goal linkage

- **Source:** User research (2026-07-16, playwright research + Render comparison)
- **Goal linkage:** Render parity (ADR018, w1/m45–m46 surface-consolidation sequence)
- **Expected outcome:** Dashboard Events tab matches Render's `/events` behavior: unified timeline of deployments + service activity
- **Why now:** w1/m45–m46 completed route and navigation parity; Events API backend exists (w3/m7); this closes the Render-parity feature surface for service management

