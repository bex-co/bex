# CLI sweep 693 — event `type` filter vs bex extension types (live, 2026-10-19)

Read-only against the baseline `hello-go`.

- Unfiltered `GET /services/{id}/events?limit=5` returned the one retained event, type `service_woken` (from sweep 675's wake).
- `?type=deploy_ended` → 200 `[]` (truthful: no such event in retention); `?type=bogus_type` and the mixed `deploy_ended,bogus_type` → 400 `type must be one of …` listing the pinned Render enum — the comma-list gate enum-checks every element, as designed (ADR018 w1/m165).
- `?type=service_woken` → the same 400: `service_woken`/`service_hibernated` are deliberate bex extension types (ADR018, w8/060 — Render records no spin-down event, and its `service_suspended` would misattribute the actor) that the REST filter's strict Render enum cannot name. Documented ◐ drift: "a strict-enum client sees an unknown string, as for every bex-named type". The events remain listable unfiltered and via GraphQL/MCP.

No bug filed.
