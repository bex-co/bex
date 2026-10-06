# w4 · m174 — Fast-exiting App containers intermittently lose stderr lines from their logs

**Worker:** worker4 **Goal:** every line a tenant App container writes, on stdout or stderr, reaches `GET /v1/logs` (and GraphQL/MCP/dashboard Logs), including containers that exit within a second: every cron run and every fast crash. **Status:** blocked

## Tasks (in order)

| id   | title                                                                  | est | depends_on   |
| ---- | ---------------------------------------------------------------------- | --- | ------------ |
| t001 | Pin the mechanism: why the API tailer drops stderr from a fast exit — **DONE**    | 30m | —            |
| t002 | Ship tenant App container logs from the node's CRI files               | 1h  | w4/m174/t001 |
| t003 | Render parity                                                          | 20m | w4/m174/t002 |
| t004 | Simplify — **DONE**                                                               | 15m | w4/m174/t003 |
| t005 | Test coverage — **DONE**                                                          | 45m | w4/m174/t003 |
| t007 | Find the residual one-line loss after the CRI-file switch               | 45m | w4/m174/t002 |
| t006 | Closeout                                                               | 10m | w4/m174/t005, w4/m174/t007 |

## Definition of done

Live, on owned Free fixtures in `bex-canary` (deleted afterwards):

- **Fast-exit cron, all lines, every run.** Create cron `alpine:3` with start command `sh -c 'echo ALPHA-x one; echo BETA-x two; echo "error: GAMMA-x three" 1>&2; echo "{\"level\":\"warn\",\"msg\":\"DELTA-x\"}"; echo 100%-x under_score-x'` and schedule `0 0 1 1 *`. Trigger 6 manual runs about 35 s apart. `GET /v1/logs?ownerId=tea-daif693dqjvc73e7as3g&resource=<id>&limit=100` shows **5 lines per run, 30 total**, including `error: GAMMA-x three` for every run. Today it is missing in 3 of 6 runs.
- **Order and timestamps.** Each run's lines carry the container's own write timestamps (CRI), not the shipper's read time. The stderr line is not reordered after lines written later.
- **Control unchanged.** A cron that sleeps 3 s between writes (today complete: `plain-stderr`, `second-stderr`, `late-stderr` plus stdout all present) stays complete. A long-running web service's live tail, `level` parsing (`{"level":"warn"}` → `warning`, plain lines → `unknown`) and request-log pipeline behave as before.
- **No duplicates.** A service whose container restarts (crash loop) shows each line once per container lifetime, not re-shipped on every tailer reconnect.
- REST, GraphQL `logs`, MCP log tools and the dashboard Logs page return the same line set for the same filters.

## Source + Goal linkage

- **Source:** infinite `/qa-find-bugs` loop44, 2026-10-05 UTC, muse.env, `bex-canary`, pin `c7afefad5`.
  - Fixture `qa-20261005-l44-logs` (`srv-db1qokm7q7bs739ppc50`) ran the DoD command; its stored `startCommand` was verified through GraphQL.
  - Runs at 13:56:19, 14:00:42, 14:01:33, 14:03:43, 14:04:18 and 14:04:53 UTC (all `successful`). `GET /v1/logs …&limit=100` grouped by second returned all 5 lines at 14:00:42, 14:01:33 and 14:04:18. At 13:56:19, 14:03:43 and 14:04:53 it returned only the 4 stdout lines; `error: GAMMA-l44 three` was absent. `&text=GAMMA` was empty after the first run, so this is not a display issue.
  - Every stdout line arrived in every run.
  - Control fixture `qa-20261005-l44-err` (`srv-db1qq1u7q7bs739ppc6g`), with a 3 s sleep between writes, received all 6 lines, 3 of them on stderr. So stderr is collected in general, and a stdout line starting `error:` is kept.
- **Where it lives:**
  - `deploy/gitops/base/log-shipper.yaml:202-205`: tenant App pods ship through `loki.source.kubernetes "app_pods"`, the kubelet-API follow tailer.
  - The same file's header (`:42-56`, w6/m123) already records that this tailer loses lines from containers that exit inside its retry/backoff window. That is why build and pre-deploy pods were moved to **file** tailing (`alloy.mounts.varlog`, `stage.cri`, `:646-688`).
  - App pods were left on the API tailer. Cron Job pods are always short-lived, and fast crashes of web/worker/private services are too, so this is exactly where the error output a user needs is most likely to vanish.
  - The stderr-only pattern (stdout always complete) is **unverified**. Candidates for t001: per-stream copy ordering in the CRI log versus the tailer's resume-by-timestamp, or the tailer starting after part of the file was written.
- **Governing docs:** ADR010 (logs), ADR038 (cron runs), ADR018 log parity.
- **Goal linkage:** ADR008 reliable hosting. Logs are the primary debugging surface, and a failed cron run's stderr is the reason it failed.
- **Expected outcome:** complete per-run logs for cron jobs and crash output for fast-failing services.
- **Why now:** w4/m114 t007 just made cron failures say "exited with status N" and points users at the logs, which then miss the stderr line about half the time.
- **Render parity:** included (t003). The log line set users read on REST/GraphQL/MCP/UI changes, although no API shape changes.
- **Unverified:** web/worker crash-loop containers (reasoned from the shared pipeline, not probed); Loki ingestion as an alternative cause (stdout lines from the same instant arrive, which argues against it); multi-node placement.
- **Severity:** major (silent loss of the error output in a core journey).

## Production replay — 2026-10-06 (qa loop63, pin `e2e4d0d7e` ⊇ `55403e56d`; deploy run 37404121909 finished 03:02:19Z)

Owned Free cron `qa-20261006-l63-cron` (`srv-db269mk20ers738obg40`, `alpine:3`, the DoD command verbatim; deleted afterwards). `GET /v1/logs?ownerId=tea-daif693dqjvc73e7as3g&resource=<id>&limit=100`, grouped by run second:

- **Batch 1** (runs 03:03:27–03:06:23, starting ~1 min after the deploy finished): 29/30 lines. Runs 03:03:30, 03:04:04, 03:04:39, 03:05:50 and 03:06:25 complete; **03:05:14 lost only `error: GAMMA-x three`** (4 lines).
- **Batch 2** (runs 03:08:18–03:11:07): **30/30**, all six runs complete with the stderr line.
- Line order now follows CRI timestamps (the stderr line sometimes precedes `ALPHA`), consistent with the file tailer being active.
- Before the fix (loop44) the same probe lost the stderr line in 3 of 6 runs.
- **Open question for closeout:** the single miss fell ~3 min after the deploy finished. It may be the log-shipper DaemonSet still rolling node by node (a pod scheduled on a node whose Alloy had not yet restarted onto the CRI-file pipeline), or a residual race. Re-run the DoD's 6-run probe once more before closing; if any run misses, check which node ran it and that node's Alloy config generation.

## Production replay 2 — 2026-10-06 (qa loop64, ≥40 min after deploy run 37404121909)

Fresh owned cron `qa-20261006-l64-cron` (`srv-db26u3k20ers738obgfg`, DoD command verbatim; deleted afterwards). Six runs, 03:46:59–03:49:55: **29/30**. Run 03:48:45 has 4 lines and is missing `{"level":"warn","msg":"DELTA-x"}`; `GAMMA` is present this time. `&level=warning`, `&text=DELTA` and a 03:48:40–50Z window each return only five `DELTA` lines, so the line never reached Loki. This rules out "DaemonSet still rolling" for the loop63 miss: a residual drop of about one line in six runs remains, and it is not specific to stderr. Filed as t007; Closeout now depends on it.
