# Below-cap CRI records duplicate because history uses the final chunk's timestamp

Why: one complete application log record must have one timestamp identity across historical reads and live tail.

**Severity:** major. **Source:** functional `$qa-find-bugs w4`, a177, 2026-10-11 10:35–10:43 UTC. Workspace bex (`tea-d98210cbbpdc73dcrkvg`, Scale), owned **Free** image web service `qa-20261011-a177-log-time` / `srv-db5mcm89n5bc73aqiigg`. The service and session are cleaned up. This milestone ships the filing, not the fix.

## Actual journey and controls

The quiet revision's complete Docker command is preserved in the encoded proof. It runs Python 3.12's HTTP server on port 3000, disables its incidental access logging, and emits exactly three stdout records only when its owned `/emit` path is requested:

- A 47-byte short Unicode line.
- An **8,071-byte** compact JSON message, written in two pieces with a 120 ms delay; its first piece remains below the runtime's CRI partial-line threshold.
- A **50,075-byte** compact JSON message using `"雪🚀β|" * 5000`, also written in two pieces with a 120 ms delay; the first piece is large enough to produce a CRI partial chunk. This whole record is **below** the API's existing 65,536-byte message cap.

The quiet revision reached Live after an ordinary Settings → Edit Docker Command → Save changes confirmation at **10:38:23.633Z**. Its config-change deploy `dep-db5me7o9n5bc73aqiijg` finished Live at **10:38:40.640495Z**, and the public URL returned `qa-a177-r2-ready 0`. There was **one** quiet `/emit` request, HTTP 200 `qa-a177-r2-ready 1`, at **10:39:15.038Z**. It was not a continuously emitting workload.

1. Capture the normal `GET /v1/logs/subscribe` for this owned service, `type=app`, `text=qa-a177-r2`, `textMode=display`, start 10:38:00Z, while requesting `/emit`.
2. Fetch the same bounded historical records through GraphQL and REST. Both historical reads return three entries, `hasMore:false`; the native SSE capture has three corresponding complete frames and zero errors.
3. Compare the complete message strings and instance ids, then the raw timestamps. The larger record's messages are identical valid JSON, untruncated, **50,075 UTF-8 bytes** / **25,075 UTF-16 code units**. Both have SHA-256 `5434b921e930da5ccb31786ac82cd873fb07bdaf37570a38ef9c32509f8f9399`. Only the timestamp differs.
4. Close the extra capture subscription. On a fresh desktop page at 1440×1000, load `/services/<id>/logs?text=qa-a177-r2-partial-1&live=0`: **one full row** at **10:40:39.436Z**. Enable Live: **two identical full rows** at **10:40:40.153Z**.
5. Set 390×844 and **freshly reload** the Live URL. It repeats with **two** mounted rows at **10:41:14.857Z**. Search `qa-a177-r2-small-1`: the smaller delayed-write control has **one** complete row. Document width remains 390.
6. Search the larger marker again and scroll to the row boundary. The screenshot shows the first copy's complete end marker followed by another row with the same visible second and original JSON case prefix. These were two mounted rows, not a virtualized count inference.

| Record | Historical UTC stamp | Live UTC stamp | Bytes equal? | JS millisecond equal? |
| --- | --- | --- | --- | --- |
| short | 10:39:14.751321494Z | 10:39:14.751321494Z | yes, 47 bytes | yes |
| small, delayed | 10:39:14.871722063Z | 10:39:14.871722063Z | yes, 8,071 bytes | yes |
| CRI-partial, delayed | 10:39:14.992657549Z | 10:39:14.872258422Z | yes, 50,075 bytes | **no**, 120.399127 ms apart |

REST and SSE give the partial record the same message hash suffix `a0bddf75` but different raw-stamp ids. **Expected:** a reassembled logical record uses its first CRI chunk's timestamp in history as the pod-log API does, and the complete record renders once. **Actual:** the collector uses the final chunk's stamp; historical and live representations get different keys and render twice.

## Actual dependency, root, and target

