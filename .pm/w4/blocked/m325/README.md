# w4 · m325 — A request-log line is Traefik's raw JSON access record (`ServiceName`, `KubernetesIngressName`, `Duration` in nanoseconds, three copies of the request id) where Render shows a readable request summary; ADR018 records the divergence without the user's sign-off

**Worker:** worker4 **Goal:** a request log line reads like Render's on every surface. It is a one-line summary of method, status, host and path, response time in milliseconds, and the request id, in Render's `key=value` shape. bex's ingress attribution fields stay internal. **Status:** blocked — t001–t004, t006–t008 and t010 done (shipped); t005 live replay waits for the next production deploy, then t009 closes

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Capture Render's request-log line format and record the target in ADR018 and ADR010 — **DONE** | 20m | — |
| [t002](done/t002.md) | Project `type=request` lines to the Render summary at read time, keeping the stored JSON for path, host and latency queries — **DONE** | 60m | t001 |
| [t003](done/t003.md) | The Logs tab renders the summary, and the raw record stays reachable only if t001 says Render offers it — **DONE** | 30m | t002 |
| [t004](done/t004.md) | Blast radius: every reader of the request line, `text` search semantics, live tail and the CLI — **DONE** | 30m | t002 |
| [t010](done/t010.md) | A request line's labels follow Render's — `host` and `path` where Render carries them, and a recorded answer for `requestID`, which Render's label enum omits — **DONE** | 30m | t001 |
| [t005](t005.md) | Live verification in `bex-canary` | 20m | t003, t004, t010 |
| [t006](done/t006.md) | Render parity — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 10m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t006 |
| [t009](t009.md) | Closeout | 10m | t007, t008 |

Ten tasks, 255m (4h15m).

**Minor (parity and readability), but on every request line of every web service and static site.** A user reading their traffic sees the edge's internal JSON with a nanosecond duration.

## Finding

Live `/qa-find-bugs w4` loop, round 89, 2026-11-11 08:54–09:00 UTC (`muse.env`, workspace `bex-canary`; production runs the code of `7ca0227aa477`).

Fixture: Free web service `qa-20261111-lg89` (`srv-dbq2q382gues73d7nndg`, `docker.io/mendhak/http-https-echo:35`). Deleted afterwards (DELETE 204, then GET 404).

### Probes

- **Traffic.** `curl https://qa-20261111-lg89.onbex.co/qa89-live-{1,2,3}` → 200. `curl -H 'x-set-response-status-code: 503' …/qa89-err` → 503. `curl -H 'x-set-response-status-code: 404' …/qa89-nf` → 404.
- **REST, from the signed-in page.** `GET /v1/logs?ownerId=tea-daif693dqjvc73e7as3g&resource=srv-dbq2q382gues73d7nndg&type=request&path=/qa89-err&startTime=<now-10m>&endTime=<now>&limit=5` returns one entry:
  - labels `type=request`, `resource=srv-dbq2q382gues73d7nndg`, `method=GET`, `statusCode=503`, `requestID=ad8c2d92-90ac-42a8-8c03-c8b6f8776c79`;
  - `message`:

  ```json
  {"DownstreamStatus":503,"Duration":8384760,"KubernetesIngressName":"tea-daif693dqjvc73e7as3g-qa-20261111-lg89","KubernetesIngressNamespace":"tea-daif693dqjvc73e7as3g","RequestHost":"qa-20261111-lg89.onbex.co","RequestMethod":"GET","RequestPath":"/qa89-err","ServiceName":"tea-daif693dqjvc73e7as3g-tea-daif693dqjvc73e7as3g-qa-20261111-lg89-8080@kubernetes","downstream_Rndr-Id":"ad8c2d92-90ac-42a8-8c03-c8b6f8776c79","level":"info","msg":"","origin_Rndr-Id":"ad8c2d92-90ac-42a8-8c03-c8b6f8776c79","request_Rndr-Id":"ad8c2d92-90ac-42a8-8c03-c8b6f8776c79","time":"2026-11-11T08:55:49Z"}
  ```
