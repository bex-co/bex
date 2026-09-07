# w4 · m96 — Use the same log message boundary in history and live streaming

**Worker:** worker4 **Goal:** one emitted container log record renders once when historical and live data overlap, while its content and distinct neighboring records remain intact. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Canonicalize container-log framing at the durable read boundary | 50m | — — **DONE** |
| t002 | Verify shared log sources, wire identities, and existing dedupe guarantees | 50m | t001 — **DONE** |
| t003 | Render parity | 20m | t002 — **DONE** |
| t004 | Simplify | 20m | t003 — **DONE** |
| t005 | Test coverage | 35m | t003 — **DONE** |
| t006 | Closeout | 15m | t004, t005 — **DONE** |

## Outcome (2026-09-07)

Shipped. Historical container log records now match the live/fallback reader's message bytes exactly, so one emitted record renders once (not twice on history/live overlap) and derives one stable id across both paths.

- **t001 — one read-boundary normalization.** `parseLokiStreams` (`lego/backend/internal/logs/loki.go`, the single production caller of the durable-history parser) now passes every message through `canonicalHistoryMessage`, which strips the single kubelet transport terminator a historical record retains but `bufio.Scanner`'s `ScanLines` already removed in the live/fallback path — at most one trailing `\n` then one `\r` (LF, CRLF, and ScanLines' EOF `dropCR`). It is deliberately **not** a `TrimSpace`/`TrimRight`: interior bytes, a JSON string's escaped `\n`, and ordinary trailing spaces survive, and a blank record stays blank rather than being dropped.
- **t002 — source-aware blast radius.** Every log-shipper pipeline reads pod stdout through `loki.source.kubernetes`, which keeps the LF (Alloy `parseKubernetesLog`) — so app, request, platform, postgres, and keyvalue records are all canonicalized. The sole exception is `type=build`, tailed from node CRI files via `loki.source.file` + `stage.cri` (already terminator-free); it is left byte-for-byte untouched so a framing we did not establish is never rewritten. `predeploy` is live-only (no Loki stream), already canonical. No frontend change: `mergeLogLines`/`logLineKey` were correct given their inputs — the fix is purely at the producing boundary.
- **ID convergence is automatic.** `render.go`'s `logID` hashes the message, so once the historical bytes match live, the historical/live ids converge for the same record with no `render.go` change; resume cursors (timestamp `id:` frames) and Last-Event-ID semantics are untouched.
- **t003 — parity.** ADR010 records the read-boundary normalization, the `loki.source.kubernetes` vs `stage.cri` (build) distinction, and the id convergence. The Render wire contract is unchanged — same `{id,message,timestamp,labels[]}` object and `{hasMore,nextStartTime,nextEndTime,logs}` envelope, no invented field — so ADR018's logs rows need no change.
- **t004 — simplify.** The normalization lives in exactly one helper at exactly one boundary; no trim calls scattered across the four dashboard viewers.
- **t005 — failure-sensitive tests.** Backend: a `canonicalHistoryMessage` table (LF/CRLF/trailing-spaces/blank/unterminated/lone-CR/interior-LF/JSON-escaped-newline/request/postgres/keyvalue/**build-kept**), a `parseLokiStreams` integration proving three retained-LF records read back as three canonical messages while a build stream keeps its terminator, and a cross-source test asserting the historical entry and the live `parseContainerLogLine` entry produce identical message, timestamp, and `logID`. Frontend: the real cron emission deduping across the GraphQL history row and the SSE frame to one line, plus a regression guard that a still-newlined historical message keys differently and double-renders (proving the fix belongs at the backend, not a frontend trim).

**Verification:** backend `go test ./...` green (61 packages, EXIT 0) incl. the new `internal/logs` cases; backend `golangci-lint` 0 issues + `go vet` clean; dashboard `yarn typecheck` + `yarn lint` clean; dashboard tests green (122 in the logs feature). **t006 live re-probe deferred to the next QA pass** — no cluster access this session, the same close-out constraint recorded for m91–m95; the deployed change should repeat the README's cron create → three-execution history/SSE byte-and-count check (three, not six, with Live off/on and reload) → cancel control → cleanup walk.

## Definition of done

- Create a free image cron that prints `qa-cron-success` once and a timestamp; allow two scheduled executions and trigger one manual execution. Historical GraphQL `logs` returns three marker records, and the Logs page shows **three**, both with Live disabled and enabled. Reload the Live page and toggle Live off/on: it remains three, one for each original pod/timestamp pair.
- Fetch the same bounded history and `GET /v1/logs/subscribe` with `Accept: text/event-stream` and the marker text filter. Each matching original record has equal message bytes (`qa-cron-success`, without the transport line ending), instance, and timestamp in both paths. A newline in a JSON string's data or ordinary trailing spaces is not removed as if it were framing.
- Preserve the cron journey controls: a scheduled and manual run reach successful history; a long manual run canceled through the UI becomes Canceled, with a start marker but no finish marker in logs.
- Delete the QA service; its REST lookup is 404 and it disappears from Overview.

## Source + Goal linkage

- **Source:** continuous live `$qa-find-bugs w4`, pass 6, 2026-09-06 PDT / 2026-09-07 05:22–05:30 UTC. Workspace tian-personal (`tea-da2isimlm39c739m4ofg`), Hobby. Free cron `qa-20260906-cron`, `srv-daf4k5nco25s73fkr3n0`, image `docker.io/library/alpine:3.22`.
- **Goal linkage:** ADR010 historical + live observability and ADR038 cron runs. Users must be able to distinguish duplicate execution from duplicate rendering.
- **Expected outcome:** the durable and kubelet readers honor one record contract; historical/live overlap does not fabricate additional output.
- **Why now:** the same three actual executions render six marker lines on a fresh page. This is a recurrence of the no-duplicate guarantee from `w9/done/053.md`, through a different input mismatch that its timestamp fix cannot remove.
- **Severity:** minor (display duplication). Runs executed successfully and cancellation worked; duplicate job execution or data corruption is not claimed.
- **Render parity:** included because message bytes and REST log IDs may change for historical records. Preserve Render-shaped envelopes and compare the same history/live record across REST, GraphQL, MCP, dashboard and subscribe encodings.

## Reproduction and complete durable probes

Create the fixture with schedule `* * * * *`, command `printf 'qa-cron-success\n'; date -u`. Scheduled runs started 05:23:00 and 05:24:00; the UI-triggered manual run started 05:23:45. All three succeeded (7s, 6s and 5s respectively). Change Schedule through Settings to `0 0 1 1 *` to stop near-term recurring QA work. Open `/services/srv-daf4k5nco25s73fkr3n0/logs` with its default Last hour and Live enabled.

The page renders each marker twice, with the same displayed second and pod suffix. A fresh reload does the same. `getByRole('switch', {name:'Live', exact:true}).click()` disables Live and changes the URL to `?live=0`; the marker count falls to **3**. Enable it again: **6**. The count was measured from `main.innerText.match(/qa-cron-success/g)`, with all 13 resulting small log rows present in the viewport; it is not an extrapolation from an incompletely mounted virtual list.

Authenticated `POST https://api.bex.co/graphql`, `Content-Type: application/json`, exact body:

```json
{
  "query": "{logs(resource:\"srv-daf4k5nco25s73fkr3n0\",type:\"app\",text:\"qa-cron-success\",startTime:\"2026-09-07T05:22:00Z\",endTime:\"2026-09-07T05:26:00Z\",limit:100){timestamp instance message type}}"
}
```

HTTP 200, complete response:

```json
{
  "data": {
    "logs": [
      {
        "instance": "tea-da2isimlm39c739m4ofg-qa-20260906-cron-29812643-lwzhj",
        "message": "qa-cron-success\n",
        "timestamp": "2026-09-07T05:23:04.82478948Z",
        "type": "app"
      },
      {
        "instance": "tea-da2isimlm39c739m4ofg-qa-20260906-cron-run-893be4c9-qqxtw",
        "message": "qa-cron-success\n",
        "timestamp": "2026-09-07T05:23:48.336006295Z",
        "type": "app"
      },
      {
        "instance": "tea-da2isimlm39c739m4ofg-qa-20260906-cron-29812644-c9qqs",
        "message": "qa-cron-success\n",
        "timestamp": "2026-09-07T05:24:03.651365618Z",
        "type": "app"
      }
    ]
  }
}
```

Authenticated GET, `Accept: text/event-stream`:

```text
https://api.bex.co/v1/logs/subscribe?resource=srv-daf4k5nco25s73fkr3n0&type=app&text=qa-cron-success&startTime=2026-09-07T05%3A22%3A00Z
```

HTTP 200, `Content-Type: text/event-stream`, complete received body (stream closed after the completed pods' matching output):

```text
id: 2026-09-07T05:23:04.82478948Z
data: {"id":"tea-da2isimlm39c739m4ofg-qa-20260906-cron-29812643-lwzhj-2026-09-07T05:23:04.82478948Z-c4d1bcd0","message":"qa-cron-success","timestamp":"2026-09-07T05:23:04.82478948Z","labels":[{"name":"type","value":"app"},{"name":"resource","value":"srv-daf4k5nco25s73fkr3n0"},{"name":"instance","value":"tea-da2isimlm39c739m4ofg-qa-20260906-cron-29812643-lwzhj"},{"name":"container","value":"app"}]}

id: 2026-09-07T05:23:48.336006295Z
data: {"id":"tea-da2isimlm39c739m4ofg-qa-20260906-cron-run-893be4c9-qqxtw-2026-09-07T05:23:48.336006295Z-c4d1bcd0","message":"qa-cron-success","timestamp":"2026-09-07T05:23:48.336006295Z","labels":[{"name":"type","value":"app"},{"name":"resource","value":"srv-daf4k5nco25s73fkr3n0"},{"name":"instance","value":"tea-da2isimlm39c739m4ofg-qa-20260906-cron-run-893be4c9-qqxtw"},{"name":"container","value":"app"}]}

id: 2026-09-07T05:24:03.651365618Z
data: {"id":"tea-da2isimlm39c739m4ofg-qa-20260906-cron-29812644-c9qqs-2026-09-07T05:24:03.651365618Z-c4d1bcd0","message":"qa-cron-success","timestamp":"2026-09-07T05:24:03.651365618Z","labels":[{"name":"type","value":"app"},{"name":"resource","value":"srv-daf4k5nco25s73fkr3n0"},{"name":"instance","value":"tea-da2isimlm39c739m4ofg-qa-20260906-cron-29812644-c9qqs"},{"name":"container","value":"app"}]}

```

Both contain three records and identical nanosecond timestamps and complete instance strings. The difference is precisely `message: "qa-cron-success\n"` in history versus `message: "qa-cron-success"` in streaming. An initial fetch without Accept returned NDJSON rather than SSE; it showed the same message difference, but the explicit-Accept repeat above is the actual browser transport control.

Local supplementary artifacts, checked on disk: `.playwright-mcp/qa-20260906-cron-history-only.png` (three markers), `.playwright-mcp/qa-20260906-cron-live-duplicates.png` (six markers), `.playwright-mcp/qa-20260906-cron-network.txt`, `.playwright-mcp/qa-20260906-cron-console.txt`. They are ignored; the durable API probes above carry the finding. Final console capture contains only the intentional cleanup GET 404.

## Root cause and target contract

1. `lego/backend/internal/logs/loki.go:364–382` (`parseLokiStreams`) copies `pair[1]` into `LogEntry.Message` unchanged. Production's stored App record includes its line terminator.
2. Source mechanism verified at the pinned dependency: `deploy/helm-artifacts.lock:16` pins Alloy chart 1.3.1; the downloaded upstream chart's SHA-256 matched the lock and its Chart.yaml names **Alloy v1.11.2**. `deploy/gitops/base/log-shipper.yaml:198–201` uses `loki.source.kubernetes` for App pods. [v1.11.2 kubetail/tailer.go](https://github.com/grafana/alloy/blob/v1.11.2/internal/component/loki/source/kubernetes/kubetail/tailer.go#L269) uses `ReadString('\n')`; `parseKubernetesLog` at :507 removes the timestamp prefix but returns the remaining string, including the terminating LF. Its pipeline attaches labels without stripping that boundary.
3. Live and fallback pod readers use `bufio.Scanner` in `lego/backend/internal/logs/service.go:1199–1203,1253–1260`. Go's `bufio.ScanLines` removes the record delimiter (including a CR immediately before LF). `parseContainerLogLine` at :1280–1298 preserves the resulting payload. Thus the API presents two message byte shapes for one record.
4. Consumer checked: `dashboard/src/features/logs/lib/map.ts:25–31` derives `timestamp-ms|instance|message`; `toLogLine` and `fromRenderLog` preserve the different messages, so `mergeLogLines` (:131–132) correctly sees two different keys. `log-viewer.tsx:152` merges them. The prior Date.parse precision fix remains intact and is irrelevant when timestamps already match exactly. GraphQL's message is a string (`logs/graphql.go:35`), so it can express the correct newline-free payload without schema invention.

**Fix the producing boundary:** make historical container records match the existing live/fallback record payload, excluding only the proven transport terminator. Apply a source/type-aware normalization at the durable read boundary so existing retained logs are corrected too; changing future Alloy ingestion alone leaves the user's Last hour history broken. Do not globally TrimSpace/TrimRight, do not erase embedded data newlines or trailing spaces, and do not remove empty log records. No log-store migration is necessary for a read-boundary correction. Retain the current frontend timestamp normalization and render order.

Define LF/CRLF and unterminated EOF behavior against the actual kubelet source. If a historical source can legitimately carry a terminal newline as message data, distinguish its provenance rather than applying the container rule globally. t002 must settle each neighboring log type before rollout.

## Shared scope and adjacent behavior

- `parseLokiStreams` has **one production caller**, `NewLokiSource`'s read path (`loki.go:72`). It is shared by authorized QueryLogs, not a cron-only adapter. The three query entrypoints are `GET /v1/logs` (`rest.go:50,116`), GraphQL `logs` (`graphql.go:97`), MCP `list_logs` (`mcp.go:133,156`). `GET /v1/logs/subscribe` supports SSE, WebSocket and NDJSON (`rest.go:131–138`); all use the same rendered live entries. WebSocket was source-traced only.
- The flat frontend mapper has **four historical hook consumers**: general log history, deploy logs, PostgreSQL logs, Key Value logs. `mergeLogLines` has **one production consumer**, generic `LogViewer`, mounted on the services logs route. PostgreSQL/Key Value dedicated viewers are history-only, so a duplicate live merge is not claimed there. Backend historical message changes still reach them.
- Cover the resource family explicitly: web/private/worker/cron App container logs; static build/deploy logs with no static runtime App pod; Postgres and Key Value history. Only cron App logs were live-probed here. Build, request, predeploy and platform-origin pipelines may have different framing; no blanket change to all type labels without checking those producers.
- `logs/render.go:72–81` derives IDs from message bytes, so changing historical message normalization changes those opaque IDs; after the fix the historical/live IDs should converge for the same record. Resume cursors use timestamps and must retain their existing Last-Event-ID semantics. Pagination, direction, time/text filters, namespaces and source labels remain authoritative.
- Preserve unauthenticated/forbidden/not-found/log-store-unavailable/timeout behavior before normalization. Do not use message comparison to widen resource visibility or fabricate successful empty history during an error.

## Prior guarantee audit and dedupe

`w9/done/053.md` / `6e35aa9a1` fixed timestamp precision drift; all of its resolution claims were checked against this finding: Date.parse normalization remains present, unparseable timestamps keep the raw key, exact-key replay dedupe remains present, and virtualized ordering was not changed. Its test shape uses equal message strings, so it does not cover the newly measured terminal LF mismatch. Preserve those cases and add this cross-source input pair. The note's then-future backend resume follow-up subsequently shipped in `w6/done/m93`; the measured stream now carries timestamp `id:` frames. `w6/done/m111` adds range lower bounds; this probe uses that supported bound. No new resume/range implementation is requested.

Searched open+done PM for duplicate-log and newline/framing terms, inspected those prior records and all open milestone titles, and fetched origin/main (7d994a115 at filing prep). Targeted history of loki.go, map.ts and log-shipper.yaml contains no message-boundary fix. DO_NOT_DO does not exclude this correctness issue. This is a recurrence/residual of the old visible symptom with an independently verified cause, not a request to repeat its timestamp patch.

## Cleanup and limits

The long manual command printed `qa-cron-cancel-start`, then slept 90s. UI Cancel → Proceed produced a Canceled row (46s); no `qa-cron-cancel-finish` appeared. Run history also retained the three successful executions. Header and API agreed on the changed annual schedule. Cancellation and execution worked; the finding is the rendering duplication.

Deleted `qa-20260906-cron` through the typed-confirm dialog. Authenticated `GET /v1/services/srv-daf4k5nco25s73fkr3n0` returned HTTP 404 with complete body `{"error":"app not found","id":"not_found","message":"app not found"}`. Overview has no QA name and retains pre-existing `tianpan-v4-web`. No pass-6 QA resources remain in the product. Kubernetes Job/PVC cleanup and retained-log expiration were not independently inspected.

Unverified: this run did not exercise every resource/log-type sibling, CRLF/blank/unterminated/large log records, live WebSocket, or a correct normalized deployment. A transient reconnect banner was visible after completed/canceled pods; it is not filed as another cause or proof of duplicate execution. The UI's disabled Trigger Run during active work is already the recorded w5/m60 choice, so no duplicate is filed for it.
