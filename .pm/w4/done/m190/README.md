# w4 · m190 — Send service hostnames in HTTP health probes

**Worker:** worker4 **Goal:** a hostname-restricted web app completes its HTTP-health deployment while probes still test the exact pod and release history stays honest. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define custom-first probe Host selection and release retention — **DONE** | 40m | — |
| t002 | Project and retain the selected Host across the three probes — **DONE** | 60m | t001 |
| t003 | Audit shared consumers and preserve sibling probe behavior — **DONE** | 45m | t002 |
| t004 | Verify Render and cross-surface health contracts — **DONE** | 30m | t003 |
| t005 | Simplify the changed code — **DONE** | 20m | t004 |
| t006 | Test Host selection and release-history safety — **DONE** | 45m | t004 |
| t007 | Replay the live fixture, clean up and close out — **DONE** | 30m | t005, t006 |

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

## Closeout — 2026-10-11

Shipped `014810e45` (`fix(operator): send the service hostname as Host on HTTP health probes`); deploy.yml run 38098734469 succeeded and images pinned in `c91dd608a`. HTTP probes keep the pod-IP connection but send Host = verified custom domain (`spec.host`, then `spec.hosts`, redirect sources skipped) else the canonical platform host; private/worker/TCP unchanged. The Host is retained per release from the live template (same `revision` label), so domain-only edits and the operator upgrade do not roll pods; new release / Restart / rollback select current domains; cancel/failed-rollout restore stays verbatim. Tests: `probe_host_test.go` (selection table, kubelet-style 200-vs-421 request against a hostname-restricted handler, per-release freeze incl. legacy absence, rollback current-host, cancel restore) + envtest in `app_controller_test.go`; `make test` and `make lint` green; mutation check (disabling retention) fails the freeze tests. ADR004 § Probe Host and ADR018 health row record the rule and that Render's reselection timing is undocumented/unverified (Render docs rechecked 2026-10-10).

Live replay (production, QA session via qa-login.sh, mutations sent as the dashboard's exact `SetHealthCheckPath` GraphQL operation over the session rather than by clicking Settings): owned Free BusyBox `qa-20261011-m190-host` (`srv-db5e5k99siac73f4s9mg`) with the finding's CGI allowlisting `qa-20261011-m190-host.onbex.co`.

- TCP → `/cgi-bin/health`: `dep-db5e5td6p5vc738lflf0` started 01:14:31Z, **live** 01:14:45Z (rev-3). New instance `…-u13s96mrq6f4gsu34ef5` logged `QA_M190_PROBE_HOST=qa-20261011-m190-host.onbex.co` every 10s (01:14:39–01:15:00Z), no pod-IP Host. Public `/cgi-bin/health` 200 `QA_A32_HOST_OK`.
- Clear → live `dep-db5e6756p5vc738lflg0` (rev-4); re-set `/cgi-bin/health` → live `dep-db5e6ep9siac73f4s9og` 01:15:55Z (rev-5); `/healthz?check=a32` → live `dep-db5e6mt6p5vc738lflhg` (rev-6); `/qa health` → live `dep-db5e6ut6p5vc738lflig` (rev-7). Public CGI 200 after every step. Padded `"  /qa health  "` normalized to `/qa health` with no new deploy (rev-7 stayed latest).
- REST `GET /v1/services/…/deploys/dep-db5e6ep9siac73f4s9og` and MCP `get_deploy` returned the same `status`/`trigger: service_updated`/`bexTrigger: config_change` contract (by then `deactivated`, superseded).
- Cleanup: service deleted ≈01:17:49Z; GraphQL `service` NOT_FOUND and public 404 at 01:18:34Z. Envgroups/KV/projects/blueprints unchanged; service/database set deltas (`srv-db5e7et6p5vc738lflkg` added, `dpg-db5e3o6m60vc7392pseg` removed) belong to concurrent sessions, not this run. `qa-login.sh --logout` → `ok logged-out`; jar then 401; local session files removed.
- Not replayed live: a long pending HTTP deploy over a prior release (each deploy went Live in ~15s), custom-domain selection, cancel/rollback/Restart — covered by unit/envtest above.
