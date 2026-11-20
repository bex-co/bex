# w3 · m506 — A running instance keeps at most 6,000 application log lines a minute, as on Render, so one flooding service cannot delay or drop every tenant's logs

**Worker:** worker3 **Goal:** the log shipper drops an instance's application lines beyond Render's documented 6,000 a minute, counts what it drops, and Loki's ingestion limits are explicit, so one service printing in a loop costs only its own excess lines. **Status:** done — t001–t005 and t007–t010 shipped in `e737b2412`; t006's 24-hour production read (§ Production read) found no 429, no discarded sample and no customer pod cut, and t011 closed the milestone on 2026-11-20

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Measure each instance's line rate on production — **DONE** | 30m | — |
| [t002](done/t002.md) | Alloy: cap `app_logs` at 6,000 lines a minute per pod — **DONE** | 45m | t001 |
| [t003](done/t003.md) | The live tail applies the same cap — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Explicit Loki ingestion limits and alerts on dropped or refused lines — **DONE** | 45m | t001 |
| [t005](done/t005.md) | dev-3 flood drill — **DONE** | 45m | t002, t003, t004 |
| [t006](done/t006.md) | Ship and verify on production — **DONE** | 30m | t005 |
| [t007](done/t007.md) | ADR010 and ADR018 — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Render parity — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 30m | t008 |
| [t011](done/t011.md) | Closeout — **DONE** | 10m | t009, t010 |

## Definition of done

- `app_logs` drops each instance's lines beyond 6,000 a minute and counts them; request and datastore logs are not capped.
- The live tail drops the same excess.
- On dev-3 a flood of 20,000 lines a minute keeps at most the cap and a neighbor loses nothing.
- Loki's ingestion limits are explicit; alerts fire on 429s and sustained drops; production shows no 429 retries after the pin.

## Shipped (2026-11-19)

- **t002:** `app_logs` keeps a per-pod `stage.limit` (rate 100, burst 6,000, `max_distinct_labels` 10,000, drop) before the severity parse; request, Postgres, Key Value and build pipelines have none. Prometheus keeps `loki.process.app_logs`' two drop counters (`label_value` names the pod). `scripts/gitops-validate.sh` fails if the stage changes, moves after the parse, or bex-api's constants drift; `scripts/test_log_shipper.py` `AppLogLineCapTest` floods 20,000 lines beside a quiet pod in the pinned Alloy image (fails without the stage, and with one bucket for both pods).
- **t003:** `lineCap` (`lego/backend/internal/logs/linecap.go`) runs in `followPods` before the filters, per instance, on line timestamps; the store-less history read (`readPodLogs`) keeps it too, beyond the task. Tests: `linecap_test.go` (fail with the cap removed, or applied after the filters).
- **t004:** `loki.yaml` sets 8 MB/s, 16 MB burst, 3 MB/s and 15 MB per stream, 5,000 streams, 256 KB lines (the dev-N copy too), pinned by `gitops-validate.sh`. Loki 429s already alert: `LogDeliveryLokiRejected` (discarded samples, `rate_limited` among them) and `LogDeliveryRetriesPersistent`, with rule tests. New: `AppLogLinesRateLimited` (info) names a pod dropped for 15 minutes, charted per pod on the Platform availability dashboard (ADR088 §6 wants a panel for every alert); `log_delivery_alerts_test.yml` covers a sustained flood, a five-minute burst and the writer alert staying quiet.
- **Deviation (dev-N):** the dev shipper tailed every pod from every node, which would have kept one bucket per node; it is node-scoped now, as production's is.
- **t009:** `/simplify` folded the cap guard into `gitops-validate.sh`'s per-pipeline loop. The bucket stays hand-written rather than `x/time/rate`: that limiter moves its clock back on an earlier-stamped line (stderr behind stdout) and has no last-time accessor for unstamped split lines.

## Measurement (t001, production, 2026-11-19)

