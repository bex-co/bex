# w4 · m147 — Refresh service activity while the page stays open

**Worker:** worker4 **Goal:** newly recorded service events appear on the open Activity page and completed deploys lose their in-progress state and Cancel action. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Refresh the activity head with a bounded moving window | 40m | — |
| t002 | Preserve paging and shared Events/Metrics callers | 35m | t001 |
| t003 | Render parity | 15m | t002 |
| t004 | Simplify | 10m | t003 |
| t005 | Test coverage | 30m | t003 |
| t006 | Closeout | 15m | t004, t005 |

## Definition of done

Repeat [the captured live probes](finding.md) with a new disposable free web service.

- Open Events during its first deploy, keep the document visible, and wait for the API's deploy_ended event. Within the existing 30-second resource refresh baseline after that event is available, the page contains the build/deploy lifecycle rows, shows the completed deploy as Live, and removes the started row's In Progress badge and Cancel action without reload.
- Reload Events once as a control, then set idleTTLSeconds to 60 through the captured GraphQL mutation while leaving the page open. The newly recorded idle_timeout_changed and service_hibernated rows appear within that same baseline after API availability. Wake the sleeping service with the captured public request/retry sequence; its service_woken event also arrives without navigation. A subsequent fresh load contains the same retained event IDs.
- Replay the fixed historical-window and current bounded-window requests. The historical query still excludes later events; the current query includes them. REST, GraphQL and both MCP event tools retain correct event identity and timestamps. Do not remove the backend range cap to make a moving UI query pass.
- Open Metrics, choose Last 30 minutes and show Event timeline. Its queries advance the selected window at the existing 15-second cadence and collect a later event, as the working control did in this sweep.
- Delete the fixture through the dashboard, confirm its REST read is 404 and its exact App/workloads are gone, then revoke the QA session.

Older-page accumulation, hidden-tab behavior, absolute Metrics ranges and sibling resource types were not live-exercised here; their verification is explicit work in t002/t005 rather than an observed-failure claim.

## Source + Goal linkage

- **Source:** continuous qa-find-bugs, 2026-10-02 sweep 7; muse.env login; user-directed w4 placement. [Finding](finding.md) includes complete requests/responses, two page lifetimes, dependency/source trace, aliases and prior-DoD disposition.
- **Goal linkage:** ADR008 reliable hosting; ADR006 one event service behind the adapters; ADR018 service events; docs/render-artifacts/service-events.md's retained, truthful timeline contract.
- **Expected outcome:** a user watching a deploy or service transition sees its recorded outcome without reloading; historical paging and selected Metrics ranges remain usable.
- **Why now:** the header reached Running/Live while Activity retained only Deploy started, In Progress and Cancel for over two minutes after the deploy completed. A fresh page then missed subsequent configuration and hibernation events. These are present in every probed API surface.
- **Sizing:** 145m across bounded refresh, shared paging/range semantics and meaningful temporal regression tests plus standing closing tasks. This is more than adding a pollInterval: a frozen endTime still excludes new rows, and advancing only that end exceeds the 720-hour cap.
- **Render parity included:** this changes a tenant UI surface. Official Render docs support bounded/cursor event reads and visible deployment history; this hunt did not measure Render's authenticated refresh cadence. The 30-second target comes from Bex's existing resource polling policy.

## Dedupe and limits

No open milestone covers this after scanning 36 open/blocked milestone READMEs, inbox notes and completed history. w4/073 fixed the Deploys list; this route does not use that hook. w3/m19 supplied historical windows/cursors, w6/m122 fixed catalog filtering, and w7/m66 supplied lifecycle facts. Their producer/history guarantees are preserved; this is an uncovered live freshness gap, not evidence those source fixes regressed. The finding walks their relevant complete DoDs. Product code is unchanged by this filing.
