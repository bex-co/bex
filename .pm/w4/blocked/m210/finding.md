# Old completed cron runs are treated as new after history fills

Why: the Recent Runs panel and API put an old successful execution above newer failures once the ten-entry history is full, making the operational history misleading.

**Severity:** minor. Source: functional $qa-find-bugs w4, 2026-10-14 a475. Filing only. [Complete requests, responses, boundary controls and cleanup](../../evidence/m210-cron-history-boundary.json).

## Live reproduction and target

Create owned Free image cron **qa-20261014-a475-history**, `srv-db7rd1mt41sc7388odc0`, BusyBox1.37, annual future schedule `0 0 1 1 *`. Trigger one normal manual run using `sh -c 'printf QA_A475_OK; exit 0'`; it succeeds. Change Command through native Settings to `sh -c 'printf QA_A475_FAIL; exit 7'`. Confirm ten manual runs, waiting for each preceding run to finish before the next. No scheduling acceleration, direct Job creation, backend trigger bypass, malformed request or concurrent/preempting execution was used.

The first ten records stay in correct order. At **17:13:49.488 UTC**, after run11 finishes, the full GraphQL history has ten entries but begins:

| Returned position | Run ID                   | Started UTC | Status       |
| ----------------- | ------------------------ | ----------- | ------------ |
| 1                 | crr-mrbv6fhqr1m3ugo8ggq2 | 17:07:03    | successful   |
| 2                 | crr-fqgri7sdoscl0bm67nbi | 17:13:43    | unsuccessful |
| 3                 | crr-u1engo2vfc3bb5iu1dho | 17:13:35    | unsuccessful |

The oldest success reappears at the head; the second-oldest run `crr-m9r0i60abb555k0p53vq` is omitted instead. Five independent unchanged reads at17:14:11–22 retain that wrong set/order. A fresh1440×1000 page shows **7m Succeeded** before newer failed runs; fresh390×1200 shows **9m Succeeded** above **3m Failed**. REST `GET /v1/cron-jobs/$id/runs?limit=10` returns the same order and set. The ten-entry cap is documented and is not itself the finding.

Restore a successful command and trigger run12. It succeeds at17:17:59, advancing lastSuccessfulRunAt correctly. History now begins new-success17:17:54, **old-success17:07:03**, failure17:13:43…: old success still precedes newer failures and displaces another more-recent entry.

**Target:** retain exactly the newest ten executions in chronological run order, newest first, with deterministic ties, stable IDs and correct terminal states. For the eleven-record fixture, the old success falls outside the retained set and run11 is first. Repeated unchanged reconciles and reads preserve that set/order; another run replaces only the oldest retained row. Keep the ten-entry bound, rather than increasing it to postpone the defect. This does not request an unbounded history or a new telemetry store.

Complete inspected card images: `.playwright-mcp/qa-a475-history-reordered-{desktop,mobile}.png`. Initial failure-control mobile reload was390×844; boundary screenshot/reload was390×1200. Do not claim a separate boundary844 capture.

**Passing controls:** first failure preserves lastSuccessfulRunAt17:07:08; recovery advances it17:17:59. All outcomes shown are the same accepted IDs; no replay/repeated execution was observed. The command's configured numeric exit7 is known, but actual exit code/stdout were not fetched independently.

## Producer, data contract and consumers

`lego/operator/internal/controller/app_controller.go:4082–4084` caps history at10. `:4484` sorts actual Job objects by jobStartKey (start time, else creation). `:4490–4493` builds the prior-name map only from the already-capped status slice. **4516–4522 then assumes a Job absent from that slice is new**, placing every such Job before retained records. A historical completed Job whose status entry was evicted is therefore indistinguishable from a new execution. `:4527–4543` appends prior entries and caps again. The recorded live symptom matches this code; underlying production Job metadata was not queried, so physical Job presence is source-inferred, not independently observed.

Manual Job creation at `:4438–4474` owns it under the App, with no TTL. There is one production cronRuns call at4238. Keep its failure-reason/exit-code preservation and canceled-run overrides intact. The ordering is separate from manualRunHandledAt acknowledgement and whether a Job may execute.

`lego/types/v1alpha1/app_types.go:1221–1248` has Name, StartedAt, FinishedAt, Status, FailureReason and optional ExitCode. It lacks a creation/order clock for a canceled-before-start record after its Job is deleted. Thus a naive sort solely by StartedAt would repair these completed runs while regressing retained cancellation ordering.

**Fix design:** merge current Job observations and retained terminal records by Job name, refresh authoritative state on the matching record, establish their chronological order, then apply the cap **once**. Persist an optional internal creation/order clock when needed to keep pre-start tombstones sortable after deletion; current Jobs supply their real creation/start clocks. Preserve the existing start-else-creation ordering meaning. Define deterministic ties and legacy fallback explicitly: recover from an existing Job or known stored timestamp; retain prior relative ordering where no clock is recoverable, without inventing a start time. Model those legacy/pre-start cases in t001 before implementation. New optional status metadata must be added in types and generated CRD/deepcopy output; adding an undeclared key only in the controller would be pruned. It must not change public run IDs or require a new public field.

