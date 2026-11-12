# w4 · m322 — A cron job's manually triggered runs, and every run of a cron job named 28–30 characters, chart no CPU, memory or instances and meter no instance time

**Worker:** worker4 **Goal:** every run a cron job executes appears in its Metrics and in its metered `instance_seconds`, whether the run was scheduled or triggered, and whatever its name length. No other App's pod is counted. **Status:** blocked — t001–t003 and t005–t007 done (shipped); t009 (round 166: long-named run pods are not truncated) is open; t004's live replay passed DoD 1 and waits on t009 for DoD 2, then t008 closes

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Select every run pod of a cron App in the shared cAdvisor matcher: manual-run Jobs and hashed long-name CronJob Jobs — **DONE** | 60m | — |
| [t002](done/t002.md) | Blast radius: the three matcher callers, sibling-App isolation and regression tests on the shapes that work today — **DONE** | 30m | t001 |
| [t003](done/t003.md) | A cron App that ran in a window but meters zero is a degraded read, not an honest zero — **DONE** | 30m | t001 |
| [t009](t009.md) | A long-named cron job's run pods are not truncated (`<cronjob>-<minute>-<5>`, `<cronjob>-r<8 hex>-<5>`, 67–68 characters), so t001's single-segment alternative matches none and the job still charts nothing | 40m | t001 |
| [t004](t004.md) | Live verification in `bex-canary` | 30m | t002, t003, t009 |
| [t005](done/t005.md) | Render parity — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 10m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005 |
| [t008](t008.md) | Closeout | 10m | t006, t007, t009 |

Nine tasks, 255m (4h15m); t009 added in round 166.

**Major.** The Metrics page reads "No data in range" for runs that ran, and the usage meter, which bills cron jobs per second of run time, records none of that time.

## Finding

Live `/qa-find-bugs w4` loop, round 81, 2026-11-11 06:40–07:13 UTC (`muse.env`, workspace `bex-canary` on the Scale plan; production runs the code of `348bd77a5`).

Fixtures, both owned and deleted at the end of the round:

- `qa-20261111-cr81` (`srv-dbq0s52p92ls73cjjfn0`): Starter cron job, image `docker.io/library/busybox:1.36`, created in the dashboard. Command `sh -c 'echo qa81 start; sleep 60; echo qa81 done'` (later `… host=$(hostname); sleep 45 …`). Schedule `0 0 1 1 *`, then `*/5 * * * *` from 06:55 to 07:02.
- `qa-20261111-cr81-longname-0030` (`srv-dbq19cak10fs73dvos2g`): the same image, a 30-character name, schedule `*/5 * * * *`, created with `POST /v1/services`.

### Probes

All API reads ran from the signed-in dashboard page with `fetch(…, {credentials: "include"})`.

- **Nine manual runs, no samples.** `GET /v1/cron-jobs/srv-dbq0s52p92ls73cjjfn0/runs?limit=20` lists runs that executed for 11–65 s each:
  - successful 06:42:59–06:44:04, 06:49:59–06:51:03, 06:52:34–06:53:38, 07:02:56–07:04:01, 07:04:26–07:05:15 and 07:06:49–07:07:38;
  - canceled mid-run 06:46:34–06:46:50 and 06:48:00–06:48:11.

  The request log has their output: `qa81 start` at 06:43:02, `qa81 done` at 06:44:02, and so on.
- **Only the scheduled run charts.** `GET /v1/metrics/{cpu,memory,instance-count}?resource=srv-dbq0s52p92ls73cjjfn0&startTime=2026-11-11T06:40:00Z&endTime=<now>&resolutionSeconds=60` at 07:09:18Z returns:
  - cpu: one series, instance `srv-dbq0s52p92ls73cjjfn0-m9o1vpp13mqbv8hepf7t`, `07:01=0.00`;
  - memory: the same instance only, `07:01`–`07:05` ≈ 385 kB;
  - instance-count: `07:01=1`, `07:02=1`.

  That instance is the 07:00:00–07:01:05 **scheduled** run. Before 07:00, at 06:57, all three returned `[]`.
