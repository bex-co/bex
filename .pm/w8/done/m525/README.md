# w8 · m525 — A live tail opened during a log flood shows the lines history keeps: the shipper and the tail cap each instance with one deterministic bucket

**Worker:** worker8 **Goal:** during a flood over Render's 6,000 lines a minute, a live tail and history keep the same lines of each instance, not only the same number, because the log shipper and bex-api's tail run one bucket that counts each line at its own timestamp. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Alloy patch: `stage.limit` can count each line at its own timestamp, with whole tokens on a fixed clock grid — **DONE** | 1h15m | — |
| [t002](done/t002.md) | bex-api: the tail and the store-less read run the same bucket, checked against one shared table — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Shipper image and pin, and the shipper harness proves Alloy and the tail keep the same lines — **DONE** | 1h | t001, t002 |
| [t004](done/t004.md) | ADR010 and ADR018 describe the shared bucket and the edges that remain — **DONE** | 20m | t003 |
| [t005](done/t005.md) | Live check on production: the w8/341 flood replay — **DONE** | 45m | t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t006, t007 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t008 |

Total: **9 tasks, about 5h25m**.

## Definition of done

1. **Same lines.** On production, a 40-second `bex logs --tail --type app -o json` opened 100 seconds into a flood of about 190 lines a second (the w8/341 repro) shows lines of which at most 2% are missing from `GET /v1/logs` for the same window. Before `a8ef77ce0` it was 47%; after it, 37.7% (below).
2. **One bucket, two places.** The shipper's `app_logs` `stage.limit` (patched, opt-in) and `lineCap` (`lego/backend/internal/logs/linecap.go`) admit a line by the same rule: whole tokens, refilled one per `1/rate` step of the line's own timestamp on a grid fixed to the Unix epoch, capped at the burst, with no refill for a line stamped at or before the bucket's last step. One table of timestamped lines (`lego/backend/internal/logs/testdata/`) drives Alloy's test in the image build and `linecap_test.go`, and `scripts/test_log_shipper.py` floods the pinned image and gets the same admitted lines as the Go cap on the same input.
3. **Nothing else moves.** 100 lines a second and a 6,000-line burst per instance; request, Postgres, Key Value, build and platform lines stay uncapped; the `sandbox_pods` limit and the drop counters (`loki_process_dropped_lines_total`, `…_by_label_total`, `AppLogLinesRateLimited`) are unchanged.
4. **Documented.** ADR010 § Per-instance line cap and ADR018's application-logs row describe the shared bucket. "Where history and the tail can still differ" keeps only the edges that remain (a shipper restart's full buckets, and whatever t001–t003 measure).

## Source + Goal linkage

- **Source:** inbox note [w8/341](../../done/341.md), filed by the `/qa-find-bugs-cli w8` routine (sweep 617d, 2026-11-19). Its first fix, `a8ef77ce0`, shipped the tail's warm-up. Its production replay on 2026-11-20 failed the note's check, and `/loopx w8` promoted it here the same day.
- **Evidence (production, pin `3e53abbe3bc3`, 2026-11-20):**
  - Fixture `qa-20261120-341-py` (`srv-dbvspi529hjs73bcfn2g`, free native Python, bex-canary, deleted). It flooded 44,580 numbered lines from 04:35:00.7 to 04:39:00.0 UTC, in bursts of 20 every 0.1 s. History kept 29,823 (10,300, 7,525, 5,998 and 6,000 a minute).
  - The tail, 04:36:40.4–04:37:20.4, showed 4,008 lines (1,007 / 999 / 999 / 1,003 per 10 s). History for the same window keeps 4,000 (1,002 / 1,003 / 1,002 / 993). The warm-up fixed the count: before it, the tail showed 7,660.
  - The lines differ. 1,513 of the tail's 4,008 (37.7%) are not in history, and 1,505 history lines are not in the tail. Per burst, the tail keeps the first 10 or 11 lines of every burst. History keeps all 20 of one burst and the first 5 of the next, then none of a burst about every eighth one.
