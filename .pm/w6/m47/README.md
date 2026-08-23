# w6 · m47 — Free-tier sleep/wake broken (blocker) + event/log copy conflates hibernate with suspend

**Worker:** worker1 **Goal:** a hibernated free-tier service actually wakes on the next request (the core free-tier value proposition), and the platform's own event/log copy stops asserting contradictory or misleading states. **Status:** todo

## Tasks (in order)

| id   | title                                                                       | est | depends_on           |
| ---- | ---------------------------------------------------------------------------- | --- | --------------------- |
| t001 | Root-cause + fix why a hibernated free-tier service never wakes (404 forever) | 90m | —                      |
| t002 | Events feed labels auto-hibernate "Service suspended", contradicting the user-suspend-only invariant | 30m | —                      |
| t003 | Logs empty-state title never branches on the active filter                  | 15m | —                      |
| t004 | Render parity                                                                | 30m | t001, t002, t003      |
| t005 | Simplify                                                                     | 20m | t004                   |
| t006 | Test coverage                                                                | 45m | t004                   |
| t007 | Closeout                                                                     | 10m | t006                   |

## Definition of done

- A freshly created Free-tier web service, hibernated by letting its idle timeout elapse with zero requests, wakes and serves its real application response on the very next request to its `.onbex.co` URL — no `404 page not found`, no indefinite `Hibernated` phase across repeated requests.
- The Events tab (and webhook/notification payloads) for that same hibernate→wake cycle report something textually distinct from a user clicking Suspend/Resume — not the identical `"Service suspended"`/`"Service resumed"` labels.
- The Logs tab's empty state, when a search/filter yields zero results on a service that has produced real log history, shows an internally consistent message (no simultaneous "no logs yet" + "no logs match your filter").
- REST, GraphQL, and MCP agree with the dashboard on all of the above (t004).
- New regression tests for the hibernate/wake cycle, the event-fact branching, and the logs empty-state title exist and are proven red-before/green-after (t006).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt, 2026-08-22 (this hunt's loop iteration; part of the `w6` continuous QA cadence — see `w9/m89`, `w9/m92`, and this workstream's own `w6/m44`–`w6/m46` for the same lineage). Evidence: repeated `curl -D -` + GraphQL `phase` probes against `qa-20260822-sleep2` (deleted, cleaned up), and exact `file:line` root causes cited in each task from a direct source read of `lego/operator/cmd/activator/main.go`, `lego/operator/internal/controller/app_controller.go`, `lego/backend/internal/store/event_facts.go`, and `dashboard/src/features/logs/components/log-viewer.tsx`.
- **Goal linkage:** [ADR029](../../../docs/ADR029-static-sites.md)'s sibling economics note and the operator's own `BEX_ACTIVATOR_SERVICE` contract (`lego/operator/CLAUDE.md`) — dense bin-packing and the free tier's cost model depend on auto-hibernate/wake actually working; a free service that never wakes is not "slow," it is silently broken, which undermines the entire free-tier pitch in [ADR008](../../../docs/ADR008-vision.md).
- **Expected outcome:** a hibernated free service is reachable again within its advertised wake window, every time; the platform's own audit trail is trustworthy enough that a user (or their webhook/notification integration) can tell an intentional suspend apart from a routine sleep cycle.
- **Why now:** this is the highest-severity finding of this hunt after `w6/m46`'s security issue — it breaks a core, load-bearing product promise (not an edge case; any idle free service reproduces it) and was caught live, not theorized.
- **Render parity:** included (t004) — this milestone touches REST/GraphQL/MCP reads of service phase and events, plus the dashboard UI, and Render's own audit-log/sleep-wake behavior is the comparison baseline (ADR006, ADR018).
