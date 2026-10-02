# w4 · m148 — Track the complete first Blueprint deployment with environment groups

**Worker:** worker4 **Goal:** a new Blueprint service's returned first deploy represents its complete initial configuration and reaches Live when that configuration serves. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Compose initial group links before dispatching the first release | 50m | — |
| t002 | Preserve shared creation, authorization and existing group-update behavior | 40m | t001 |
| t003 | Render parity | 20m | t002 |
| t004 | Simplify | 10m | t003 |
| t005 | Test coverage | 40m | t003 |
| t006 | Closeout | 15m | t004, t005 |

## Definition of done

Repeat [the captured live probes](finding.md) with new disposable free Go web services, `autoDeployTrigger: off`, and the same hello-go source.

- Submit the first manifest once, creating a group and a service using `fromGroup`. The returned `latestDeployId` tracks the complete initial configuration: it finishes Live when the public URL returns the group's `MESSAGE`. Fresh dashboard header/Deploys, GraphQL, REST and MCP show that same Live deploy; initialization alone does not leave its only row Canceled or publish a second untracked release.
- Submit the second manifest once, creating another service that references the existing group. It has the same result without a manual deploy, cancel, settings edit or re-apply. Observe the App and build Jobs: initial group attachment must not dispatch a build of a partial configuration followed by an untracked replacement build.
- The literal-value service created beside the first grouped service retains its working behavior: one initial Live deploy, the literal response body, and a matching Running service. Check the public URLs from the browser and with the captured external curl commands.
- Delete the three services and group. Their REST reads return 404, the fresh dashboard omits them, their App/workload/build resources disappear, and the QA session is revoked.

Only native Go web creation was exercised live. Other service types, auto-deploy-on, connected Blueprint create/sync, authorization failures and other initial environment forms are verification work in t002/t003/t005, not claims of observed failures.

## Source + Goal linkage

- **Source:** continuous qa-find-bugs, 2026-10-02 sweep 8, muse.env login, user-directed w4 placement. [Finding](finding.md) carries complete requests/responses, two independent creations, the literal control, source/dependency trace and the prior-DoD audit.
- **Goal linkage:** ADR008 reliable hosting and deploy-from-chat; ADR049's one Blueprint apply contract; ADR004/ADR006's attributable release/deploy lifecycle.
- **Expected outcome:** following a new Blueprint service's first deploy gives a truthful completion result for the configuration the user submitted.
- **Why now:** both grouped services built twice and served the intended value at rev-2, while every probed deploy API and a fresh dashboard showed only a canceled first deploy. The literal control reached Live at rev-1. Automation following the returned deploy ID receives a false cancellation.
- **Sizing:** 175m across initial composition, the authorized group-link seam and its shared callers, lifecycle regressions and closing tasks. This crosses the API/store/operator boundary; a badge change or a new annotation alone cannot restore the contract.
- **Render parity included:** tenant-facing Blueprint and deploy surfaces change. Render documents fromGroup and group-link deploy behavior; authenticated Render first-create history was not measured. Preserve Bex's existing auto-deploy-off group-update policy while fixing initial composition.

## Dedupe and limits

This is a regression of **w6/m46's first-deploy guarantee**, through Blueprint group initialization. Its release-generation stamp is present both in production and on main; the missing case is a later initial-spec patch after the operator has adopted generation 1. The finding walks every original DoD item.

The 37 open/blocked milestone READMEs and open/completed notes were scanned. w8/m46 covers competing user deploy triggers; this has one create request and no replacement deploy row. w4/m146 covers repeated empty-env applies; these are first applies with a nonempty group. w4/m147 covers an open Events page's refresh; fresh pages and all three APIs reproduce this persisted result. w2/m94's intentional gate remains a regression constraint. No matching fix is waiting on deployment. This filing changes no product code.
