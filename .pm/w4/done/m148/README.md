# w4 · m148 — Track the complete first Blueprint deployment with environment groups

**Worker:** worker4 **Goal:** a new Blueprint service's returned first deploy represents its complete initial configuration and reaches Live when that configuration serves. **Status:** done 2026-10-02 — all tasks complete; hosted first-create replay passed

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Compose initial group links before dispatching the first release — **DONE** | 50m | — |
| t002 | Preserve shared creation, authorization and existing group-update behavior — **DONE** | 40m | t001 |
| t003 | Render parity — **DONE** | 20m | t002 |
| t004 | Simplify — **DONE** | 10m | t003 |
| t005 | Test coverage — **DONE** | 40m | t003 |
| t006 | Closeout — **DONE** | 15m | t004, t005 |

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

## Parked gate — 2026-10-02

The release pipeline must deploy the backend and migration 0137. QA must then
repeat both native Go grouped creates and the literal control, capture one
complete first build/deploy per grouped service, verify external MESSAGE values
and the same Live deploy through REST/GraphQL/MCP/dashboard, then delete the
three services/group and revoke the QA session. No hosted fixtures were created
in this implementation run. [Verification](verification.md) distinguishes
local/source coverage from the remaining production and connected-Git probes.

## Live acceptance — 2026-10-02

Production carried backend fix `c7773a005` (ancestor of deployed `18958459c`). Manifests are the finding's, renamed `qa-20261002-w4x-*`, submitted once each to `POST /v1/blueprints/deploy`. Times UTC 2026-10-03.

- **First manifest:** created group `qa-20261002-w4x-group` (`evg-db0abe8ehcmc739j13tg`, `MESSAGE=qa-w4x-group`), `qa-20261002-w4x-groupweb` (`srv-db0abfoehcmc739j13u0`, `fromGroup`) and `qa-20261002-w4x-literal` (`srv-db0abh0ehcmc739j13vg`). Returned `latestDeployId`s `dep-db0abfoehcmc739j13ug` and `dep-db0abh0ehcmc739j1400` are each service's only deploy and reached `live` (06:54:24, 06:55:54). External curl and a browser-context request return HTTP 200 `qa-w4x-group` and `qa-w4x-literal`.
- **Second manifest:** `qa-20261002-w4x-groupweb2` (`srv-db0abk2tm2ss7389qo8g`) referencing the existing group; returned `dep-db0abk2tm2ss7389qo90` is its only deploy, `live` 06:56:54, HTTP 200 `qa-w4x-group`. No manual deploy, cancel, settings edit or re-apply.
- **Parity:** for each service, REST by-id, GraphQL `deploys`/`server` (phase Running, rev-1), MCP `get_deploy`/`list_deploys`, and the fresh dashboard header ("Service Running · Latest deploy Live") and Deploys list (one Live "First Deploy" row) show the same deploy. No Canceled row.
- **Build Jobs (read-only kubectl):** exactly one build Job per service, all `gen-1` (`bld-…-w4x-groupweb-gen-1`, `…-w4x-literal-gen-1`, `…-w4x-groupweb2-gen-1`, each Complete 1/1); no partial-configuration build followed by a replacement.
- **Not exercised live:** connected Blueprint create/sync and `render.yaml`/legacy `bex.yml` filename aliases (need a Git-hosted qa manifest); they remain verified by t002/t005 tests.
- **Cleanup:** three services and the group deleted by exact ID; REST GET → 404 for each; the fresh dashboard overview and `/env-groups` list no `w4x` resources; no App, workload or build Job remains (read-only kubectl). QA session revoked at the end of the run.
