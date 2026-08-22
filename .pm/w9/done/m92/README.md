# w9 · m92 — Agent-sessions dashboard QA-hunt fixes

**Worker:** worker9 **Goal:** the `/agents` dashboard (list + detail) tells the truth about every session — no fake PR references, a search box that actually narrows results, copy that reads as English, a session's status agrees everywhere it's shown, failed sessions show their real transcript, and long user-submitted task text never floods the accessible tree **Status:** done

## Tasks (in order)

| id   | title                                                                              | est | depends_on          |
| ---- | ----------------------------------------------------------------------------------- | --- | -------------------- |
| t001 | Fix fake "#0" PR badge on sessions with no real PR                                  | 40m | —                     | — **DONE** |
| t002 | Sidebar session search is practically non-functional against long task titles       | 30m | —                     | — **DONE** |
| t003 | Stale "Recent" section heading + malformed `archived=%22true%22` query param         | 35m | —                     | — **DONE** |
| t004 | Bound the raw task text feeding the DOM/accessible name of every session row        | 35m | —                     | — **DONE** |
| t005 | Failed sessions with a persisted transcript render "conversation stream unavailable" | 45m | —                     | — **DONE** |
| t006 | Grammar fixes: "N turns" pluralization + broken idle-session banner sentence         | 20m | —                     | — **DONE** |
| t007 | Sidebar session list shows a stale phase that disagrees with the detail page        | 40m | —                     | — **DONE** |
| t008 | De-duplicate the unbounded 5s AgentSessions poll against the existing SSE stream     | 35m | —                     | — **DONE** |
| t009 | Render parity check (REST/GraphQL/MCP + dashboard)                                  | 30m | t001–t008              — **DONE** |
| t010 | Simplify                                                                             | 20m | t009                  | — **DONE** |
| t011 | Test coverage                                                                        | 45m | t009                  | — **DONE** |
| t012 | Closeout                                                                             | 10m | t011                  | — **DONE** |

## Definition of done

- No session anywhere in the dashboard (list, sidebar, detail header) shows a `#0` PR badge; sessions without a real PR show no PR badge at all, and the badge for sessions that *do* have a PR is unaffected.
- Typing a realistic query (e.g. a distinctive word from a task's title) into the sidebar "Search sessions" box measurably narrows the visible list instead of matching nearly everything via unrestricted subsequence fuzzy matching.
- The `<h2>` above the Recent/Archived/All tabs reflects the active tab, and the Archived tab's URL is a clean `archived=true` (or an equivalent non-JSON-collision value) instead of `archived=%22true%22`.
- A session with a very long task prompt renders a length-bounded title/tooltip/accessible-name in every list row (sidebar + main list); the full raw text is never in the DOM/ARIA tree outside the session's own detail page.
- The repro'd Failed session (`ags-da2i7o2rmfbs73eu90b0`, and any other terminal session with a persisted transcript) replays its transcript instead of showing "The conversation stream is unavailable right now."
- Session-detail header shows "1 turn" (singular) for a single-turn session, and the idle-session banner reads as grammatical English.
- The sidebar list's status phrase for a given session never disagrees with that same session's own detail-page status (repro: `ags-da38m165qpic73e6v48g` showed "Working…" in the list while its detail page showed "Hibernated" + an idle banner).
- The sidebar's background `AgentSessions` poll no longer refetches the full unbounded session history every 5 seconds while a session detail page (with its own SSE stream) is open — either capped/paginated or reduced in scope.
- Regression tests cover each fixed behavior; `yarn typecheck && yarn lint && yarn test` (dashboard) and `go test ./...` + `make lint` (backend, if t001/t003 touch it) all green.

## Source + Goal linkage

- **Source:** two independent live-Playwright QA hunts of `dashboard.bex.co/agents` on 2026-08-21 (one by the assistant, one relayed by the user from a parallel session), cross-verified against the dashboard source (`dashboard/src/features/agent-sessions/`, `dashboard/src/routes/agents.tsx`, `dashboard/src/common/components/dashboard-layout/agent-sessions-nav-section.tsx`) and the backend (`lego/backend/internal/agentsessions/`, `lego/backend/internal/sshgateway/agentattach/`). Continues the `w9/m89`–`m91` pattern of materializing live-dashboard-hunt findings directly into a milestone.
- **Goal linkage:** [docs/ADR047-cloud-coding-agent-sessions.md](../../docs/ADR047-cloud-coding-agent-sessions.md) — the Agents feature's dashboard is the primary trust surface for cloud coding sessions; a dashboard that shows a fake PR reference, a dead-looking search box, disagreeing status labels, and a dead-end on failed sessions actively undermines that trust. The DOM/accessible-name bloat item is a straightforward accessibility defect (WCAG name-role-value) independent of the agents theme.
- **Expected outcome:** the `/agents` surface (list, sidebar, detail) is honest and internally consistent — every finding in "Definition of done" above is fixed and regression-tested.
- **Why now:** these are live, user-visible defects on a feature real workspaces are actively using today (the historical session list is strewn with the fake `#0` badge across dozens of real sessions); most are small, well-localized fixes now that exact file/line citations are in hand, and several (search, stale status, redundant polling) compound the longer the underlying causes go unaddressed.
- **Render parity task included** because t001 (PR badge) touches the GraphQL/REST `prNumber` field shape and t003 (archived query param) touches how the `archived` filter value is represented — both need a REST/GraphQL/MCP consistency check alongside the dashboard fix. t002/t004/t006/t007/t008 are dashboard-only but ride the same check for completeness.
