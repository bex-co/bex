# w2 · m31 — ListDeploys pagination + filtering (REST/GraphQL/MCP)

**Worker:** worker2 **Goal:** `deploys.Service.List(ctx, service string)` takes no arguments — every surface (`GET .../deploys`, GraphQL `deploys(serviceId)`, MCP `list_deploys`) always returns the full, unbounded deploy history. Render's real `ListDeploysParams` supports `status[]`/`createdBefore`/`createdAfter`/`cursor`/`limit`; add the subset bex can honestly back today. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | `store.ListDeploys`: add `status[]`/`createdBefore`/`createdAfter`/`cursor`/`limit` query support             | 40m | —          |
| t002 | REST: `GET .../deploys` reads the matching query params                                                       | 20m | t001       |
| t003 | GraphQL: `deploys(serviceId, status, createdBefore, createdAfter, cursor, limit)`                              | 20m | t001       |
| t004 | MCP: `list_deploys` gains `limit`/`cursor` — matches Render's own real MCP tool (bex's currently doesn't)      | 20m | t001       |
| t005 | Render parity — cross-surface consistency; refresh `docs/ADR018-render-parity.md`'s "List / get deploy" row   | 15m | t004       |
| t006 | Simplify — `/simplify` over the code this milestone changed                                                   | 15m | t005       |
| t007 | Test coverage — filter combinations, cursor pagination correctness, limit bounds                              | 30m | t005       |
| t008 | Closeout — DoD met → move milestone to `done/`                                                                 | 10m | t007       |

## Definition of done

An agent (or the dashboard, once it grows deploy-history UI) can page through a long-lived service's deploy history instead of always receiving the entire unbounded list, and can filter by status/date range on all three surfaces identically. `docs/ADR018-render-parity.md`'s "List / get deploy objects" row states the pagination/filter support explicitly instead of implying full completeness under a bare ✅.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-07-14, same audit as `w2/m30` — ground truth `render-oss/render-mcp-server`'s `types_gen.go` `ListDeploysParams` and `pkg/deploy/tools.go`'s real `list_deploys` MCP tool (which itself takes `limit`/`cursor`, unlike bex's).
- **Goal linkage:** pillar 3 (agent-native — an agent polling deploy history for a service with hundreds of deploys should not have to fetch all of them every time) + pillar 1 (Render parity, including matching Render's own MCP tool's completeness, not just the REST API's).
- **Expected outcome:** `list_deploys`/`GET .../deploys`/`deploys(serviceId)` all support bounded, filterable queries; the parity ledger reflects it accurately.
- **Why now:** cheap, additive, no design ambiguity (unlike `m30`'s `deployMode` question) — safe to do independently and in parallel with `m30`.
- **Render parity closing task: included** — REST, GraphQL, and MCP all change; no dashboard UI surface exists yet for deploy history browsing beyond the Events tab's already-limited feed, so UI is out of scope here.
