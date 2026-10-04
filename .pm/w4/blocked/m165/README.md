# w4 · m165 — Loop1: workspace-switch navigation + identical-cancel undeployed flag

**Worker:** worker4 **Goal:** workspace switch never shows cross-workspace data; cancel without changes never raises the saved-changes flag **Status:** blocked — t001–t005 done 2026-10-04; t006 live closeout remains

## Tasks (in order)

| id   | title                                                     | est | depends_on    |
| ---- | --------------------------------------------------------- | --- | ------------- |
| t001 — **DONE** | Dashboard workspace switch navigates to overview          | 40m | —             |
| t002 — **DONE** | Identical-config cancel leaves undeployedChanges false    | 50m | —             |
| t003 — **DONE** | Render parity across read surfaces                        | 30m | w4/m165/t002  |
| t004 — **DONE** | Simplify changed code                                     | 20m | w4/m165/t003  |
| t005 — **DONE** | Test coverage for shipped behavior                        | 30m | w4/m165/t003  |
| t006 | Closeout                                                  | 15m | w4/m165/t005  |

## Definition of done

- Workspace switch: from a bex /project/:id page switch to tian-personal via the switcher — URL becomes / showing the tian-personal overview with its own projects/resources, no bex project content anywhere, breadcrumbs show workspace names never a prj- ID; repeat bex -> bex-canary from a /services/:id page with the same result.
- Identical cancel: on a Live web service with no pending changes (no banner), Manual Deploy latest commit (same SHA) then Cancel+Proceed — service header shows no Saved-changes banner, GraphQL service{undeployedChanges revision} returns false with revision unchanged, REST /v1/services/:id agrees; curl of the public URL still serves the Live revision body.
- Control preserved: save a real env var without deploying — banner appears with undeployedChanges true on GraphQL+REST; next successful deploy clears both. Config-change deploy canceled mid-build still reports true until the next successful deploy (w1/m152 intact).

## Source + Goal linkage

- **Source:** infinite /qa-find-bugs loop1 2026-10-04 UTC (muse.env auth; journeys 1-4,6-8; journeys 5,9-15 deferred to loop2), researched at HEAD 0269ab5fc. Evidence: .playwright-mcp/qa-workspace-switch-stale-1.png (stale cross-workspace view after switch), .playwright-mcp/qa-cancel-undeployed-banner-1.png plus GraphQL+REST probes showing undeployedChanges true with revision unchanged after identical cancel (reproduced twice). Surfaces belong to ADR004 (deploys report truthful state) and dashboard navigation.
- **Goal linkage:** ADR008 reliable Render-alternative hosting + truthful service/deploy state.
- **Expected outcome:** switching workspace via the dashboard switcher always lands on the new workspace overview, never a stale cross-workspace resource view; canceling a deploy that carries no configuration change leaves undeployedChanges false with no Saved-changes banner.
- **Why now:** loop1 findings mislead tenants (stale tenant view shows another workspace's data; false-positive deploy prompt after a no-op cancel); t002 closes the identical-config gap left by w1/m152 (done, config-change cancel=true). Render parity is INCLUDED because t002 changes operator status consumed by REST/GraphQL/MCP/UI, which must agree; t001 is UI-only with no API change.
