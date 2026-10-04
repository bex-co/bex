# m151 live acceptance — 2026-10-02 (UTC 2026-10-03T06:34–06:49Z)

**Deployed:** dashboard and bex-api from the GitOps pin `20fb64867` → `1263d12ae` (contains fix `4b6c73230` and the requested `18958459c`); `bex-api` image `ghcr.io/bex-co/bex-operator@sha256:e396cf3585ac…`. **Fixture:** free web service `qa-20261003-logs-m151` / `srv-db0a43oehcmc739j133g` (public `bex-co/bex` `main`, `examples/hello-go`, Go, `go build -o app .`, `./app`, port 3000, auto-deploy off), deploy `dep-db0a43oehcmc739j1340` live 06:38:46Z. 140 sequential GETs `/qa-r12/alpha/001…140` at 06:38:57–06:40:40Z, all 200.

**Browser:** the shared Playwright MCP browser was busy, so the UI checks ran in an independent headless Chromium (Playwright library) with its own QA session, 1440×900, UTC. State was sampled every 250 ms, with a MutationObserver counting first-load skeletons (`role=status` "Loading logs…") and viewport unmounts. Custom and preset ranges were opened through their URL deep links (`range=custom&rangeStart…&rangeEnd…`, `range=7d`), not by clicking the picker.

## Relative Last hour, Live off, text `/qa-r12/alpha/`

| Run | Observation |
| --- | --- |
| A (06:42:06Z) | First page 100 rows (alpha/041–140) with Load older showing. Load older returned 40 rows (alpha/001–040, `hasMore:false`). Reading position `scrollTop 200`, `scrollHeight 3363`, top row alpha/009. Two automatic head refreshes, at 06:42:36Z and 06:43:06Z, moved bounds forward 30 s and each returned the same 100-row head. At 06:43:20Z: scrollTop 200, scrollHeight 3363, top alpha/009, no Load older. Observer: **0 skeletons, 0 viewport unmounts, 0 state changes** over 68 s. Walking the viewport found 140 unique markers, alpha/001–140. The unchanged 3363 px height (same as 100+40 before the tick) shows there were no duplicates. |
| B, fresh page (06:43:24Z) | Same 100+40 and anchor. The refresh at 06:43:54Z had new bounds. Through the pending request and after it settled: 0 skeletons, 0 unmounts, 0 state changes, 140 unique markers. |

(Before the fix, the same sequence dropped to the 100-line head at the bottom with a 659 ms skeleton.)

## Custom, 7 days, text and reload

- Custom `2026-10-03T06:38:30Z`–`06:41:00Z`: 100+40. Reading anchor scrollTop 200 / top alpha/009 stayed fixed from 06:44:25Z to 06:45:30Z, with **no Logs request** in that window. 140 unique markers. Reload kept `type=app`, the text, `live=0`, `range=custom` and both bounds in the URL and controls ("Application logs", "Custom…", "Live tail paused").
- Last 7 days: 100 rows, then Load older returned 40. Scrolling to the top shows alpha/001. 140 unique.
- Text `/qa-r12/alpha/00`: exactly 9 rows (alpha/001–009), no Load older or cap notice. `qa-r12-no-such-marker`: "No matching logs / No logs match these filters."
- Live on, text `/qa-r12/live-`: "Live — streaming new lines". A new `GET /qa-r12/live-002` (200) showed up in the viewport within about 0.1 s.

## API controls (unchanged)

- GraphQL `logs` limit 100: `hasMore:true`, 100 rows alpha/041–140, cursor `nextStartTime 06:38:30Z / nextEndTime 06:39:28.120168363Z`. Page 2: 40 rows alpha/001–040, `hasMore:false`. Union is 140 unique. With limit 500, still 100 rows and `hasMore:true` (cap kept).
- REST `/v1/logs` limit 5, text `/qa-r12/alpha/00`: backward 005–009 then 001–004 (`hasMore` true→false); forward 001–005 then 006–009. Each chain returns 9 rows, 9 unique.
- MCP `list_logs`: backward 5+4, same rows and cursors as REST, `hasMore` true→false. Labels map keys are container/instance/level/service/type, and the text content equals `structuredContent`.

## Build leg

On the deploy page the build log has 100+16 rows. Load older reached `==> Build queued`, which was still reachable after 12 s of first-page polls on the same window. No skeleton appeared.

## Cleanup

Pre-delete identity: App `tea-d98210cbbpdc73dcrkvg-qa-20261003-logs-m151`, UID `0d0fc1e1-4da5-44dd-901f-ac87873bdc67`. `deleteService` 06:48:09Z → `true`. At 06:49:11Z: REST GET 404, name-filtered list `[]`, public URL 404. No matching Apps, Deployments, ReplicaSets, Services, Ingresses, Certificates/Requests, Jobs, Pods, ServiceAccounts, NetworkPolicies, PVCs, Secrets or ConfigMaps in the namespace. No kpack Images/Builds, and no cluster-wide pod/job/secret match. The session was revoked at the end of the run.

**Not live-exercised** (local regressions only, per verification.md and t002/t005): mobile width, other preset timers, window expiry, refresh errors, simultaneous live and paging at a tick, cron/worker/private aliases, both datastore viewers, the static deploy route.