- **Dashboard.** `/services/srv-dbq2q382gues73d7nndg/logs?type=request` (Request logs, Live on) renders each line as `12:55:47 AM GET 200 {"DownstreamStatus":200,"Duration":9424492,"KubernetesIngressName":…}`: the method and status chips, then the whole record inline.

### Root cause

- **The line is stored and served verbatim.** Request lines are Traefik's JSON access log, shipped as is into the `type=request` store (ADR010 § Request logs, `docs/ADR010-observability.md:76`). The read path only trims line terminators: `canonicalHistoryMessage` at `lego/backend/internal/logs/loki.go:765-771`. REST, GraphQL, MCP, live tail and the dashboard therefore all render the raw record.
- **The divergence is recorded, but never signed off.** ADR018's "Request / HTTP logs" row (`docs/ADR018-render-parity.md:247`) lists "the request-log message is the raw JSON access line, not a rendered summary" under Divergences, and ADR010:193 does the same. Neither records a user sign-off.
  - w8/176 (done) noted exactly that: "That divergence also has no sign-off; filing it needs a captured Render request-log line to state the target format."
  - Earlier passes (w4/068, w4/126, w4/198) left it as a product decision. The 2026-10-17 parity rule (AGENTS.md, ADR008) says not to park it.
  - w5/m23 t004 ("method/status/path chips … JSON collapsed") is still `status: todo` inside the closed `w5/done/m23`. The chips shipped; the collapse did not.
- **The stored JSON has to stay.** Several readers parse it:
  - the path and host filters (`loki.go:555-566`, `| json request_path="RequestPath", request_host="RequestHost"`);
  - the Loki-backed request metrics and latency (`lego/backend/internal/metrics/lokisource.go:160-164`, `latency_ns="Duration"`);
  - the log shipper's attribution, which reads `ServiceName` and `KubernetesIngressName`.

  So the fix is a read-time projection, not a change to the shipped line.

### Fix (target behavior)

- **t001 captures Render's line first.**
  - What is known: Render's logging docs search example `/responseTimeMS=\d{3}\d+/` (cited in w8/176) shows the message is a `key=value` summary.
  - Which fields: w5/099 trimmed bex to the fields Render documents (method, status code, host, path, response time) plus the request id.
  - Record the captured line and its source in ADR018 and ADR010, replacing the divergence.
- **t002 projects at read time.** For `type=request` lines only, `canonicalHistoryMessage` (or its caller) renders the stored JSON as the captured summary. The response time is in milliseconds, from `Duration` in nanoseconds; `requestID` comes from `Rndr-Id`.
  - **Leave out** `ServiceName`, `KubernetesIngress*`, the duplicate `*_Rndr-Id` keys, `level`, `msg` and `time`.
  - **t002 leaves the labels unchanged.** t010 settles which labels a request line carries.
  - **Fallback.** A line that doesn't parse as an access record is returned as stored, never dropped.
  - **One change covers the readers:** REST, GraphQL, MCP `list_logs` and the live tail all read through this path.
- **t003, the dashboard.** The Logs tab shows the summary text after the method and status chips. No raw JSON is rendered by default.
- **Searching:** the `text` filter keeps matching the stored record today, so `text=responseTimeMS` would find nothing while `text=KubernetesIngressName` would match. t004 picks the semantics: match the projected summary, or document why it can't.

### Blast radius

- **Readers of the stored line:**
  - `logs/loki.go` (history, the path and host filters, `text`);
  - `metrics/lokisource.go` (request count, latency, discovery);
  - the live-tail store poll (w4/m237);
  - the CLI's `bex logs`, which prints the REST message.
- **Producers:** `deploy/gitops/base/values/traefik.values.yaml` (access-log field allowlist, w5/076) and `deploy/gitops/base/log-shipper.yaml` (request pipeline). Neither should change.
- **Static sites' request lines** go through the same path (static-server, w4/m113). t004 checks their summary too.