- **The pod names.** A manual run printed `host=tea-daif693dqjvc73e7as3g-qa-20261111-cr81-run-1c18537e-wkvfv`, which is the pod name. The App's Kubernetes object name is `tea-daif693dqjvc73e7as3g-qa-20261111-cr81`.
- **Long names: scheduled runs too.** The 30-character cron job's scheduled run (07:10:00–07:10:49, successful) printed `host=tea-daif693dqjvc73e7as3g-qa-20261111-cr-5cb8a9072637-29906350-q`. That is its hashed CronJob name, the minute stamp, then the pod suffix (the hostname is cut at 63 characters). Its object name is `tea-daif693dqjvc73e7as3g-qa-20261111-cr81-longname-0030`. At 07:11:35Z, `cpu`, `memory` and `instance-count` for `srv-dbq19cak10fs73dvos2g` over 07:05–07:11 each returned `[]`.
- **Dashboard.** On Metrics (Last 12 hours, 06:56, before the scheduled run), the first cron job showed:
  - the Instances list with three run instances;
  - `Memory … No data in range`, `CPU … No data in range` and `Total Instances … No data in range`.

  The long-name job shows the same on Last hour, with one instance listed. The listed instances come from the pod-spec limit fallback (`GET /v1/metrics/cpu-limit` answered three instances at one timestamp), which finds the pods by label rather than by name.
- **Usage, supporting but not conclusive.** `GET /v1/usage?ownerId=tea-daif693dqjvc73e7as3g` (period 2026-11) shows `instance_seconds` total `0` for all seven cron jobs metered this month:
  - `qa-20261105-cron86`, `qa-20261105-cronfail108`, `qa-20261106-cr121`, `qa-20261106-cr125`;
  - `qa-20261110-cron`, `qa-20261110-54a1-cron`, `qa-20261110-cron2`.

  263 of the 350 `srv-` rows are non-zero. These fixtures' run lengths are unknown, and a run shorter than one 15 s scrape can legitimately read 0. This round's fixtures had not rolled up when they were deleted.

### Root cause

- **The selector is a pod-name regex.** CPU, memory and instance-count select an App's pods by name: `egressquery.PodNameMatcher` (`lego/backend/internal/egressquery/podname.go:92-94`), called from `promResourceQueryFor` (`lego/backend/internal/metrics/source.go:288`). cAdvisor series carry pod names but no pod labels (`source.go:262-270`).
- **The regex accepts two shapes** (`podname.go:69-84`):
  - `<obj>-[a-z0-9]+-[a-z0-9]{5}`;
  - the generateName-truncated `<obj>-[a-z0-9]{62-len(obj)}`.

  Its doc names "`<cronjob>-<timestamp>-<5 random>` for a CronJob's Job pods" (`:48-49`). Two run-pod shapes match neither:
  1. **Manual runs.** `ManualCronRunJobName` (`lego/types/v1alpha1/cron.go:39-42`) names the Job `<app>-run-<8 hex>`, so its pod is `<obj>-run-<8 hex>-<5>`: three segments after the object name. Every manually triggered run is affected, at any name length. Past 63 characters, `k8sname.Fit` hashes the Job name, so the pod does not even start with `<obj>-`. The shape dates from `3329aec7a`/`20bfa08b5`, so this is not a regression.
  2. **Long names.** `CronJobName` → `k8sname.FitCronJob` (`cron.go:30-32`, `lego/types/k8sname/k8sname.go:64-68`, `MaxCronJob` 52) hashes any object name over 52 characters to `<first 39>-<12 hex>`. The tenant prefix is 24 characters plus a hyphen, so this means any service name of 28–30 characters. That cron job's scheduled Job pods are not prefixed by `<obj>-` either.
- **The usage meter shares the selector by design.** `queryInstanceSeconds` (`lego/backend/internal/usage/service.go:1236-1249`) calls `PodNameMatcher(namespace, crName)` "precisely so a service can never be charged for pods its Metrics page does not chart". So these run seconds are never metered either.
- **The zero is treated as honest.** `expectsRunningPods` (`usage/service.go:1417-1419`) returns false for every cron App. A zero is therefore recorded as healthy, and coverage never degrades.
- **The selector is not wrong for Deployments.** w6/m110 verified the truncated shape live for web services. This defect is confined to Job-backed run pods.

