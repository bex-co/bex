# Shared relative presentation clock — implementation evidence

Implemented 2026-10-02. RelativeAge and RelativeUntil consume the existing
useNow hook, now backed by one shared clock per cadence rather than one interval
per subscriber. Default cadence is 60 seconds. Returning a hidden document to
visible refreshes the sample immediately; delayed interval callbacks read the
actual current clock rather than incrementing a counter. The last subscriber
removes the interval and visibility listener. Idle store identity is retained so
StrictMode/navigation cannot create competing timers for one cadence.

Exact datetime/title values, compact formatting, missing/invalid fallbacks and
as=span behavior remain unchanged. The hydration snapshot is cached per mounted
hook and clock-derived text retains its hydration guard. No API polling or
scheduler write was added. SessionRow samples the same clock for its accessible
revoke label so it stays synchronized with the visible age.

## Verification

- Mounted unchanged-prop Age/Until tests cross three minute buckets without a
  parent rerender; exact datetime/title and fallback/span contracts remain.
  Forty labels plus the deploy-style clock share one timer and release it on
  unmount. Separate cadences remain bounded by cadence count; changing cadence,
  unmount/remount, late time and hidden-tab resume are covered.
- Original-wrapper negative controls fail both new mounted-clock tests while
  seven pre-existing format/hydration controls pass. Temporary negative-control
  files were removed before the final full-suite run.
- A real SSR/hydrateRoot regression advances Date.now on every read and checks
  that hydration reports neither unstable-snapshot warnings nor recoverable
  errors. Existing boundary and bare-formatter negative controls remain.
- SessionRow's unchanged session advances visible and accessible ages together,
  retains the exact timestamp and button identity, and invokes no revoke.
  The actual deploy-list control advances 5→7 minutes with unchanged row data,
  DOM time element and exact source instant. These caller suites pass 24 tests;
  common clock/relative-time suites pass 16 tests.
- Full dashboard suite: **464 files / 3,995 tests passed**. Dashboard lint,
  including typecheck, ESLint and unused-code checks, passes.
- Existing header/deploy/cron-run/non-static/datastore metadata controls pass:
  six files / 69 tests. Backend cron timestamp and adapter controls pass.
- Three simplify reviews completed. The only required refinement was caching
  the hydration snapshot; React's state initializer supplies one stable sample
  instead of an impure repeatedly-read render-time snapshot.

## Scope and contract

The finding's inventory remains exact: 27 JSX sites across 18 files, comprising
26 RelativeAge uses and one RelativeUntil. All receive the common clock without
caller-specific polling. SessionRow is the sole direct production formatter
consumer outside the formatter module and now receives the same clock sample.
Web/static/cron/worker/private share the service header; PostgreSQL and Key Value
use their metadata/recovery views. Project, event, Blueprint, account and
integration rows retain their existing components and authorization boundaries.
The canonical service/static trees and web/worker/pserv/cron/d/r aliases are
unchanged. This source audit is not a live test of every resource family.

The [official Render cron documentation](https://render.com/docs/cronjobs)
confirms UTC scheduling. Render's authenticated countdown refresh cadence was
not observed. Backend createdAt/last-success instants, UTC next-run calculation,
and omission/null behavior remain unchanged; nextRunAt remains the existing Bex
extension. No job, schedule or runtime log generation was changed.

## Remaining release and QA gate

The release pipeline must deploy the dashboard. QA must run the two fresh,
untouched, two-minute cron-page samples from the README, correlate unchanged
DOM/API instants with wall clock, retain the deploy-row/scheduled-run/log controls,
and verify desktop/narrow rendering as part of that browser replay. Delete only
the owned fixture, confirm exact App/CronJob/Job/Pod/Secret and API removal, and
revoke the session. No live fixture or session was created in this run. Tasks
t003/t006 retain deployed parity/acceptance; passing local checks is not a claim
that the production dashboard already runs this code.
