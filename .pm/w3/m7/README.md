# w3 · m7 — Service events feed (`GET /v1/services/{id}/events`)

**Worker:** worker3 **Goal:** The activity feed the parity matrix parked behind two prerequisites that are both now done — deploy objects (w2/m5) and the audit log (w4/m10): one paged, newest-first events surface per service composing deploy transitions, lifecycle/config writes, and scale/sleep transitions, under Render's `GET /services/{id}/events` shape. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                       | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Event model + store view: compose deploys + audit_events + scale/sleep transitions; derived-vs-recorded per type | 35m | —          |
| t002 | REST `GET /v1/services/{id}/events` (Render envelope/cursor, shapes vs OpenAPI) + GraphQL `serviceEvents`        | 30m | t001       |
| t003 | MCP: mirror the official server's events tool if one exists, else document the omission                          | 15m | t002       |
| t004 | Acceptance: suspend + deploy + scale a service → feed shows each, newest-first, paged; no secrets                | 25m | t002       |
| t005 | Render parity — surface consistency + Render event-type comparison; matrix events row ✖ → status                 | 20m | t003, t004 |
| t006 | Simplify — `/simplify` over the code this milestone changed                                                      | 20m | t005       |
| t007 | Test coverage — composition correctness, cursor stability, redaction, store-less 503                             | 30m | t005       |
| t008 | Closeout — DoD met → move milestone to `done/`                                                                   | 10m | t007       |

## Definition of done

A service's events endpoint returns a truthful, cursor-paged, newest-first feed whose entries correspond 1:1 to real deploys, lifecycle/config verbs, and scale/sleep transitions — mapped onto Render's event vocabulary where one exists, bex-named where it doesn't (documented); env-var and secret values never appear in any event; store-less mode (`BEX_CP_DB_URI` unset) returns 503 (omitted, not faked); the matrix's "Service events / activity feed" row moves from ✖.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-12; docs/ADR018-render-parity.md row "Service events / activity feed" (✖, previously cross-referenced to "w2/m5 (deploy objects) + w4/m10 (audit log)" — both done, making this a composition milestone); Render `GET /services/{id}/events` (OpenAPI).
- **Goal linkage:** GOAL.md #2 (observability — "what happened to my service") + pillar 1 (Render service-detail parity); pillar 3 (agents reason over activity, not just current state).
- **Expected outcome:** "what happened to this service overnight?" is one API call; the last unowned observability row in the matrix's Services section gets an owner.
- **Why now:** both data sources just landed and every new write verb (w2/m10 cancel/rollback queued in parallel) widens the surface a later retrofit must cover; composing now is a view, later it's an excavation. Placed in w3 by topical fit (activity/observability) and load.
- **Render parity closing task: included** — REST/GraphQL(/MCP) surface. The dashboard **Events tab** is deliberately out — filed as `w5/007` (shared with w2/m10's deploy-list buttons), per the UI-half pattern.