### Unverified

- Render's exact line, including its field order, quoting and `responseBytes`/`clientIP`/`userAgent`. t001 captures it.
- How GraphQL `logs` shapes request entries. Its argument names differ from REST, and the round-89 GraphQL probe errored on my argument shape, not on the server.
- The CLI's rendering of request lines.

### Render

- Render's request-log lines are `key=value` summaries; its own docs search them with `/responseTimeMS=\d{3}\d+/`.
- Render doesn't expose its load balancer's internal record.

### Dedupe

- **w4/198 and w5/076 (done)** trimmed fields and kept the view unchanged.
- **w5/099 (done)** trimmed the record to Render's documented fields.
- **w8/176 (done)** explicitly left this divergence unfiled pending a capture.
- **w5/m23 t004** is the stale partial attempt.
- No open item covers it: `grep -rln "rendered summary" .pm` finds only done items.
- **Not deploy lag:** `git log 7ca0227aa..origin/main -- lego/backend/internal/logs dashboard/src/features/logs` shows `684391bc9` (w21/044, deploy-page filtering), which doesn't change request-line text.

## Definition of done

Signed in to `bex-canary` after the release pins this build, on an owned Free web service from `docker.io/mendhak/http-https-echo:35` (deleted afterwards):

1. **REST reads a Render-shaped summary.** After `curl -H 'x-set-response-status-code: 503' https://<svc>.onbex.co/qa-err`, `GET /v1/logs?…&type=request&path=/qa-err` returns a `message` in t001's recorded Render format. It carries method `GET`, status `503`, host, path `/qa-err`, response time in ms and the request id. It contains none of `KubernetesIngressName`, `ServiceName`, `Duration` or `_Rndr-Id`. The labels are unchanged.
2. **The dashboard shows the summary.** Logs → Request logs (Live on and off) shows the same text for that line.
3. **Filters and metrics still work.** `path=/qa-err`, `statusCode=503`, `host=<svc>.onbex.co` and the Metrics page's request count and latency for the window are unchanged.
4. **Static sites too.** On an owned static site, a request line reads in the same format.
5. **Labels follow Render (t010, added from loop round 130).** The same `GET /v1/logs?…&type=request&path=/qa-err` returns exactly the request-line label names t010 records from Render, and GraphQL and MCP agree. Today it returns `type`, `resource`, `method`, `statusCode` and `requestID`: no `host` or `path`, though Render's pinned `logLabel.name` enum lists both and omits `requestID`. `requestID`'s status is recorded in ADR010 and ADR018.

## Source + Goal linkage

- **Source:** the finding above (round 89 of the `/qa-find-bugs w4` loop, 2026-11-11). The durable evidence is the probe record; no screenshot was kept.
- **Goal linkage:**
  - [ADR010](../../../docs/ADR010-observability.md) (request logs) and [ADR018](../../../docs/ADR018-render-parity.md) (the "Request / HTTP logs" row).
  - [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first): a divergence stands only with the user's sign-off.
- **Expected outcome:** a user reads their own traffic as Render shows it, and bex's ingress internals stay out of tenant logs.
- **Why now:** every request line on every web service and static site is affected. The divergence has carried no sign-off since w8/176 flagged it on 2026-10-22.
- **Render parity included (t006):** REST, GraphQL, MCP and dashboard output changes.

## Post-release probe (QA loop round 159, 2026-11-12)

Pin `e7a56b07e0d7` (`c5518dc24`, 02:23 UTC) contains `6be581c67`. This is a partial run of the DoD, recorded for t005; t005 stays todo.

