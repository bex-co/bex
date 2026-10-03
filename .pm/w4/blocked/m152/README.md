# w4 · m152 — Keep relative hosting times current on open pages

**Worker:** worker4 **Goal:** elapsed ages and cron countdowns track the client clock while an otherwise unchanged page remains open. **Status:** blocked — t001/t002/t004/t005 done; t003/t006 await dashboard release and deployed clock samples.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 — **DONE** | Refresh compact relative times from a bounded shared clock | 30m | — |
| t002 — **DONE** | Verify all shared callers and the advancing deploy control | 30m | w4/m152/t001 |
| t003 | Render parity for time presentation and API instants | 15m | w4/m152/t001, w4/m152/t002 |
| t004 — **DONE** | Simplify the milestone changes | 10m | w4/m152/t003 |
| t005 — **DONE** | Test unchanged data, clock boundaries and hydration | 30m | w4/m152/t003, w4/m152/t004 |
| t006 | Closeout after live acceptance | 10m | w4/m152/t005 |

Total: **6 tasks, about 2h 5m**. This filing schedules the fix; product code is unchanged.

## Definition of done

Repeat the free cron fixture and timestamp sampler in [finding.md](finding.md). These criteria are drawn from the two stable-page reproductions; the other shared consumers are verification work in t002/t005.

- After a successful scheduled run, freshly open its Deploys page and leave the visible tab untouched for at least two minutes, ending before the next run. With unchanged `createdAt`, `lastSuccessfulRunAt` and `nextRunAt`, the header's Created and Last run ages advance and Next run counts down from the current client clock. Each clock sample is at most 60 seconds old; no reload, click, mutation or changed API result is needed.
- Repeat from another fresh page. Compare the displayed buckets against each element's unchanged `datetime` and the sampled clock. Reloading does not correct labels that have remained stuck beyond that 60-second allowance.
- Keep the existing deploy-row control passing: its relative elapsed text advances on the same page while the exact deployment instant and row remain unchanged. Background polls keep the populated page rendered.
- Replay the recorded GraphQL, REST and MCP reads: last-success, next-run and created instants retain their existing semantics. Scheduled runs still succeed and their runtime logs contain the configured marker; the presentation fix does not create jobs or alter their schedule.
- Delete only the verification cron fixture, confirm API 404/list removal and absence of its exact App/CronJob/Job/Pod/Secret artifacts, then revoke that QA session.

## Source + Goal linkage

- **Source:** continuous `$qa-find-bugs`, production sweep 14 on 2026-10-02, using `muse.env` and filing in w4 by user request. [Finding](finding.md) includes both fresh-page reproductions, complete API probes, pinned dependency evidence and caller inventory.
- **Goal linkage:** ADR008 usable hosting; [ADR038](../../../../docs/ADR038-cron-jobs.md) honest cron run/schedule information; [ADR018](../../../../docs/ADR018-render-parity.md) last-success/next-run presentation; [ADR006](../../../../docs/ADR006-bex-api.md) shared API semantics.
- **Expected outcome:** an operator can read a recent-run age or upcoming-run countdown without manually refreshing an otherwise healthy page.
- **Why now:** the current header remained “now / in 4m / 4m” across minute boundaries while the same page's deploy timer advanced. Correct API timestamps isolate the defect to the shared presentation component.
- **Render parity included:** tenant-visible presentation changes. Render documents UTC schedules; exact authenticated Render countdown cadence was not observed. `nextRunAt` remains the existing bex extension.
- **Dedupe:** an uncovered clock-input gap beside w6/done/m102's hydration guard and w9/done/056's last/next fields; no proof that either old fix regressed. Open w4/m147 and m151 concern query-window freshness and log paging, respectively. No open equivalent, anti-goal match or undeployed clock fix found.
- **Limits:** cron header clock behavior was reproduced live twice; the 27 shared JSX sites were source-audited, not all live-probed. Mobile, hidden-tab resume, other resource families and hydration boundary checks remain assigned verification. The hunt's fixture cleanup and session revocation are complete.

## Implementation verdict — 2026-10-02

Shared clock, accessible-label synchronization, hydration/lifecycle regressions and local caller checks are complete. [Verification and remaining release/QA gate](verification.md). No live fixtures or sessions were created.

## Independent supplemental verification

The parallel drain retained shared-clock implementation `79910e98c` and added real cron-header and SessionRow hydration regressions. [Supplemental evidence](supplemental-verification.md) distinguishes the independent runs from merged verification.
