# Native CLI timestamp replay for w4/m199

Functional CLI sweep128, 2026-10-11 UTC: actual Bex v0.3.2 and unmodified Render v2.27.0 at pin a764810a7682, against Bex in `bex-canary` / `tea-daif693dqjvc73e7as3g` with isolated human OAuth. Source HEAD `d6ebd8d5ae3908da9064f9b61c2b55671e5ab59b` now pins the patched collector image; the actual deployed collector/backend/node revisions were not observed.

One fresh owned Free Python web service, `qa-20261011-66ac1b-logtime` / `srv-db5q2pt13j8s73csrf2g`, was created natively and reached Live. The quiet HTTP handler suppresses access logging; a single actual `/emit/b1` HTTP200 trigger emits three synthetic stdout records. The two larger records are written in two byte pieces 120ms apart. The larger record's first write is 25,046 bytes, exceeding the ordinary CRI partial threshold; physical node CRI chunks were not read in this sweep.

Both actual native `logs --resources <id> --type app --limit 100 --tail --confirm -o json` streams were started before the trigger. Both return every expected message exactly once and untruncated. Each client's non-tail history command returns the same three complete messages exactly once, with equal raw timestamp, ID and resource/instance/container/type attribution:

| Record | UTF-8 bytes | Raw timestamp in both clients and sources |
| --- | --- | --- |
| short | 35 | `2026-10-11T14:50:29.097810395Z` |
| small delayed write | 8,088 | `2026-10-11T14:50:29.218021234Z` |
| larger partial write | 50,092 | `2026-10-11T14:50:29.218102808Z` |

[Complete native streams and exact trigger receipt](native-live.json), [complete historical commands/output and comparisons](history.json), [creation/readiness](fixture.json), [actual quiet program](program.py.txt), [cleanup](cleanup.json). Reproduction helpers: [fixture](fixture-driver.py.txt), [native tails](dual-live-driver.py.txt), [history comparison](history-driver.py.txt). Raw native HTTP/WebSocket frames were not intercepted; native command output is retained rather than claimed to be complete wire captures.

This is a passing native web subset for [existing w4/m199](../../../w4/done/m199/README.md), with no duplicate implementation task and no milestone closeout. Dashboard desktop/mobile overlap, GraphQL/SSE, the actual locked local collector replay, all four CRI pipelines, other hosting/data/build classes, cron/cancel/restart/buffer bounds and the original complete m96 guarantees remain that milestone's verification work. Quiet stdout evidence does not verify the independent mixed stdout/stderr framing fix in w4/m200 or the over-cap mismatch in w4/256. Do not infer the deployed image digest from this behavior.

Both tail processes were intentionally stopped after observation with SIGINT (-2, no forced kill), reaped, and temporary Render config copies removed. Native delete exits0 in2.063s; own service detail and former public runtime return404, the native inventory contains exactly the six baseline IDs, service termination is0 and Postgres/KeyValue usage/termination are0. No owned fixture survives. The isolated human grant remains active for the authorized continuing loop. No product/deployment change or commit/push occurred.

## Scheduled cron replay (sweep132)

At source `7308b037080e55be0b0bc3569611b5248623ef40`, a fresh native Free Python cron fixture `qa-20261011-e1b3ba-crontail` / `srv-db5qcnshj6ks739l0ljg` reaches Live with schedule `* * * * *`. Its quiet command emits the same short/small/partial pattern, tagged by run minute, then exits. No manual run or cancellation is requested.

Both actual native tails remain open across completed-run metadata snapshots, including 15:12:22.756979Z and 15:12:43.676641Z, and receive three record kinds from three distinct instance IDs. For the first two common run batches, each client's six expected messages appears exactly once in both history and live output: **twelve pairs** match complete bytes, raw timestamp, ID and attribution. Sizes per run are35, 8,094 and50,098 bytes. Full output retains the additional third run; acceptance comparison selects the first two. This also supports the existing [w1/m146](../../../w1/done/m146/README.md) idle-tail guarantee through native WebSocket clients. Actual zero-pod state, transport frames, dashboard behavior and deployed revisions remain unobserved.

[Complete native tail events/output](cron-native-live.json), [complete history and comparisons](cron-history.json), [creation plus scheduled-run metadata](cron-fixture.json), [actual program](cron-program.py.txt), [cleanup and reconciliation](cron-cleanup.json). Exact helpers: [fixture](cron-fixture-driver.py.txt), [native observation](cron-native-driver.py.txt), [history comparison](cron-history-driver.py.txt), [cleanup](cron-cleanup-driver.py.txt).

Harness qualification: the initial idle predicate incorrectly required exactly one recorded batch, overlooking valid terminal-run snapshots after the second run. Independently checking the saved timestamps and process-alive receipts establishes the idle gap. The owned observation driver was deliberately interrupted after sufficient evidence; its finally block reaps both native clients with SIGINT -2 and no forced termination. This is not a product failure or an autonomous native exit.

Native deletion acknowledges in1.210s. Initial background teardown is still pending; the bounded reconciliation then returns service404 and scheduled-runs404, inventory retains the six baseline IDs, and quota is6/0 terminating with PG/KV0. All five scheduled child IDs observed before deletion are retained in the cleanup evidence. A cron has no public runtime URL; direct Kubernetes descendant teardown is unobserved. No owned fixture remains, and temporary Render config copies are removed. No duplicate issue or milestone closeout is filed.

Board update: main `7911ab99f9977f837fe87ee29ee901fc6afc5fc4` closes w4/m199 after its separate complete live verification. This hunt did not perform that closeout; the native subset and its original unverified areas above remain accurately qualified.
