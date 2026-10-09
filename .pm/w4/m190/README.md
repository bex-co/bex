# w4 · m190 — Send service hostnames in HTTP health probes

**Worker:** worker4 **Goal:** a hostname-restricted web app completes its HTTP-health deployment while probes still test the exact pod and release history stays honest. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define custom-first probe Host selection and release retention | 40m | — |
| t002 | Project and retain the selected Host across the three probes | 60m | t001 |
| t003 | Audit shared consumers and preserve sibling probe behavior | 45m | t002 |
| t004 | Verify Render and cross-surface health contracts | 30m | t003 |
| t005 | Simplify the changed code | 20m | t004 |
| t006 | Test Host selection and release-history safety | 45m | t004 |
| t007 | Replay the live fixture, clean up and close out | 30m | t005, t006 |

## Definition of done

- Create an owned Free BusyBox web service with [the finding's CGI](finding.md). Settings → set `/cgi-bin/health` → save → wait. The new deploy becomes Live; its instance logs the actual service hostname rather than a pod IP, and public `/cgi-bin/health` remains 200 with `QA_A32_HOST_OK`. Fresh-load and repeat the TCP → HTTP transition.
- Clear the path through the same UI and wait for Live, then set it again and wait for Live. A query-string path `/healthz?check=a32` and literal-space path `/qa health` still become Live. A padded query-path no-op remains normalized with no new deploy.
- While an HTTP deployment is pending, the prior healthy release keeps serving the marker; the list/detail/API readers converge on its real terminal status. REST get_deploy, GraphQL deploys and MCP get_deploy keep their existing status/trigger contracts.
- Delete every owned fixture, verify API/public absence and baseline resource ID sets, and revoke only this QA session. Closeout requires dated live evidence after the fix is deployed.

## Source + Goal linkage

- **Source:** live qa-find-bugs cycle32, 2026-10-09 UTC, muse credentials; user requested w4. [Researched finding and complete probes](finding.md); local ignored qa-health-host-a32 ledger/screenshot. Observed blocker; filing only.
- **Goal linkage:** ADR008 usable Render-alternative hosting, ADR004 healthy zero-downtime deployment and ADR005 verified domain intent.
- **Expected outcome:** hostname validation accepts pod health checks, so a publicly healthy web app can finish the deploy without abandoning its HTTP application-health check.
- **Why now:** the live pod receives a different Host from public requests; normal HTTP success is insufficient to make the release Ready. The shared handler reaches startup/readiness/liveness, and a naive header fix would also change templates on operational domain edits.
- **Scope:** one observed defect, 7 tasks, 4h 30m. Host projection is allowlisted to web HTTP probes with an admitted public hostname. Retain that selection within a release, including absent legacy Host, to avoid hidden/migration rollouts. New releases select from current domain intent; cancel restores the prior served template.
- **Render parity included:** the user-visible health contract is affected across every create/update adapter and deployment reader, despite no new API field. Record reselection timing and private HTTP as bounded Bex behavior; do not invent Render observations.
- **Unverified:** custom-domain/subdomain cases, private/worker/cron/static/datastore controls, post-fix rollback/Restart/cancel, steady-state health failure, mobile/dark, foreign-resource refusals and live Render. These are test obligations, not live DoD observations.
