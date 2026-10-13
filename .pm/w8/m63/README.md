# w8 · m63 — Keep completed initial hooks out of later deploys

**Worker:** worker8 **Goal:** a completed one-time Blueprint hook cannot be selected again by a later native manual deployment; regular pre-deploy commands keep their intended per-deploy behavior. **Status:** todo

## Tasks (in order)

| id   | title                                            | est | depends_on |
| ---- | ------------------------------------------------ | --- | ---------- |
| t001 | Preserve regular-command and initial-hook roles  | 40m | —          |
| t002 | Complete initial hooks at the lifecycle boundary | 45m | t001       |
| t003 | Render parity                                    | 30m | t002       |
| t004 | Simplify                                         | 15m | t003       |
| t005 | Test coverage                                    | 35m | t003, t004 |
| t006 | Closeout                                         | 15m | t005       |

## Definition of done

An owned local or otherwise authorized Free web fixture with distinct harmless initial/regular markers reaches its first Live deploy. Without another Blueprint sync, published/current Bex and equivalent pinned Render native `deploys create <owned-id> --confirm -o json --wait` cause no new initial-hook execution and do run the intended regular command. A second later deploy and a Blueprint re-sync retain those counts; empty regular command causes no regular job, and equal initial/regular command text retains the per-deploy regular role. Pending/failed hook retries, legacy state migration and saved/history configuration neighbors are tested without losing regular command intent. Actual Job and runtime marker receipts are required before closeout; source/fake-client evidence alone does not close this milestone. Preserve upstream request/response shape, all supported callers and whole w2/m45 guarantees. Delete exact owned fixture/children, confirm detail/list/runtime/quota teardown and baseline preservation. All implementation/standing tasks are done only when this observable state holds.

## Source + Goal linkage

- **Source:** functional CLI sweep206, 2026-10-13 UTC. [Researched finding](finding.md), [native production admission plus current local native/API/operator proof](evidence/cli-sweep206/README.md). The first Live/hook completion is modeled; hosted execution is untested. Addresses the remaining no-rerun guarantee in [w2/done/m45](../../w2/done/m45/README.md).
- **Goal linkage:** ADR008 reliable hosting and ADR049 Blueprint intent: initialization stays one-time while normal deployment keeps regular commands.
- **Expected outcome:** native manual deploy after completed initialization selects the regular command, independently of whether the Blueprint was re-synced.
- **Why now:** both pinned native clients drive the current handler into a new release that the actual current operator schedules with the old initial command; re-sync controls isolate the missing lifecycle completion transition.
- **Sizing:** two implementation tasks plus required closing work, 3h total. Persisted roles, backend/operator completion and legacy/sibling validation exceed a sub-hour inbox fix.
- **Render parity:** included because command selection and field intent are tenant-facing across API, Blueprint and dashboard deployment flows. Record the existing pre-deploy-mechanism timing difference honestly; do not silently claim exact post-deploy timing or expand product scope.

No implementation or ship authorization is supplied by this QA filing. Live revisions remain unknown; no production fixture is created by sweep206.
