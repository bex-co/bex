# w4 · m206 — Make environment-group creation and moves recoverable

**Worker:** worker4 **Goal:** ordinary scoped group creation, cloning and a two-group move complete within the existing API budget, and an interrupted write has a discoverable, recoverable outcome. **Status:** blocked for final live closeout — t001–t007 done; t009 is actionable follow-up on the remaining create/clone admission cost, then release + t008 replay and name-residue confirmation remain.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Bound scoped creation and move work without full environment hydration — **DONE** | 40m | — |
| t002 | Recover failed creation and its owned name reservation — **DONE** | 50m | t001 |
| t003 | Reconcile interrupted membership moves and source selection — **DONE** | 45m | t001, t002 |
| t004 | Audit shared callers, aliases and passing controls — **DONE** | 30m | t003 |
| t009 | Keep small native group clones within the existing response budget | 60m | t001, t002 |
| t005 | Render parity — **DONE** | 15m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 40m | t005, t006 |
| t008 | Closeout | 20m | t007, t009 |

## Definition of done

Repeat [the actual native journeys](finding.md) with fresh owned qa-prefixed empty, unlinked groups and a disposable project with Left and Right environments:

- From a fresh dashboard, create one empty group directly in Left. One Create click returns its ID within the existing GraphQL budget and the group appears once in the complete owner list and Left membership. Workspace-scope creation and a single Manage → Move group remain passing controls.
- After an interrupted scoped create, read the complete owner group list before retrying the same name. The result is either one discoverable committed group or a recoverable failed attempt that permits one subsequent creation; it cannot remain an invisible duplicate-name conflict. Recover the exact residual `qa-20261013-a385-first` only after proving the original attempt is no longer active and the reservation's owner/outcome. Record its disposition.
- Put two unlinked groups in Left. Select the first, search for the second, select all visible resources, then clear the search: both remain selected. Move them together to Right. A settled successful response and ready page show Left empty, Right containing both IDs, and no selected source resources. Repeat from a fresh 390px page.
- If the move times out, the client reads the authoritative source and target before offering another write. It shows the actual complete or partial outcome, and selection contains only groups still in the canonical source membership. The recorded desktop case must never show one source row with “Selected: 2.” Preserve selected groups hidden only by search.
- Delete each visible fixture by exact ID; its REST read returns 404 and all seven starting inventory ID sets are restored. Record any failed-name recovery residue separately from visible cleanup. Revoke only this QA session.

From [a633](evidence-a633-clone.json): clone the recorded small group (two variables and two files, including empty entries) from fresh desktop and 390px pages. One click returns a new ID within the existing 25s GraphQL budget; the clone appears exactly once, preserves the exact contents and copies no service links. An independent clone edit leaves the source service unchanged, and linking the edited clone delivers its values/files. The two native attempts returned WRITE_OUTCOME_UNKNOWN; REST cloning took24.772s and an empty-create recovery took24.035s. t009 owns the remaining admission work and repeats the scoped parity/simplify/regression checks before t008. Production stage timings remain unmeasured.

Injected late-acknowledgment, compensation failure and concurrent writer cases remain verification work, not claims of observed production failures.

## Source + Goal linkage

- **Source:** continuous functional qa-find-bugs a385, 2026-10-13 UTC, user-directed w4 placement and muse.env login via qa-login.sh. [Finding](finding.md) and [complete durable requests/responses](evidence.json). No product fix is included in this filing.
- **Goal linkage:** ADR008 reliable Render-alternative hosting; ADR006 consistent API outcomes; ADR013 existing group storage/compensation; ADR018 project/environment and group workflows.
- **Expected outcome:** small valid create/move requests finish promptly, failed creates cannot trap an undiscoverable name, and move retries start from truthful placement.
- **Why now:** two scoped-create attempts timed out, subsequent retries refused the same undiscoverable name for almost an hour, and desktop/mobile two-group moves both timed out with different persisted outcomes. Users cannot infer whether retry is safe.
- **Sizing:** 255m across bounded lookup work, creation recovery, membership outcome/UI reconciliation, a dedicated shared-caller audit and standing closing tasks. These are separate write-lifecycle gaps; reducing lookup work alone cannot guarantee recovery.
- **Render parity included:** REST, GraphQL, MCP and dashboard share these customer workflows. Render documents create/list and environment moves; authenticated Render timeout behavior was not measured.
- **Cleanup:** five visible fixtures deleted and QA session revoked. **Residual:** failed name `qa-20261013-a385-first` still returned ENV_GROUP_NAME_EXISTS at 22:07:02.632Z, without a group ID. No administrative repair was attempted.

## Additional live evidence — a633, 2026-10-15

[Full normalized mutation requests/responses, controls, probe and cleanup](evidence-a633-clone.json). The existing t001 note leaves name-sweep cost unresolved. Current source runs independent quota/name metadata sweeps; the local scoped probe shows7/87/407 metadata Gets for0/20/100 retired groups with one active source. This establishes read growth, while the production contribution of each stage is unprofiled. Extend this milestone instead of filing a duplicate of its shared create/clone admission work. All four known a633 group IDs and the owned service return404, seven baseline inventories match, and the exact QA session is revoked. The old a385 residual was not touched. Source unchanged; this ships the filing only.