- **Cause:** Alloy's `stage.limit` calls `rate.Limiter.Allow()` (`internal/component/loki/process/stages/limit.go:109` at the pinned `091a20eb3eb5`), which refills by the wall clock at the moment the file tailer hands Alloy a batch of lines. The tail's `lineCap` refills by each line's timestamp. So the two keep the same number of lines but different ones, and no tail-side change can reproduce Alloy's batch timing. Fractional float tokens would keep two timestamp-driven buckets out of step for good, so the shared rule uses whole tokens on a fixed grid. Two such buckets fed the same lines converge once a flood has drained both or a quiet spell has filled both. Their token difference is a whole number that never grows, and it shrinks by one each time only one of them admits.
- **Rejected alternative:** tailing App lines from the store, as `pollRequestLogs` does for request lines (`lego/backend/internal/logs/service.go:1963`). It would match history by construction, but it adds the shipper's delay to every App line, ties the live tail to Loki (ADR010 keeps them independent), and leaves the store-less read on the pod path anyway.
- **Reference product:** Render: "Excess lines are dropped and don't appear in the log explorer or in log streams" ([Logging](https://render.com/docs/logging)). Render's live tail during a flood was not observed (no Render account).
- **Goal linkage:** Render parity of logging ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first), [ADR010](../../../../docs/ADR010-observability.md#per-instance-line-cap), [ADR018](../../../../docs/ADR018-render-parity.md)). It completes w3/m506's per-instance cap.
- **Expected outcome:** an owner watching a flooding service live sees only lines that are still there when they scroll back.
- **Why now:** the cap is live on production and w8/341's fix narrowed the gap only in volume. The divergence has no ADR018 sign-off, so under the parity rule it does not stand.
- **Render parity included:** the tail is a tenant-facing surface (REST `/v1/logs/subscribe`, the CLI's `--tail`, the dashboard's live viewer, the project views' merged tail). t006 walks them.
- **Test floods on production:** each flood counts against w3/m506 t006's drop counters. t005 records its pod and window in [w8/342](../../done/342.md), as w8/341's replay did.

## Parked (2026-11-20, `/loopx w8`)

- **Shipped:** `6ad7ebee0` (t001, t002: the carried `limit-entry-timestamp.patch`, `lineCap`'s whole tokens, the shared table) and `1c3ee254f` (t003, t004, t006–t008: the pinned `bex-alloy@sha256:56d02b11e69b`, `refill_by_entry_timestamp` on `app_logs`, the shipper harness, ADR010 and ADR018).
- **Gate:** the log-shipper DaemonSet runs the new image only once production's pin carries `1c3ee254f`. Production last rolled at 04:24 UTC on pin `3e53abbe3bc3`, so the next roll is due from about 10:24 UTC. The gate clears by itself, and nothing is needed from the user.
- **Next:** t005 replays the w8/341 flood on a fresh free native Python fixture in `bex-canary`. It passes when at most 2% of the tail's lines are missing from history (37.7% before), with per-10-second counts of about 1,000. t005 then records the share in ADR010 and ADR018, and the flood's pod and window in w8/342, and deletes the fixture. t009 closes the milestone.


## Unblocked (2026-11-20, `/loopx w8`)

- **Source:** verified evidence. Production's pin `12716d1d7097` (`4e37ec03f`, deploy run 44701654499, `bex-api` rolled at 08:52:36 UTC) carries `1c3ee254f` and `6ad7ebee0`: `git merge-base --is-ancestor` succeeds for both, and neither was in `3e53abbe3bc3`. The gate cleared by itself, and no user decision was needed.
- **Next:** t005's replay, then t009.

## Closed (2026-11-20, `/loopx w8`)

- **t005 passed on production (pin `12716d1d7097`):** a 40-second tail opened 100 s into a w8/341-shaped flood showed 3,590 lines, at about 1,000 every 10 s. **0 of them (0.0%) are missing from history**, and history holds no line the tail lacks. The share was 37.7% on `3e53abbe3bc3` and 47% before `a8ef77ce0`. Each burst kept its first 10 or 11 lines in both views. The fixture was `qa-20261120-525-py` (`srv-dc00lfsd1ruc73b8cec0`, deleted, `GET` 404). Its flood ran from 08:58:23.8 to 09:02:23.1 UTC and is recorded in [w8/342](../../done/342.md). Details are in [t005](done/t005.md).
- **Definition of done:** item 1 is the replay above. Items 2–4 shipped in `6ad7ebee0` and `1c3ee254f`, and ADR010 and ADR018 now carry the measured 0%.
- **Not checked directly:** the log-shipper DaemonSet's image. Without `HCLOUD_TOKEN` on this host there is no `kubectl`. The replay shows the patched limiter at work instead: the wall-clock limiter kept 20-then-5 bursts.
