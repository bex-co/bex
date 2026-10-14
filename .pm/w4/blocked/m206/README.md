# w4 · m206 — Make environment-group creation and moves recoverable

**Worker:** worker4 **Goal:** ordinary scoped group creation and a two-group move complete within the existing API budget, and an interrupted write has a discoverable, recoverable outcome. **Status:** blocked (release pipeline deploys backend + dashboard; then QA replays the DoD and confirms the production name residue freed itself, for t008). t001–t007 done.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Bound scoped creation and move work without full environment hydration — **DONE** | 40m | — |
| t002 | Recover failed creation and its owned name reservation — **DONE** | 50m | t001 |
| t003 | Reconcile interrupted membership moves and source selection — **DONE** | 45m | t001, t002 |
| t004 | Audit shared callers, aliases and passing controls — **DONE** | 30m | t003 |
| t005 | Render parity — **DONE** | 15m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 40m | t005, t006 |
| t008 | Closeout | 20m | t007 |

## Definition of done

Repeat [the actual native journeys](finding.md) with fresh owned qa-prefixed empty, unlinked groups and a disposable project with Left and Right environments:

- From a fresh dashboard, create one empty group directly in Left. One Create click returns its ID within the existing GraphQL budget and the group appears once in the complete owner list and Left membership. Workspace-scope creation and a single Manage → Move group remain passing controls.
- After an interrupted scoped create, read the complete owner group list before retrying the same name. The result is either one discoverable committed group or a recoverable failed attempt that permits one subsequent creation; it cannot remain an invisible duplicate-name conflict. Recover the exact residual `qa-20261013-a385-first` only after proving the original attempt is no longer active and the reservation's owner/outcome. Record its disposition.
- Put two unlinked groups in Left. Select the first, search for the second, select all visible resources, then clear the search: both remain selected. Move them together to Right. A settled successful response and ready page show Left empty, Right containing both IDs, and no selected source resources. Repeat from a fresh 390px page.
- If the move times out, the client reads the authoritative source and target before offering another write. It shows the actual complete or partial outcome, and selection contains only groups still in the canonical source membership. The recorded desktop case must never show one source row with “Selected: 2.” Preserve selected groups hidden only by search.
- Delete each visible fixture by exact ID; its REST read returns 404 and all seven starting inventory ID sets are restored. Record any failed-name recovery residue separately from visible cleanup. Revoke only this QA session.

Injected late-acknowledgment, compensation failure, concurrent writer and linked-service cases are verification work in t002/t003/t007, not claims of observed production failures.

## Source + Goal linkage

- **Source:** continuous functional qa-find-bugs a385, 2026-10-13 UTC, user-directed w4 placement and muse.env login via qa-login.sh. [Finding](finding.md) and [complete durable requests/responses](evidence.json). No product fix is included in this filing.
- **Goal linkage:** ADR008 reliable Render-alternative hosting; ADR006 consistent API outcomes; ADR013 existing group storage/compensation; ADR018 project/environment and group workflows.
- **Expected outcome:** small valid create/move requests finish promptly, failed creates cannot trap an undiscoverable name, and move retries start from truthful placement.
- **Why now:** two scoped-create attempts timed out, subsequent retries refused the same undiscoverable name for almost an hour, and desktop/mobile two-group moves both timed out with different persisted outcomes. Users cannot infer whether retry is safe.
- **Sizing:** 255m across bounded lookup work, creation recovery, membership outcome/UI reconciliation, a dedicated shared-caller audit and standing closing tasks. These are separate write-lifecycle gaps; reducing lookup work alone cannot guarantee recovery.
- **Render parity included:** REST, GraphQL, MCP and dashboard share these customer workflows. Render documents create/list and environment moves; authenticated Render timeout behavior was not measured.
- **Cleanup:** five visible fixtures deleted and QA session revoked. **Residual:** failed name `qa-20261013-a385-first` still returned ENV_GROUP_NAME_EXISTS at 22:07:02.632Z, without a group ID. No administrative repair was attempted.