### Fix (target behavior)

- **One selector, every run.** The cAdvisor selector for a cron App selects every pod its CronJob or manual-run Jobs create, scheduled or triggered, at every service name length up to 30. It selects no pod of any other App. Metrics CPU, memory, instance-count and limit history, and the usage meter, read that one selector, so they cannot disagree.
- **Recommended: select by identity, not name shape.** Run pods carry the App's labels (`app_controller.go:4988`, `:5284`), but kube-state-metrics exports none today; `deploy/gitops/base/prometheus.yaml:108` allows only `nodes=[bex.co/pool]`. Two ways to get there:
  - add the App identity label to `metricLabelsAllowlist` and join cAdvisor series `on(namespace, pod)` to `kube_pod_labels`;
  - join through `kube_pod_owner` and the Job → CronJob/App ownership.

  Either works for every name length and both Job kinds.
- **Acceptable alternative: rename.** Name both Job kinds so their pods fit `PodNameRegex`, for example a manual run as `<obj>-m<8 hex>`, and teach the regex the hashed `FitCronJob` prefix. Two constraints:
  - A regex alternative `<obj>-run-[0-9a-f]{8}-[a-z0-9]{5}` is not acceptable on its own. It also matches the Deployment pods of a sibling App named `<name>-run` whenever the pod-template hash happens to be eight hex-alphabet characters, which breaks the anchoring guarantee `source.go:262-270` states.
  - Renaming changes `ManualCronRunJobName`, which the backend (`apps/service.go:1348`, `:4154`) and the operator (`app_controller.go:5246`, `:5545`) both use for a run's identity (w4/m300). State how a run triggered during a backend/operator version skew keeps its `crr-` id.
- **No silent zeros (t003).** See t003.
- **Pre-settle state.** Not applicable: metrics are read on demand. Already-rolled-up usage hours stay as recorded unless t003's decision repairs them.

### Blast radius

- **`PodNameMatcher` has three production callers:**
  - `metrics/source.go:288` (cpu, memory, instance-count);
  - `metrics/source.go:453` (limit history);
  - `usage/service.go:1246-1248` (instance seconds).

  `PodNameRegex` is only called by `PodNameMatcher`. Datastores use `statefulInstanceMatchers` (`usage/service.go:1263-1267`) and are unaffected.
- **Surfaces fixed by the one change:** REST `/v1/metrics/{cpu,memory,instance-count,cpu-limit,memory-limit}`, GraphQL `metrics`, MCP `get_metrics`, the dashboard Application Metrics card, the Scaling page's Recent Metrics, and `GET /v1/usage` (REST, GraphQL, MCP and the Billing page).

### Unverified

- **Billing.** Whether the hourly rollup of this round's hours reads 0 for the manual runs. The fixtures were deleted first (rollup ticks hourly from process start, `usage/service.go:547-548`, `:580-590`); the claim rests on the shared matcher.
- **Render's own cron Metrics tab.** That it charts CPU and memory per run is unverified.
- **Egress.** Whether run pods' egress is attributed to the cron App. That is the label-based meter (w4/453), not this selector.

### Render

- Cron jobs are billed per second of run time (render.com/docs/cronjobs; the same basis as w4/546). A triggered run is a run.

### Dedupe

- **w4/m173 (blocked)** and **w5/m111** own the instance-seconds query's arithmetic and staleness, not which pods it selects.
- **w6/m110 (done)** added the truncated Deployment shape.
- **w4/m300 (blocked, pending live closeout)** owns run identity and the `crr-` id; t001 must keep its guarantees.
- **w4/546** owns the New Cron Job form's pricing copy.
- **w7/m154 (done)** introduced `FitCronJob` for CronJob admission. It did not touch metrics.
- `grep -ril "PodNameRegex\|manual run" .pm` finds no item about run-pod selection.
- **Not deploy lag:** `git log 348bd77a5..origin/main -- lego dashboard` is empty.