**Verified pin, not an inferred version:** `deploy/helm-artifacts.lock:16` locks Alloy chart **1.3.1** with archive SHA-256 `6f81df76a5f4239ec67b1a522262f8b1f0b18a4eaab96d4af579bdca0e190863`; `deploy/gitops/base/log-shipper.yaml:168` selects its OCI digest `sha256:66b76f583ddbe6f63aa10d8542131b0463a9a3d5cc0aa6128b1b215755e0e702`. Pulling and checking that archive through `scripts/helm-artifact.sh pull`, then rendering the actual production values, produced **docker.io/grafana/alloy:v1.11.2**.

**Opened the exact version's implementation:** [Alloy v1.11.2 extensions.go](https://github.com/grafana/alloy/blob/v1.11.2/internal/component/loki/process/stages/extensions.go#L110). The upstream physical file's `cri.Run` is at **110–163** (GitHub/raw source line numbers, not the web renderer's blank-line-collapsed numbering). Its base pipeline parses each chunk's own time before reassembly. On another `P`, **134–142** concatenates the previous and current payload into the current entry and replaces the buffered entry; on the completing `F`, **150–159** concatenates payloads into the current entry and emits it. **Neither merge retains the previous Timestamp.** The final entry therefore carries the final chunk's timestamp. Its config at **23–27** provides only partial-buffer count/size/truncation settings; there is no supported first-timestamp switch to turn on.

**Executed the pinned dependency, not a mirror of the algorithm:** the existing `scripts/test_log_shipper.py:47–138` harness rendered the current **app_logs** pipeline, replaced only its input and sink, and ran the actual pinned image in an owned local container with no network. Input:

```text
2026-10-11T10:35:00.100000001Z stdout P qa-a177-local-first-
2026-10-11T10:35:00.220000002Z stdout F last
2026-10-11T10:35:00.300000003Z stdout F qa-a177-local-control
```

Actual output:

```json
{
  "qa-a177-local-first-last": {
    "app": "web",
    "container": "app",
    "level": "unknown",
    "namespace": "tenant",
    "pod": "web-1",
    "type": "app",
    "__ts": "2026-10-11T10:35:00.220000002Z"
  },
  "qa-a177-local-control": {
    "app": "web",
    "container": "app",
    "level": "unknown",
    "namespace": "tenant",
    "pod": "web-1",
    "type": "app",
    "__ts": "2026-10-11T10:35:00.300000003Z"
  }
}
```

The correct reassembled timestamp is **10:35:00.100000001Z**; the unsplit control is already correct. The harness removed its own container afterward. The complete output is in the encoded proof.

**Pod API comparison:** `lego/backend/internal/logs/podlogs.go:41–70` requests `Timestamps:true, Follow:true`; `service.go:1603–1663` reconstructs newline-delimited messages and retains their timestamp prefix. The [Kubernetes cri-client v0.35.0 source](https://github.com/kubernetes/cri-client/blob/v0.35.0/pkg/logs/logs.go#L246), matching this repo's client dependency family, adds a timestamp only when beginning a logical newline-delimited record (`write:246–254`, `ReadLogs:410–424`). The first CRI partial therefore provides its prefix. This source is a mechanism comparison; **the production kubelet version was not queried or asserted**. The live captured first/final timestamp difference and the real Alloy replay independently establish this finding.

**Consumer:** `dashboard/src/features/logs/lib/map.ts:25–32` keys by parsed millisecond + instance + exact message; both wire mappers use it, and `mergeLogLines:154–179` has **one** production consumer, LogViewer:172. Here message and instance already match. A 120 ms timestamp difference fails the key predicate. Do not reduce precision further, discard message/instance identity, or group by the visible second: that can merge genuinely distinct neighboring records. The old precision-format fix in `w9/done/053` is working as designed.

**Fix target:** retain the **first chunk's timestamp** whenever CRI partial payloads are merged, both while updating an existing `P` buffer and when completing it with `F`. This is collector work; the final timestamp cannot be repaired accurately in `parseLokiStreams` after the first timestamp has already been discarded. Keep the final message bytes, all labels, partial-stream grouping, file positions, caps and cursor precision intact. If preserving extracted time is necessary for a subsequent stage, preserve it with the same first-chunk ownership; these four production pipelines have no later `stage.timestamp` override.

**Repository boundary:** deliver a verified collector artifact and pin it through `deploy/gitops/base/log-shipper.yaml`. Prefer an upstream release proven by this exact replay. Otherwise build the locked Alloy source with the narrow first-timestamp correction, recording source/patch/image identity in a small reproducible collector build under `deploy/log-shipper/` and wiring its ordinary artifact build/publish into `.github/workflows/deploy.yml`. The exact upstream merge sites above specify the change. No existing configuration option or merely newer tag should be claimed to fix this without running the replay. Current upstream `main` was also inspected: its refactored `cri.go` still stores/emits the current entry after concatenation, so a blind upgrade is not the researched fix.

This does **not** propose changing the ADR010 pod-based live-tail architecture, replacing Loki, or widening the frontend equivalence relation. Keep the separate message-cap work in **w4/256** independent: a 50,075-byte record takes the unchanged branch of that cap, so w4/256 cannot fix this timestamp-only duplicate.

## Shared callers, aliases, and adjacent classes

An exhaustive grep found **four** production `stage.cri` sites, all using 200000-byte partial reassembly caps:

- `app_logs:282–284`: web, private, worker and cron App containers.
- `database_logs:578–580`: Postgres.
- `keyvalue_logs:712–714`: Key Value.
- `build_logs:821–823`: service/static build and predeploy Job file streams.

Apply first-timestamp preservation to **all four**, since the same collector stage is the cause. The platform and edge-request pipelines have no `stage.cri` site; keep their non-CRI timestamp rules unchanged. Static has build history but no static runtime container. The direct predeploy API read is already pod-based; the collector's build-tagged Job stream is a separate source and must keep its existing attribution. Only web App logs were live exercised here; only the actual App pipeline was replayed locally. The other three pipelines belong to t002's verification work.

Historical aliases are generic **GET /v1/logs**, GraphQL **logs**, MCP **list_logs**; dedicated Postgres **GET /v1/postgres/{id}/logs**, **databaseLogs**, **get_postgres_logs**; dedicated Key Value **GET /v1/key-value/{id}/logs**, **keyValueLogs**, **get_key_value_logs**. `internal/api/server.go:762,765` has **two** dedicated-source assignments, both through `datastoreLogsAdapter:611–635` → `logSvc.QueryLogs` → the durable reader. `parseLokiStreams` has **one** production caller, `NewLokiSource:87`. Live **GET /v1/logs/subscribe** remains pod-based. The typed datastore nil-seam fallback is separate and is not changed by a collector artifact.

Controls to preserve: single `F` records retain their own timestamps; multiple `P` chunks plus `F` retain the first stamp even across a second boundary; stdout and stderr, different pods and different containers remain distinct buffer keys; out-of-order independent records retain their original write stamps and remain present; count/size limits and overflow flush behavior stay bounded; file-position restart does not re-read everything. Keep existing Postgres unwrapping/classification, KV/app/build labels, stream/filename label drops, 64 KiB API message policy, text filtering and error classes. Old stored records keep their old stamp and age out; do not rewrite existing Loki history or promise a migration dedupes it.

Before history settles the live buffer may show the correct first-stamped row; once the corrected collector history arrives, the matching key must preserve one complete row. This is not solved by hiding the live row while history loads.

## Dedupe and earlier guarantees

Searched open/done board records for first/chunk/partial timestamp, CRI, duplication and message-size terms, scanned open READMEs, and reviewed targeted source history. No current first-timestamp collector fix is already on main **4abcd7d8f**. Anti-goals were re-read; functional log identity does not match an excluded product capability.

- **w4/256:** historical message cap, different cause; keep separate. The a176 cross-millisecond timestamp observation is now independently verified below the cap here.
- **w4/done/m96:** follow-up/regression of its one-logical-record identity guarantee for CRI partial messages. Its terminator fix remains present. Walk its entire DoD: (1) two scheduled plus one manual cron markers/reload/toggle were **not** run in a177; require the original journey during closeout. (2) bounded REST/GraphQL/SSE, instance and exact-message controls were captured, and the partial timestamp mismatch is confirmed; escaped-newline/trailing-space variants were **not** emitted here and remain original regression controls. (3) scheduled/manual success and long-run cancellation were **not** exercised here; require them before declaring the old guarantee restored. (4) own-service delete/API404/inventory absence and session revoke were completed below.
- **Open w4/blocked/m174:** its t007 is residual loss of an actual five-line fast-exit set from Loki, not timestamp identity of one complete record already present in both sources. Its DoD's timestamp rule prevents substituting shipper read time; retain that rule and its restart/no-reshipping guarantees. Do not claim this web pass verifies its six-run fast-exit, delayed-stderr, level, request, crash-loop or multi-surface controls, and do not close that milestone.
- **w5/done/m122:** moved datastore pipelines to the same CRI stage and owns their classification/200000-byte bound; that makes them verification siblings, not evidence they were live broken here.
- **w9/done/053** and **w4/done/m192:** precision-format and pause/chronological merge guarantees remain required controls.

**Excluded first attempt:** the original Python handler's incidental stderr access log interleaved with an unfinished stdout record. That capture is saved in the local ledger as an independent observation, **cause unverified in this filing**, and is omitted from this timestamp proof. The quiet revision suppresses access logging and uses stdout only.

**Render:** [Render's logging guide](https://render.com/docs/logging) describes generated timestamps and historical/live views; [the logs API](https://api-docs.render.com/reference/list-logs) uses timestamp paging. The exact CRI partial reassembly policy was not authenticated against Render. The target is bex's existing pod/live identity contract, not an assertion that Render runs Alloy.

## Complete durable proof

Local screenshot, viewed: `.playwright-mcp/qa-log-timestamp-a177-mobile-boundary.png`, **68,030 bytes**. Local ledger: `.playwright-mcp/qa-log-timestamp-a177-ledger.json`; local real-collector replay: `.playwright-mcp/qa-log-timestamp-a177-local-alloy.json`. They are ignored; the encoded complete probe below survives the handoff.

This is gzip+base64 of UTF-8 compact JSON, decoded **737463 bytes**, SHA-256 `8350c3f00dc41a3d428a2a8322965792239dd40c229c9208d18e85455b9e7f85`. It preserves the quiet command/save receipt, public readiness, actual emission, complete bounded REST and GraphQL request/responses, exact native SSE URL and all three complete frames, desktop/mobile DOM, and complete local Alloy input/output. For the fresh mobile page, its complete **Logs operation** and corresponding complete response are extracted from a batched HTTP exchange; unrelated operations are explicitly omitted. It does not claim that the reduced operation is the original full HTTP batch. No authentication headers/cookies or unrelated billing data are included. Decode equality and hash were checked.

```python
import base64, gzip, hashlib, json
raw = gzip.decompress(base64.b64decode(encoded))
assert len(raw) == 737463
assert hashlib.sha256(raw).hexdigest() == "8350c3f00dc41a3d428a2a8322965792239dd40c229c9208d18e85455b9e7f85"
evidence = json.loads(raw)
# evidence["history"]["reads"]: complete bounded GQL/REST operations
# evidence["history"]["capture"]: complete native SSE URL and frames
# evidence["localAlloyReplay"]: pinned image, input, actual, completeOutput
```

```base64
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
```

## Cleanup

Delete returned HTTP 200 `{"data":{"deleteService":true}}` at **10:42:21.312Z** for the exact owned id. At **10:42:37.504Z**, `GET https://api.bex.co/v1/services/srv-db5mcm89n5bc73aqiigg` returned **404**:

```json
{
  "code": "NOT_FOUND",
  "error": "not found",
  "id": "not_found",
  "message": "not found",
  "params": null
}
```

The owned public URL returned **404**, body `404 page not found\n`, at **10:42:38.051Z**. Inventory contains no owned a177 service and all persistent baseline IDs remain. Another worker's baseline static fixture `srv-db5m5s4f52sc73b2rskg` was also absent after its independent cleanup; this pass never mutated it, and main now contains that worker's m197 closeout. Do not describe the inventories as exactly equal. This pass's logout returned `ok logged-out`; owners API was **401** at **10:43:30.646Z**, followed by zero cookies, one blank page, and removal of its session handles. The local Alloy harness removed its own container.
