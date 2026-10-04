# w4 · m143 — Initialize service-page environment-group scope on the first controlled opening

**Worker:** worker4 **Goal:** an environment-scoped service creates a group in its own environment on the first dialog opening after a cold page load. **Status:** done 2026-10-02 — all tasks complete; production UI acceptance passed.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Initialize scope and selection together for a controlled open — **DONE** | 40m | — |
| t002 | Verify both dialog consumers and service-route aliases — **DONE** | 30m | t001 |
| t003 | Render parity — **DONE** | 20m | t002 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Test coverage — **DONE** | 30m | t003, t004 |
| t006 | Closeout — **DONE** | 10m | t005 |

## Definition of done

- Hard-load an environment-scoped QA service's Environment tab, then use the top **Create group** action. The first opening shows the service's actual environment and its checked, visible service; creating a named group succeeds in that scope. Repeat through the panel's **Create group** action after a fresh load. No cancel/reopen workaround is required.
- Cancel/reopen preserves correct defaults; the same first-open journey on a workspace-scoped QA service still creates a workspace group linked to that service.
- From `/env-groups`, Workspace lists the workspace-scoped control and excludes the environment-scoped control; selecting the QA environment lists its service and allows successful creation. These existing working branches remain usable.
- A deliberate mismatched GraphQL create still returns `ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH` without creating a group; equivalent REST POST still returns 409. Delete this drill's groups/services/project/environment and revoke its session.

These clicks and API probes were run in this hunt. MCP and other service types remain explicit verification work in t002/t003, not observed results.

## Source + Goal linkage

- **Source:** live `$qa-find-bugs` 2026-10-02, `muse.env` account, sweep r1; [research and full API evidence](finding.md). Verified local artifacts: `.playwright-mcp/qa-env-r1-1.png`, `.playwright-mcp/qa-env-r1-1.json` (gitignored).
- **Goal linkage:** ADR008 hosting usability, [ADR032](../../../../docs/ADR032-environments.md) scope semantics and [ADR013](../../../../docs/ADR013-secrets.md) shared configuration.
- **Expected outcome:** the first attempt to share configuration succeeds with a compatible scope and visible selection, without silently retaining a hidden incompatible service ID.
- **Why now:** this is the missed cold-load acceptance case from [w4/m111](../../done/m111/README.md), whose live re-probe was deferred. The same backend correctly rejects the malformed first request and accepts the post-cancel request; weakening backend validation would hide the UI bug.
- **Render parity:** included because the shared create form builds GraphQL mutation inputs. Render does have environments; its workspace-group linking rule differs from bex's exact-scope rule. t003 must correct the old m111 parity assumption and record the divergence without broadening this fix.

## Outcome — 2026-10-02

The cold-open lifecycle fix, caller regressions, parity correction and simplify pass are complete. [Verification](verification.md) records the original-code failures, full dashboard checks, 12 successful live API expectations, and cleanup. t002–t005 are done. **BLOCKED:** the production-deploy pipeline must release the dashboard, then QA must repeat the first-open toolbar/panel, reopen, workspace-service, and list-page click matrix in the DoD. t001 retains that live criterion and t006 remains open. No QA resources or live session from this replay remain.

## Live acceptance — 2026-10-02

Production dashboard carried fix `10a262596` (ancestor of deployed `18958459c`). Workspace `tea-d98210cbbpdc73dcrkvg`; fixtures: project `qa-20261002-w4x-prj` (`prj-db09r8itm2ss7389qmcg`) with environment `qa-20261002-w4x-stage` (`evm-db09r8itm2ss7389qmd0`), env-scoped service `qa-20261002-w4x-envsvc` (`srv-db09rdgehcmc739j11p0`) and workspace-scoped `qa-20261002-w4x-wssvc` (`srv-db09rdqtm2ss7389qmeg`). Times UTC 2026-10-03 06:16–06:19.

- **Toolbar, first open after hard load:** dialog briefly shows "Loading environments and services…" with Create disabled, then Environment `qa-20261002-w4x-stage` and the service checkbox checked. `qa-20261002-w4x-grp-top` created → GraphQL `environmentId=evm-db09r8itm2ss7389qmd0`, `serviceLinks=[srv-db09rdgehcmc739j11p0]`.
- **Panel, first open after a fresh load:** same environment + checked service; `qa-20261002-w4x-grp-panel` created in the environment, linked. No cancel/reopen needed.
- **Reopen:** the next open and a cancel/reopen both kept `qa-20261002-w4x-stage` + checked service.
- **Workspace service first open:** "Workspace (no Environment)" with the current service checked; `qa-20261002-w4x-grp-ws` created with `environmentId=null`, linked to `srv-db09rdqtm2ss7389qmeg`.
- **`/env-groups`:** Workspace candidates list `qa-20261002-w4x-wssvc` (and other workspace services) and exclude the env-scoped service; selecting `qa-20261002-w4x-stage` lists only `qa-20261002-w4x-envsvc`; checking it created `qa-20261002-w4x-grp-list` in that environment.
- **Negatives:** GraphQL `createEnvGroup(environmentId:null, serviceIds:[envsvc])` → `ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH`, `createEnvGroup: null`; REST `POST /v1/env-groups` equivalent → HTTP 409 with the same code; neither group exists afterwards.
- **Cleanup:** four groups, two services and the project deleted by exact ID; REST GET of each group/service/project/environment → 404; no `w4x` App CR remains (read-only kubectl). QA session revoked at the end of the run.