## Definition of done

Signed in to `bex-canary` after the release pins this build, with owned Starter cron jobs (image `docker.io/library/busybox:1.36`, command `sh -c 'echo start host=$(hostname); sleep 45; echo done'`), deleted afterwards:

1. **Manual runs chart.** On a cron job with a short name and schedule `0 0 1 1 *`, `POST /v1/cron-jobs/{id}/runs` three times, each after the previous run ends.
   - `GET /v1/metrics/cpu`, `memory` and `instance-count` over the window return one series per run, labelled with that run's instance.
   - The dashboard Metrics page charts all three runs. It never reads "No data in range" for a window holding a run.
2. **Long names chart.** A cron job named with 30 characters on `*/5` charts its scheduled run and one manual run the same way.
3. **Usage meters them.** After the hour rolls up, `GET /v1/usage?ownerId=tea-daif693dqjvc73e7as3g` lists each fixture's `instance_seconds` within ±15 s per run of the summed run durations from `GET …/runs`.
4. **Siblings stay out.** With a Free web service `<name>-run` beside a cron job `<name>`, neither's Metrics nor usage includes the other's pods. A web service's Metrics and usage are unchanged, at both a short name and a 30-character name.
5. **Checks pass.** Backend `go test ./...` with `BEX_TEST_REQUIRE_DEPS=1`, operator `make test` if the operator changes, and dashboard `yarn lint` and `yarn test` if the dashboard changes.

## Source + Goal linkage

- **Source:** the finding above (round 81 of the `/qa-find-bugs w4` loop, 2026-11-11). The durable evidence is the probe record above; no screenshot was kept.
- **Goal linkage:**
  - [ADR038](../../../../docs/ADR038-cron-jobs.md) (cron jobs) and [ADR010](../../../../docs/ADR010-observability.md) (the metrics selector).
  - ADR023/[ADR040](../../../../docs/ADR040-billing-metronome.md) (usage metering and billing) and [ADR030](../../../../docs/ADR030-pricing.md).
  - [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** a cron job's Metrics and usage describe the runs it actually executed. A manually triggered run, the normal way to test a cron job, is charted and billed like a scheduled one.
- **Why now:**
  - A user debugging a cron job reaches for Trigger Run first, then sees "No data in range" for the run they just watched succeed.
  - Every paid cron run triggered that way is unbilled, silently: the zero reads as healthy.
- **Render parity included (t005):** REST, GraphQL and MCP metrics and usage numbers change, and the dashboard charts change.

## Post-release probe (QA loop round 166, 2026-11-12): DoD 1 passes, DoD 2 fails (filed as t009)

Pin `e7a56b07e0d7` (`c5518dc24`, 02:23 UTC) contains `c1cc477b8`. Fixtures are in t009, deleted afterwards (GET 404).

- **DoD 1 passes.**
  - Three `POST /v1/cron-jobs/<A>/runs` (no body; a `{}` body is refused as "request body is not allowed for this operation", Render's contract), each after the previous run ended.
  - `GET /v1/metrics/cpu` returned one series per run instance (`…9hngb0mq`, `…1rn0i5u6`, `…38jb3gir`), and `memory` the same three.
- **DoD 2 fails.** The 30-character `*/5` cron's manual and scheduled runs both succeeded, and `cpu`, `memory` and `instance-count` all returned `[]`. The run pods' names are not the truncated shape t001 matches (t009).
- **Seen for DoD 3, not judged yet:**
  - A's `memory` series continue at the run's last value for ~5 minutes after each run ends. Run 1 ended 04:24:45, and its series has points 04:25–04:29.
  - `instance-count` read `2` at 04:27, though runs 2 and 3 didn't overlap (run 2 ended 04:26:00, run 3 started 04:26:33).
  - If the usage meter counts those tails, each 49 s run would meter several minutes. The hourly rollup wasn't waited for, so DoD 3's ±15 s check is still open.
- **Not run:** DoD 3 (usage after the hour rolls up) and DoD 4 (sibling isolation).