Read from production Loki (each pod's lines per minute, `count_over_time`, in six-hour chunks) and Prometheus over the 14 days to 2026-11-19 14:50 UTC.

- **Platform.** Peak 38,172 lines in one minute (2026-11-12 08:03 UTC) and 3.3 MB in one minute (2026-11-19 09:53 UTC), 677 lines/s at Loki's distributor at the busiest minute, 9.1 lines/s on average. 267 active streams in the last hour, 21 running App instances, application lines of 151 bytes on average (24 h).
- **Loki's limits were Loki 3.6.7's defaults:** 4 MB/s, 6 MB burst, 3 MB/s and 15 MB per stream, 5,000 streams, 256 KB lines. In the 14 days no push answered 429 (pushes answered 204, plus 74 connection errors and 81 retries), no sample was discarded and the writer dropped nothing. The milestone's 429 storm was a risk, not yet an incident.
- **Per instance.** One service exceeded 6,000 lines in a minute: `twenty-crm` in `tea-d98210cbbpdc73dcrkvg`, 37,212 lines at 2026-11-12 08:03 UTC and 19,346 (another pod) at 2026-11-17 12:11 UTC, Node.js `Error: read ECONNRESET` stack traces in a loop, up to 2,808 lines in one second. Replaying each second's count through a 100/s bucket with a 6,000-line burst, the cap would have kept 21,614 of 70,150 lines between 07:30 and 08:30 on 11-12 and 9,164 of 20,563 between 11:45 and 12:40 on 11-17. The next highest were 5,617 (`twenty-crm`, 11-08 09:54, a 2,836-line second) and 5,499 (`beancount-forum`, 11-17 12:11); the bucket drops none of either. Build pods also ship a `type=app` copy in `bex-build` (peak 4,636 a minute), which no tenant read selects.
- **Expectation for t006:** only an instance printing more than about 100 lines a second for longer than a minute loses lines; on this traffic that is `twenty-crm`'s error loops alone.

## dev-3 drill (t005, 2026-11-19)

`dev-env.sh 3 up` from this tree: the dev shipper carries the cap and is node-scoped like production's (it tailed every pod from every node before, so each node would have kept its own bucket), Loki runs the explicit limits, and bex-api has the tail cap. In a fresh verified identity's workspace `tea-dbvh821jg4r2ukg7bc00`, two busybox 1.37 background workers on one node: `flood` (`srv-dbvh84hjg4r2ukg7bc10`) prints 333 numbered lines and sleeps a second, about 19,750 lines a minute (`seq=100000` at 15:26:59.69, 5 minutes 2 seconds after it started at 15:21:57); `quiet` (`srv-dbvh84pjg4r2ukg7bc30`) prints a numbered line a second. Stored lines per minute (Loki, by line time) and the live tail (`/v1/logs/subscribe` SSE for both services, 15:22:36 to 15:25:56, which replays from the pods' start):

| Minute (UTC) | `flood` stored | `flood` live tail | `quiet` stored |
| --- | --- | --- | --- |
| 15:21 (from 15:21:57) | 1,332 | 1,332 | 3 |
| 15:22 | 10,458 | 10,903 | 60 |
| 15:23 | 6,043 | 6,042 | 59 |
| 15:24 | 5,946 | 5,945 | 60 |
| 15:25 | 6,007 | 5,605 (to 15:25:56) | 60 |
| 15:26 | 6,046 | — | 59 |
| 15:27 | 5,945 | — | 60 |

- The flood's first minute keeps the 6,000-line burst plus the refill; every later minute keeps 6,000 within 1%. 35,832 of its 100,000 lines to 15:27:00 were stored. Alloy's `loki_process_dropped_lines_total{component_id="loki.process.app_logs",reason="ratelimit_drop_stage"}` read 60,452 at 15:26:43 and 71,363 at 15:27:2x, the same as `loki_process_dropped_lines_by_label_total` for the flood's pod; the quiet pod has no series.
- The tail and history agree within a line from the third minute on. The second minute differs by 445 lines: Alloy's bucket runs on its clock while it catches up the first seconds, the tail's on the lines' timestamps (ADR010 § Per-instance line cap).
- Every numbered `quiet` line arrived: `seq` 1 to 369 stored with none missing or repeated, the newest 1.1 s old when read at 15:27:2x; the tail carried `seq` 1 to 238 with none missing.
- Both fixtures were deleted (`DELETE` 204, `GET` 404).

## Render parity (t008, 2026-11-19)

| Surface | History | Live tail | Matches Render |
| --- | --- | --- | --- |
| REST `GET /v1/logs` | capped in the store (shipper) | — | yes: excess "do not appear in the explorer" |
| REST `GET /v1/logs/subscribe` (WebSocket, SSE, NDJSON) | — | `lineCap` before any filter | yes: excess do not appear "in log streams" |
| GraphQL `logs`, project views (`logs(project, …)`) | capped store | the same `followLogs` | yes |
| MCP `list_logs` | capped store | no tail | yes |
| Dashboard explorers (service, project) and the CLI | read the surfaces above | read the surfaces above | yes |
| Store-less fallback (no `BEX_LOKI_URL`) | `readPodLogs` keeps the same bucket | — | yes |
| Request, Postgres, Key Value, build lines | not capped | not capped | yes: Render caps "application-generated" lines |

- Same fields, semantics and error shapes on every surface: the cap removes lines and adds nothing, so no field, filter or error changes. Render shows no marker for dropped lines and neither does bex.
- **Unverified, recorded in ADR018:** Render does not document its window. bex keeps a token bucket, so a flood's first minute after a quiet spell keeps up to 12,000 lines, and a tail opened mid-flood passes up to one more burst than history kept. Not observable on Render without flooding a Render service; no follow-up filed, since no Render behavior contradicts it.
- **Not checked on Render:** whether Render counts a line split by the kubelet (over 16 KiB) once; bex counts the reassembled line once.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w3 2026-11-18/19 (topic: one aspect of Render parity, project and environment observability and the log explorer), filed by `/pmx them all to w3` 2026-11-19 after a read-only capture of Render's live project Observability views in the user's Render workspace (`docs/render-artifacts/project-observability.md`).
- **Goal linkage:** ADR008 § Render parity first: [render.com/docs/logging](https://render.com/docs/logging) documents the limit. bex has no cap on tenant application logs; only sandbox pods are limited.
- **Recommendation, not a user decision:** the token bucket (100/s, burst 6,000) is the recommendation, since Render does not document its window; ADR010 records where it differs from a per-minute count.
- **Expected outcome:** one service printing in a loop loses its own excess, as on Render, instead of slowing or dropping every tenant's logs.
- **Why now:** Loki runs single-tenant with default ingestion limits, so a flood from any one service gets every push refused with 429 and retried, delaying or dropping everyone's lines. The sandbox pipeline already proved the mechanism.
- **Render parity included:** the capped history is what every log surface reads.

## Gate (2026-11-19)

**Clears by itself; no decision needed.** t006 needs a production deploy carrying `e737b2412` (the shipper's cap, the Loki limits, the alert and the tail cap), then 24 hours of it. Check: the newest `chore(deploy): pin platform images to <sha>` on `main` passes `git merge-base --is-ancestor e737b2412 <sha>`, and production's `monitoring/log-shipper` ConfigMap carries `stage.limit` in `app_logs`.

**Next (t006, then t011):** once the pin is 24 hours old, fetch the production kubeconfig (`scripts/fetch-app-kubeconfig.sh`) and read, over those 24 hours: Alloy's `loki_process_dropped_lines_by_label_total{label_name="pod"}` per pod (expect only pods printing more than about 100 lines a second for over a minute, on current traffic `twenty-crm`'s error loops; § Measurement), `loki_write_request_duration_seconds_count{status_code="429"}` and `loki_discarded_samples_total` (expect none), and Loki's `/config` (8 MB/s, 16 MB burst). Record it here, then close out with t011. A pod dropped that the measurement did not predict is a finding to explain before closing.

## Unblocked (2026-11-20, `/loopx w3`)

- **Source:** verified evidence; no user decision was needed. The first pin carrying `e737b2412` is `c81d2d447bd8` (`aefb144dc`, 18:04:33 UTC on 2026-11-19): `git merge-base --is-ancestor e737b2412` fails on `dff52c596d13` before it and succeeds on it and on every later pin, through `11b1e8f85f06` (`0779189fc`, 15:24 UTC on 2026-11-20). The read ran at 18:05 UTC on 2026-11-20, so the 24 hours had passed. Production's `monitoring/log-shipper` ConfigMap carries `stage.limit` in `app_logs` (rate 100, burst 6,000, `by_label_name = "pod"`, drop).
- **When each part went live.** Argo CD's `log-shipper` and `loki` applications take their values from `main`, not from the pin. So the shipper's cap and Loki's limits went live when `e737b2412` reached `main` (17:07 UTC on 2026-11-19; `loki-0` restarted at 17:17:11), before the window opened. bex-api's tail cap came with the pin. No drop-counter series existed before 20:10 UTC on 2026-11-19, so nothing was dropped between 17:17 and the window's start.
- **w8/m525 inside the window.** `1c3ee254f` (bex-alloy `56d02b11e69b` with `limit-entry-timestamp.patch`, and `refill_by_entry_timestamp = true` on `app_logs`) reached `main` at 06:37 UTC on 2026-11-20. The DaemonSet rolled to revision 14 from 06:46:16 to 06:54:20, two hours before that change's pin `12716d1d7097` (`4e37ec03f`, 08:47:17; bex-api rolled `6ad7ebee0`'s tail side at 08:52:36). So the shipper's bucket refilled by the wall clock from 18:04 to 06:46 and by each line's own timestamp from 06:54 to 18:04 (§ Production read, "m525").
- **Next:** t006's read below, then t011.

## Production read (t006, 2026-11-19 18:04:33 → 2026-11-20 18:04:33 UTC)

Read on 2026-11-20 at 18:05 UTC from production Prometheus and Loki (kubeconfig from `scripts/fetch-app-kubeconfig.sh`; queries run with `wget` inside the `prometheus-server` container, since the API server's service proxy cannot reach pod IPs). Each drop counter is summed per series: its last value if the series was born inside the window, else its rise. `increase()` reads 124,864, because it misses the first sample of a series born mid-window (each pod's counter starts at its first drop), and the m525 roll replaced every shipper.

**No 429, no retry, no discarded sample.** 224,868 pushes from the shipper to Loki answered 204. Their `500` and `-1` series did not rise, and no `status_code="429"` series has ever existed. `loki_write_batch_retries_total` (27 series) and `loki_write_dropped_entries_total` (108 series, every reason including `rate_limited` and `stream_limited`) did not rise, and 2,218,035 entries were sent. `loki_discarded_samples_total` has no series in the last 14 days. Loki's `/config` reads `ingestion_rate_mb: 8`, `ingestion_burst_size_mb: 16`, `per_stream_rate_limit: 3MB`, `per_stream_rate_limit_burst: 15MB`, `max_global_streams_per_user: 5000` and `max_line_size: 256KB`. `LogDeliveryLokiRejected` and `LogDeliveryRetriesPersistent` never went pending.

**Every drop was a deliberate test flood.** Eight pods dropped 152,084 lines. That is the same total as the four shipper pods' `loki_process_dropped_lines_total{reason="ratelimit_drop_stage"}` (76,481 + 32,060 + 29,273 + 14,270), and Prometheus keeps only `app_logs`' drop counters. All eight belong to bex-canary QA fixtures in `tea-daif693dqjvc73e7as3g`, named in [w8/342](../../../w8/done/342.md) and [w8/340](../../../w8/done/340.md):

| Pod | Flood | Dropped | Counter rose (UTC) | Bucket refill |
| --- | --- | --- | --- | --- |
| `qa-20261119-617d-py-68c54d84-74lxh` | cli-qa routine's run-pod flood, 20:08:21–20:12:21, about 190 lines/s (w8/342) | 14,516 | 11-19 20:10–20:13 | wall clock |
| `bld-…-qa-20261119-617d-py-gen-5-xlk5j` | w8/340 case A's build: 20,000 lines in about 1 s | 5,647 | 20:19 | wall clock |
| `bld-…-qa-20261119-617d-py-gen-7-j4rjl` | w8/340 case C's build: case B, then a failing `exit 1` | 26,413 | 20:25 | wall clock |
| `bld-…-qa-20261119-617d-py-gen-9-49ftb` | w8/340 case B's build: 52,500 lines over 35 s | 22,897 | 20:35–20:36 | wall clock |
| `bld-…-qa-20261119-340r-py-gen-3-8slc5` | w8/340's closing check, case A | 12,088 | 22:34 | wall clock |
| `bld-…-qa-20261119-340r-py-gen-5-bvqbx` | w8/340's closing check, case B | 41,496 | 22:37–22:38 | wall clock |
| `qa-20261120-341-py-767658f56b-jjgxc` | w8/341's replay, 04:35:00.7–04:39:00.0, 44,580 lines (w8/342) | 14,757 | 11-20 04:37–04:39 | wall clock |
| `qa-20261120-525-py-7f45b555b8-ggbl6` | w8/m525 t005's replay, 08:58:23.8–09:02:23.1, 44,180 lines (w8/342) | 14,270 | 09:00–09:02 | line timestamp |

- **The counter agrees with history.** w8/341's replay kept 29,823 of 44,580 lines in history, and 44,580 − 29,823 = 14,757 is exactly its counter. m525's replay kept 29,911 of 44,180, so 14,269 against its counter's 14,270.
- **No customer pod dropped a line.** The busiest stored minute of any other pod was `tianpan-v4-web`'s 5,035 (11-20 16:39 UTC), inside the burst. `beancount-cms-v2`'s build copy reached 4,252, `twenty-crm` 1,006 and `beancount-forum` 916. `twenty-crm`'s `ECONNRESET` loops, the only drop t001 predicted, did not recur in the window.
- **`AppLogLinesRateLimited` went pending on six of the eight pods and never fired.** Each flood ended inside the rule's 15-minute `for`, so the rule flags sustained drops and not a four-minute test.

**The five `bld-…` pods are build pods' `type=app` copy (§ Measurement).** Build pods carry `app.bex.co/app`, so `app_pods` tails their files into namespace `bex-build`, and the cap thinned only that copy. No tenant read selects it: `lokiSelectorFor` pins `namespace` to the App's own tenant namespace (`lego/backend/internal/logs/loki.go:476`). Build history comes from `build_logs`, and in the live ConfigMap only `app_logs` and `sandbox_logs` carry `stage.limit` (`request_logs`, `database_logs`, `keyvalue_logs`, `build_logs` and `platform_logs` carry none). w8/340's closing check read the two `340r` builds' history whole, `seq` 1–20,000 and 1–52,500 each once, in the same minutes the copy dropped 12,088 and 41,496. So these drops cost no tenant a line, but they do reach the per-pod counter and `AppLogLinesRateLimited`. A build printing more than 100 lines a second for 15 minutes would fire the info alert under a `bld-…` pod name.

**m525: what changed in the window, and what it did to the read.**

- The rule changed which lines a flooding pod keeps, not how many. Rate 100/s and burst 6,000 did not change. The two near-identical replays on either side of the roll kept 29,823 (wall clock) and 29,911 (line timestamps). Each is about 6,000 plus 100 a second for 240 seconds.
- The roll restarted every shipper. So every bucket started full, and every counter restarted as a new series, which is why the read sums per series. No pod was dropping between 04:39 and 09:00, so the refilled buckets gave no flood an extra burst.
- The roll did no harm under either refill. A wall-clock bucket that catches up a backlog after a restart can drop lines a pod wrote under the cap, while the line-timestamp refill cannot. No non-fixture pod dropped on either side of 06:46.
- **Roll artifact, not a cap finding.** From about 06:47 each old shipper pod reported one unhealthy component until it was replaced, and the new pods never did. This is most likely the config-reloader loading the new ConfigMap, whose `refill_by_entry_timestamp` the old image does not know. So `LogDeliveryCollectorUnhealthy` fired from 06:52 to 06:54 for the last nodes replaced, and `LogDeliveryCollectorMissing` went pending for one minute at 06:48. Across the roll, `alloy_config_last_load_successful` stayed 1, no push failed, retried or dropped, and the shipper kept sending 1,041–2,775 entries a minute.

**Result:** t006's acceptance holds. There were no 429 retries, and no drop went beyond t001's expectation: every drop was a deliberate test flood. Five of the eight are copies of test builds whose build history kept every line.

## Closed (2026-11-20, `/loopx w3`)

- **Definition of done:**
  1. `app_logs` drops and counts each instance's excess on production (§ Production read: the counter equals each replay's missing lines), and request, Postgres and Key Value pipelines carry no limit in the live ConfigMap.
  2. The tail drops the same excess: t005's drill, plus w8/m525 t005's production replay with 0 of 3,590 tail lines missing from history.
  3. The dev-3 flood kept the cap and its neighbor lost nothing (§ dev-3 drill).
  4. Loki's limits are explicit in `/config`, the 429 and drop alerts ship with rule tests (t004), and production shows no 429 and no retry after the pin.
- **Not checked:** a firing `AppLogLinesRateLimited` page on production. No flood lasted 15 minutes, and t004's rule tests cover the firing path.