- **Fixtures**, both deleted afterwards (DELETE 204, then GET 404):
  - Free image service `qa-20261112-r159-rb` (`srv-dbqidc2140ds739rb7g0`, `traefik/whoami:v1.10.1`);
  - Free Docker service `qa-20261112-r159-ctx` (`srv-dbqidcerbf4s73estt1g`, `examples/hello-node`).
  - Not the DoD's `http-https-echo` image, so the 503 path wasn't sent.
- **DoD 1 passes for 200 lines.** `curl …/qa159-req2`, then `GET /v1/logs?…&type=request` returned:
  - `message` `clientIP="162.224.81.143" requestID="0bf6ae84-01ad-423e-8d6b-0633c1739026" responseTimeMS=3 responseBytes=613 userAgent="curl/8.7.1"`;
  - labels `resource, method=GET, statusCode=200, host=qa-20261112-r159-rb.onbex.co, path=/qa159-req2, level=info, type=request`.
  - That is the format and label order `docs/render-artifacts/request-logs.md` records. `text=KubernetesIngressName` → 0 lines, `text=responseTimeMS` → 1.
- **DoD 2 passes.** Logs → Request logs, with Live off and with Live on, reads `06:42:20 PM GET 200 qa-20261112-r159-rb.onbex.co/qa159-req2 clientIP="…" requestID="…" responseTimeMS=3 responseBytes=613 userAgent="curl/8.7.1"`. A request sent while Live was on appeared within 12 s in the same shape.
- **DoD 3, filters only:**
  - `path=/qa159-req2` → 1;
  - `statusCode=200` → 2;
  - `host=qa-20261112-r159-rb.onbex.co` → 2;
  - `host=nope.onbex.co` → 0.
- **Not run:** a 503 line, the Metrics request count and latency (DoD 3), a static site (DoD 4), and GraphQL/MCP label agreement (DoD 5).

## Post-release probe addendum (QA loop round 162, 2026-11-12): the rest of the DoD except MCP

Same pin. Fixtures, both deleted afterwards (DELETE 204, then GET 404):

- Free web service `qa-20261112-r162-web` (`srv-dbqj28a140ds739rb87g`), `docker.io/mendhak/http-https-echo:35`, `HTTP_PORT=8080`. This is the DoD's image.
- Free static site `qa-20261112-r162-st` (`srv-dbqj28a140ds739rb85g`), `bex-co/bex` `examples/static-site`.

- **DoD 1 passes for the 503 line.** `curl -H 'x-set-response-status-code: 503' …/qa-err`, then `GET /v1/logs?…&type=request&path=/qa-err`:
  - `message` `clientIP="162.224.81.143" requestID="817a36de-…" responseTimeMS=9 responseBytes=982 userAgent="curl/8.7.1"`;
  - labels `resource, method=GET, statusCode=503, host=qa-20261112-r162-web.onbex.co, path=/qa-err, level=error, type=request`.
- **DoD 3:**
  - filters: `statusCode=503` → 2 lines (the app's and the first-deploy activator's); `host=…` → 3;
  - `GET /v1/metrics/http-requests?…&aggregateBy=statusCode` over the window → `200`: 1, `503`: 2, matching the three requests.
  - **Latency observation, not filed:** `GET /v1/metrics/http-latency` returned `values: []` for those first three requests (after more than a minute). After 12 more requests, one point appeared (`03:25:57`, 7.7 ms), while `http-requests` showed two buckets (`03:25=3`, `03:26=12`). This may be the quantile window's alignment rather than a defect, and needs a closer look before anyone files it.
- **DoD 4 passes.** The static site's request lines read the same way, e.g. `clientIP="…" requestID="…" responseTimeMS=49 responseBytes=10 userAgent="curl/8.7.1"` with `statusCode=404`, `path=/qa162-static`, `level=warning`. Whether static request lines should exist at all is w4/585.
- **DoD 5, GraphQL agrees.** `logs(resource, type: "request", path: ["/qa-err"])` → the same `message`, with `method GET`, `statusCode 503`, `host`, `path /qa-err` and `level error`. **Not run:** MCP `list_logs`.
