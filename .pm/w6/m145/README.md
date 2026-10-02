# w6 · m145 — Render list_events and multi-type event filtering

**Worker:** worker6 **Goal:** Render-compatible agents can list activity using upstream's tool name and multi-type filters. **Status:** todo

**Size:** 2h30 implementation + 1h45 standing closing work = ~4h15 (7 tasks).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add multi-type matching to shared events filters and REST | 50m | — |
| t002 | Implement Render's list_events MCP contract | 50m | w6/m145/t001 |
| t003 | Preserve the legacy tool and refresh the reviewed upstream pin | 50m | w6/m145/t002 |
| t004 | Render parity | 30m | w6/m145/t003 |
| t005 | Simplify | 20m | w6/m145/t004 |
| t006 | Test coverage | 40m | w6/m145/t004 |
| t007 | Closeout | 15m | w6/m145/t005, w6/m145/t006 |

## Definition of done

- [ ] `list_events` exposes the reviewed upstream arguments: required `serviceId`, optional `eventTypes` array, time bounds, cursor, limit and workspace selection; its output matches the captured upstream contract.
- [ ] REST comma-separated event types and MCP arrays select the union, preserving exclusive paging, time bounds, ordering and workspace/resource authorization. Single-type and omitted-type behavior remain compatible.
- [ ] The existing GraphQL events read continues to work; any additive multi-type argument delegates to the same core without breaking its existing single-type argument. The dashboard's existing feed remains functional.
- [ ] `list_service_events` remains callable with its existing arguments and response, delegating to the same implementation. No published tool is removed.
- [ ] The upstream pin has reviewed commit/date provenance and reflects the implemented contract; MCP parity assertions are unchanged or strengthened. Any additional upstream drift is recorded separately rather than acknowledged falsely or built here.
- [ ] Meaningful backend/composed-adapter regressions cover multi-type selection, paging, legacy callers, empty results and denied/foreign resources. Relevant backend gates pass.
- [ ] ADR018's Service events / activity feed row records the actual result and remaining divergences. No unrelated partial row is declared complete.

## Source + Goal linkage

- **Source:** User approved the two Render-only proposals for w6 on 2026-09-30. Research checkout: `677cdec98`. Absorbs the entire implementation scope of `w1/m165/t003`, including multi-type filtering and pin refresh; see [the original investigation](../../w1/done/m165/README.md). That task is removed from w1's active work, not marked implemented.
- **Goal linkage:** ADR008 pillars 1 and 3: Render-compatible APIs and agent tools that transfer without client rewrites.
- **Expected outcome:** An agent using Render's `list_events(eventTypes: [...])` can retrieve the requested activity on bex; older bex agents retain their tool.
- **Why now:** w1/m165 captured upstream `d9a8abd53669` adding this tool while bex still exposes only `list_service_events(type: string)`. The REST comma-joined filter also currently matches one literal string rather than the intended union.
- **Render parity:** Included; this is tenant-facing REST/MCP behavior and the existing GraphQL/UI consumers must remain coherent. Use ADR018's Service events / activity feed row and the reviewed upstream source (`render-oss/render-mcp-server`, `pkg/event/tools.go`).

## Scope and handoff

The user-approved recommendation is to retain `list_service_events` as a compatibility alias. `get_service_event` remains unchanged. The w1 watchdog milestone retains diagnostics and workflow acceptance and depends on this milestone's closeout. Blueprint `buildSources`, Blueprint schema re-pinning, new event sources and other MCP tools are outside this scope. A real unrelated upstream drift must remain visible; it is not grounds to weaken the parity assertion.

Implementation can begin independently: w1/m165/t002's implementation decision is already recorded. Use isolated dev-6 resources if live verification is needed. No production rollout or implementation is authorized by this board-filing operation.

## Queue refresh — 2026-10-02

The filing rebase found w1/m165 already completed in upstream (`1d07a8b2e`, closed 2026-10-02). Its completed records were preserved, and the proposed transfer did not replace them. This item remains pending evidence-based triage against the shipped implementation; no second implementation is scheduled blindly.
