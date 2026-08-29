# w6 · m134 — Record project/environment reassignments in the service Events feed

**Worker:** worker6 **Goal:** a successful service move between projects or environments leaves one truthful, service-scoped event on REST, GraphQL, MCP, and the dashboard, without colliding with the existing env-var-change event **Status:** in progress — t001–t006 done 2026-08-28 (contract `service_moved` + typed audit columns, emission from both funnels, all API/webhook/dashboard surfaces, parity record, simplify pass applied, regression coverage green incl. real-Postgres store suite); t007 (closeout) remains: the live-production DoD bullet needs the change shipped/deployed and a real move observed in both the workspace audit log and the service Events feed

## Tasks (in order)

| id   | title                                                                            | est | depends_on |
| ---- | -------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Define a non-colliding reassignment event contract and detail shape — **DONE**    | 35m | —          |
| t002 | Emit exactly one event from both successful membership funnels — **DONE**         | 45m | t001       |
| t003 | Carry and render the event across REST, GraphQL, MCP, and dashboard — **DONE**    | 45m | t002       |
| t004 | Render parity — **DONE**                                                          | 30m | t003       |
| t005 | Simplify — **DONE**                                                               | 20m | t004       |
| t006 | Test coverage — **DONE**                                                          | 40m | t004       |
| t007 | Closeout                                                                          | 10m | t006       |

## Definition of done

- Moving a service between projects and moving it between environments each records exactly one service event after the membership write succeeds; no-op and failed writes record none.
- Event details identify the previous and new placement without leaking tenant-internal Kubernetes names.
- The new event is distinct from `service_environment_changed`, which remains the existing env-var/config-rollout fact.
- REST, GraphQL, MCP, webhook delivery, and the dashboard Events tab expose the same event type and details, with translated dashboard copy.
- A live production move is visible in both the workspace audit log and the service Events feed, and the two records describe the same successful change.
- Backend and dashboard regression suites cover project moves, environment moves, no-ops, failures, and the existing env-var-change control.

## Source + Goal linkage

- **Source:** promoted from [`w6/054`](../done/054.md), live-reproduced 2026-08-27: `environments.SetServices` was present in the workspace audit log while the service feed remained byte-identical before and after the move.
- **Goal linkage:** [ADR006](../../../docs/ADR006-bex-api.md) and [ADR018](../../../docs/ADR018-render-parity.md): the service feed is the user-facing record of service changes and is shared by REST, GraphQL, MCP, webhooks, and the dashboard.
- **Expected outcome:** a tenant can answer when and where a service moved from the service's own history instead of correlating a separate workspace audit stream.
- **Why now:** the move already produces durable audit evidence, but the service-facing history silently drops it; leaving the two truth surfaces inconsistent makes successful organizational changes look as though they never happened.
- **Render parity:** included because this adds a tenant-visible event vocabulary entry on all public API and UI surfaces; compare Render's equivalent project/environment move behavior before settling the wire spelling.
- **Triage 2026-08-28:** keep — re-verified against the current tree: the vocabulary in `lego/backend/internal/events/service.go` has no reassignment type, and `lego/backend/internal/events/vocabulary_test.go` still carries the deliberate w6/m19 exemption for `environments.SetServices` ("not itself a per-App verb … environments is a grouping feature, like projects, neither of which has one yet"). That exemption map is the guard t002 must revisit when the new event lands.
