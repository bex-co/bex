# Relative hosting clocks — local verification

## Implemented behavior

`useNow` owns one shared browser clock per cadence (one minute by default), independent of rendered row count.
The retained implementation samples the wall clock immediately on visibility resume and removes each cadence timer and listener after its final subscriber unmounts. Hidden tabs rely on browser timer throttling; their intervals are not explicitly paused. Server rendering uses a render-local instant; no server timer or
request-specific module state is created. Existing compact formatters, exact
`datetime`, title, custom fallback and span behavior remain unchanged.

`RelativeAge` and `RelativeUntil` subscribe to that clock. SessionRow also uses
the same sample for its accessible revoke label. The existing deploy-list caller
shares the timer; no API polling, query loading policy, timestamp projection,
cron scheduling or layout was changed.

## Reproduction and passing control

A mounted real ServiceDetailHeader with fixed cron facts failed before the fix:
Created stayed `4m` when the clock required `5m`. After the fix, two minute ticks
produce Created `4m → 5m → 6m`, Last run `now → 1m → 2m`, Next run
`in 4m → in 3m → in 2m`, without rerendering props or invoking deploy/restart.
All exact DOM instants remain unchanged. The existing deploy row independently
advances through two minutes while retaining the same DOM row, timestamp and
unchanged query object. Both caller suites passed (52 tests).

## Evidence boundary and remaining acceptance

These are controlled local mounted-component and API tests, not a deployed
browser replay. The release pipeline must deploy the dashboard. QA then runs
the two fresh, visible cron pages for at least two minutes before the next
scheduled run, compares wall clock and unchanged DOM instants, and verifies
scheduler/log and REST/GraphQL/MCP controls. Desktop and narrow-mobile hosted
checks and the named sibling routes remain part of that replay. Delete only
the owned fixture, verify exact workload/API absence, and revoke that session.
No live fixture or QA session was created in this implementation run.

Render documents UTC schedules, not an authenticated dashboard countdown refresh
cadence. The 60-second clock is bex's existing display policy; `nextRunAt` remains
a bex extension. No exact Render UI cadence claim is made.

## Shared consumers, hydration and checks

The refreshed source inventory retains 27 JSX sites across 18 files (26 Age,
1 Until), plus SessionRow's accessible-label formatter. The finding's complete
consumer table remains the inventory: service headers/recent runs, projects,
Postgres metadata/recovery, Key Value metadata, events/Blueprints, webhooks,
API/registry/SSH keys, connected agents and sessions. This shared-component
change reaches every site; they were not all independently timed in a browser.
The service/static compositions and web/worker/pserv/cron/d/r alias mappings
were source-audited; alias-helper tests passed, not a fresh browser route walk.

SessionRow renders a stable exact ISO instant (or existing missing fallback) in
its accessible label during SSR/initial hydration, then uses the shared relative
sample. Only its time leaf is replaced once after hydration: suppressed server
text otherwise survives until another bucket change. Tests cross a minute
boundary with no console attribute warning or recoverable hydration error,
assert immediate visible/accessible agreement, and then the next clock tick.
No row/button/dialog remount or session mutation occurs.

Clock tests cover 51 subscribers sharing one timer, late joins preserving
cadence, final unmount/remount, hidden pause and immediate visible resume,
delayed callbacks sampling real wall time, StrictMode cleanup, and per-render
SSR sampling without a browser timer. Mounted wrappers retain exact datetime,
title, span/fallback behavior and unchanged props across two minute ticks.
Existing hydration negative controls still demonstrate a bare formatter failure.

The complete dashboard suite passed 464 files/3,990 tests (83.97s), with
typecheck/ESLint/knip passing. After final lifecycle and hydration refinements,
the five focused suites passed 68 tests (3.50s), and final SessionRow/relative
suites passed 10 tests (1.56s). Final `yarn lint` also passed, including typecheck, ESLint and knip. The existing backend
cron projection/run adapter/error/trigger/cancel subset passed (0.690s), including
last-success, next-run and suspended/invalid omission controls. This was a local
contract regression run; no live scheduler or runtime-log replay is claimed.

Three simplify reviews completed: reuse removed the unused interval parameter,
quality verified state/hydration ownership, and efficiency verified shared
lifecycle and the unchanged-snapshot notification guard. No dependency added.

The API subset asserts core/REST last-success and next-run fields and omission. GraphQL/MCP run-adapter coverage does not independently assert every service timestamp field; direct source mapping establishes that boundary.

## Concurrent implementation reconciliation

Commit `79910e98c` independently shipped this milestone during verification. Its clock implementation, custom-cadence compatibility, common/deploy regressions, docs and board evidence are retained. Earlier local checks above include a single-cadence hidden-pause implementation that was not shipped; those lifecycle details are historical, not claims about the retained implementation. Additional tests retain only unique cron-header, clock-lifecycle and SessionRow hydration coverage. Final merged checks are recorded below.
