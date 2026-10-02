# Service Activity freshness — verification

## Scope and existing contracts

`useServiceEvents` has three production callers: the shared Events page, Metrics
chart markers and EventTimeline. Events mounts under both services and static
routes; Render-style aliases preserve the route suffix. The service page covers
web, cron, worker and private services. Postgres and Key Value have separate
logs/metrics views and do not use this hook. Deploy detail also has its own
timeline hook, outside this change.

Metrics owns its selected-window schedule through `useLiveRange`; chart markers
and EventTimeline receive explicit bounds with automatic cursor pagination.
Relative windows advance on their resolution, while custom absolute ranges
remain fixed. The new temporal hook tests verify a 30-minute range advancing at
15 seconds, an unchanged absolute range, transition to custom range, and timer
cleanup. All three tests pass.

The backend's shared event List authorizes against the owning App, validates
the time cap and forwards inclusive bounds/cursors. Event IDs derive from the
stored event key. REST, GraphQL and both MCP tools retain this contract. MCP
tools keep their different defaults/envelopes; explicit identical bounds still
use the same List operation. The existing backend event suite passed (0.757s).

## Parity

The official [event-list reference](https://api-docs.render.com/reference/list-events)
documents start/end bounds, cursor pagination and limits; [deployment docs](https://render.com/docs/deploys)
describe deployment history. Neither specifies a dashboard refresh SLA. The
30-second requirement is Bex's `RESOURCE_POLL_INTERVAL_MS` policy. No
authenticated Render UI comparison was performed, and no API range cap or
event vocabulary changes are needed.

## Regression evidence

Five mounted-page regressions failed against the original frozen-window code:
initial In Progress/Cancel assertions passed, but later Live/Failed/Canceled and
sleep/wake rows never arrived. The Apollo test transport applies the actual
720-hour cap and timestamp bounds rather than asserting a timer option. Tests
cover the shared services/static page and retained filter selection.

## Hosted acceptance still required

After the release pipeline deploys the dashboard, QA must repeat the README's
visible deploy and sleep/wake sequence, compare retained IDs after reload and
bounded historical/current API reads, verify Metrics' live control, and delete
the owned fixture and revoke its session. No production fixture was created by
this implementation run. Local tests do not certify hosted timing, every
resource alias, or authenticated Render behavior.

## Implemented behavior and review

Events opts into a visible-only 30-second head refresh. Both time bounds advance
within 720 hours. Head pages merge by stable ID in chronological order, preserving
the loaded history cursor; pagination scans through the loaded portion to catch
bursts and delayed facts. Metrics retains its existing explicit-window behavior.

Synchronous request guards prevent duplicate paging/refresh calls. Generation
and unmount checks reject late responses, including A → B → A navigation.
Failures retain rendered rows with an inline retry notice. Retry uses current
bounds, including recovery from an initial query error. Initial pending and
normal ready geometry are unchanged.

Focused tests cover timestamp precision, overlapping/new/older pages, bursts,
delayed facts, hidden tabs, unmount, failed reads/retry, and service-switch
responses. Six mounted-route cases pass; disabling live refresh made all five
original arrival cases fail, and restoration made them pass again. The added
sixth case verifies visible rows survive refresh failure and retry reaches Live.

Three parallel simplify reviews completed. The duplicate Metrics timing case
was removed in favor of the dedicated stronger tests. No additional production
abstraction or scheduling changes were justified. The loaded-range rescan is
intentional to discover delayed facts; refresh overlap is suppressed.

## Final checks

The complete dashboard suite passed: 462 files / 3,972 tests (67.08s). Typecheck, ESLint and knip passed with no ESLint warnings; diff whitespace checks passed. Eight hook, six route and three Metrics timing regressions are included.
