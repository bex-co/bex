# w3 · m506 — A running instance keeps at most 6,000 application log lines a minute, as on Render, so one flooding service cannot delay or drop every tenant's logs

**Worker:** worker3 **Goal:** the log shipper drops an instance's application lines beyond Render's documented 6,000 a minute, counts what it drops, and Loki's ingestion limits are explicit, so one service printing in a loop costs only its own excess lines. **Status:** blocked — t001–t005 and t007–t010 done and shipped in `e737b2412`; t006 reads production for 24 hours once a production pin carries it (§ Gate)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Measure each instance's line rate on production — **DONE** | 30m | — |
| [t002](done/t002.md) | Alloy: cap `app_logs` at 6,000 lines a minute per pod — **DONE** | 45m | t001 |
| [t003](done/t003.md) | The live tail applies the same cap — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Explicit Loki ingestion limits and alerts on dropped or refused lines — **DONE** | 45m | t001 |
| [t005](done/t005.md) | dev-3 flood drill — **DONE** | 45m | t002, t003, t004 |
| [t006](t006.md) | Ship and verify on production | 30m | t005 |
| [t007](done/t007.md) | ADR010 and ADR018 — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Render parity — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 30m | t008 |
| [t011](t011.md) | Closeout | 10m | t009, t010 |

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
