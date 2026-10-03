# Relative log-window continuity — implementation evidence

Implemented 2026-10-02. The service reader separates resource/filter/explicit
range identity from clock-driven transport bounds. A same-reader refresh retains
and deduplicates the usable head and loaded older pages while pending and after
completion. Explicit selection changes reset immediately; generation checks
reject delayed completions even across A→B→A selections. Stored history is
pruned at the moving lower bound, slow page completions are pruned before commit,
and continuations are bounded by the current range.

The virtualized viewport anchors to its visible row instead of treating all
height growth as a prepend. This preserves an unpinned reader across head/SSE
appends, older prepends and expiry. Initial skeleton and fixed viewport geometry
are unchanged. Background transport, timeout and no-store failures retain useful
rows with an alert; access/session/not-found failures clear the retained reader.
SSE subscription identity and its frozen connection lower bound are unchanged;
expired live rows are omitted from the current display.

## Local verification

- New real-Apollo/cache integration tests exercise the actual relative clock,
  head queries, pager, and virtualized list: 100+40 rows survive two delayed
  ticks, preserving viewport identity, a concrete unpinned scroll position,
  nonempty reading anchor, uniqueness and exhausted continuation.
- Slow older-page completion crossing a tick prunes expired rows. Resource,
  filter, preset and Custom changes discard pending old-reader pages. Network
  failure preserves usable content plus error; forbidden, not-found and
  unauthenticated responses clear it. All ten integration cases pass.
- Original hooks loaded through temporary baseline module imports fail the two
  rollover/slow-page cases by dropping to zero rows; four explicit-reset controls
  pass. Temporary modules were removed.
- A separate real-virtualizer regression checks head append, older prepend and
  prefix expiry without moving the visible row. Viewer tests check mounted
  content plus honest alerts for three background failure classes.
- Full `dashboard/yarn test`: **463 files / 3,986 tests passed**. `yarn lint`
  passes, including typecheck, ESLint and unused-code checks. Final focused
  checks also pass. The first full run caught a hardcoded test timestamp outside
  Last hour; the fixture now uses a relative timestamp and the complete rerun
  passed.
- Existing logs/deploy/metrics/scaling and route controls: 19 files / 212 tests.
  Full backend `go test ./internal/logs -count=1` passes. No backend production
  code, API shape, cursor format, page cap or retention policy changed.
- Three simplify reviews completed. The lower bound is parsed once per viewer
  merge; expiry scans are memoized by retained pages and lower bound. Existing
  mapping/deduplication and error classification are reused.

## Shared callers and parity

Three useLiveRange callers remain service logs, Metrics and Scaling. Two
useLogHistory callers remain service and datastore viewers. Two useOlderLogPages
callers remain history and the deploy helper (three typed legs). One LogViewer
mount serves the service route; two datastore mounts serve PostgreSQL and Key
Value. DeployLogPanel has one mount and DeployDetailPage two route families.

Only service logs supply the semantic selection key that permits clock rollover.
Fixed deploy/datastore query changes still reset; same-window deploy polling
retains older pages. Existing controls cover all three deploy legs, datastore
filters/capped/empty/error/paging behavior, Metrics/Scaling clocks, route aliases
and URL state. Static has deploy history but no runtime Logs; web/private/worker/
cron share the service viewer, while datastores use their own viewer.

The [official Render logging documentation](https://render.com/docs/logging)
was checked for range, Custom, live and deploy-log controls. Its exact idle-clock
implementation was not executed or inferred. ADR010/ADR018 describe Bex's reader
continuity and unchanged API boundaries, not a live Render comparison.

## Remaining deployed acceptance

The release pipeline must deploy the dashboard. QA must replay the full numbered
Free-service fixture, two independent two-tick observations, Custom/search/URL
controls, GraphQL/REST/MCP cursor probes, live arrival, long-build paging and
required sibling journeys. Measure desktop and narrow-mobile pending/ready
geometry in that browser replay; local virtual geometry is not a screenshot or
real browser-layout claim. Delete owned fixtures, verify exact artifact and
API/public-route absence, and revoke the QA session. No hosted fixture or session
was created by this implementation run. Tasks t003/t006 retain this acceptance;
passing local tests or a successful push does not establish deployment.