Do not use absence from bounded status as a new-execution predicate, let evicted old Jobs jump to the head, or simply sort the wrong retained subset in the API/UI. Keep the operator DB-free and preserve `operator → types ← backend`.

Consumers all preserve incoming order:

- `backend/internal/apps/service.go:1260–1277`: cronRunViews maps the slice and derives each immutable crr-ID from the existing Job name.
- `:3606–3616`: ListCronRuns paginates that slice. `internal/core/pagination.go:404–423` selects the requested window; it does not repair order or membership.
- REST `apps/rest.go:1080`, GraphQL `graphql.go:1173`, and MCP `mcp.go:738` each call that core list once. `render.go:727–733` preserves order in the resource/cursor wrapper.
- AppView also projects the same slice at `service.go:1106`, while `lastSuccessfulCronRunAt:1303–1310` selects its first succeeded record. Preserve and test that neighbor; this fixture's success timestamps passed.
- `dashboard/src/features/services/hooks/use-cron-runs.ts:85–139,198–207` reads the first five rows and preserves API ordering across its retained/appended pages; `components/cron-runs-section.tsx:234` maps them directly. Canonical services Events and Render cron aliases use this family. No frontend sort/cache/poll rewrite is justified.

Framework mechanism is the explicit sort/map/append/cap code above, not an inferred Kubernetes list-order guarantee or React reordering. The Kubernetes Job schema supplies creation/start times; the CR status schema currently does not preserve the missing creation clock for deleted pre-start Jobs. Generated schema and compatibility must move with any internal-field addition.

## Dedupe, prior guarantees and evidence limits

Searched open/blocked/done boards for cron history, old/newest/reappearing records, caps, prior order and maxCronRuns; scanned **67 open/blocked milestone READMEs**, anti-goals, recent source history and targeted blame. No open owner or landed ordering fix found. Blame pins the faulty new-entry assumption and prior-order branch to **a8d7c059d, July15**, so this is an uncovered original cap case, not a demonstrated later regression.

- **w5/m108** prevents canceled executions running again after history eviction. Full DoD disposition: display retention stays bounded, and fresh triggers execute/terminate normally here. Canceled-Job absence, cancellation-slot replacement, controller restart, suspend/resume and legacy adoption were not replayed; preserve its independent acknowledgement and those tests. No execution resurrection is claimed.
- **w5/m60** supplies Trigger Run/detail UI; trigger/live history passed, run-detail expansion and notification/registry settings were not recertified. Its wider suite/parity DoD remains independent.
- **w4/230** repairs appended-page recovery after a shifted first page with eight rows; this defect is already in the complete backend ten-row set and appears after fresh reload. Its Load more recovery was not replayed.
- **w9/056** derives the success timestamp from bounded history; first-failure and recovery timestamps passed here. No loss/regression of that timestamp is claimed.
- **w4/m188**, command refresh276, preemption/m114 and scheduling guards remain separate. Chinese/local clock and whole prior suites were not recertified.

[Render's trigger endpoint](https://api-docs.render.com/reference/run-cron-job) describes normal manual execution and its statuses. Bex's GET history is a documented extension (`render.go:720`); upstream retention/internal storage are unspecified in the primary sources reviewed by m108. This filing restores Bex's own documented newest-first contract; no authenticated Render cap experiment or identical retention policy is claimed. Scheduled/manual mixtures, deletion/tombstones, ties, legacy records, process restart, cursor tails and MCP/CLI execution are source-traced verification work.

## Cleanup

Native DeleteService true17:19:19.183 UTC. Service and run-list REST404, all seven exact baseline ID sets restored17:19:26.541. Own logout succeeds; whoami401, cookies0, one blank page17:20:47.445. Own observers removed and stat-matched session handles deleted. No baseline resource, paid plan or cybersecurity action. No implementation shipped.

## Native CLI corroboration (2026-10-15 UTC)

[Fresh w8 sweep254](../../../w8/evidence/cron-history-sweep254/README.md) reproduces the same incorrect retained set on one CLI-created Free native Go cron in bex-canary. Native command replacement and recovery work, all12 accepted manual API triggers terminate as expected, both clients preserve all12 log messages, and REST/GraphQL/MCP agree on the wrong cap11/recovery12 membership/order. The source fix f42f5d086 is on main and its four focused tests pass; deployed revision is unknown. This is attached to the existing defect, with no new implementation bug or execution-replay claim. Exact service/run404, baseline-six and quota-zero-termination cleanup is retained.
