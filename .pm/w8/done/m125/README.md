# w8 · m125 — Give every tenant request Render's `Rndr-Id` and a `requestID` in request logs

**Worker:** worker8 **Goal:** Every request to a web service or static site carries an edge-minted `Rndr-Id`. The app receives it, the client gets it back, and the request log entry shows the same value as `requestID`. An app that logs the header can find a request's whole history with `--text <id>`, as Render's logging guide describes. **Status:** done (2026-10-29)

## Tasks (in order)

| id   | title                                                                      | est | depends_on                 |
| ---- | -------------------------------------------------------------------------- | --- | -------------------------- |
| t001 — **DONE** | Edge: the client-identity plugin mints `Rndr-Id` on request and response   | 40m | w8/m123                    |
| t002 — **DONE** | Access log: keep `Rndr-Id` in Traefik's header allowlist                   | 20m | w8/m125/t001               |
| t003 — **DONE** | Logs: request entries expose `requestID` on REST, GraphQL, MCP, dashboard  | 45m | w8/m125/t002               |
| t004 — **DONE** | Render parity: request correlation in ADR018/ADR010                        | 15m | w8/m125/t003               |
| t005 — **DONE** | Simplify the changed code                                                  | 15m | w8/m125/t004               |
| t006 — **DONE** | Test coverage: minted, overwritten, logged and surfaced                    | 30m | w8/m125/t004               |
| t007 | Close out the milestone — **DONE 2026-10-29** | 30m | w8/m125/t005, w8/m125/t006 |

## Definition of done

Live on production after GitOps syncs Traefik and the release pipeline deploys bex-api, on a fresh free echo service deleted by ID with a 404 proof:

- A request gets an `Rndr-Id` response header, and the app receives the same value in `rndr-id`. A client-supplied `Rndr-Id` is replaced.
- `bex logs -r <id> --type request -o json` shows that value as `requestID`. `bex logs -r <id> --text <value>` returns the request line and any app line that logged it.
- A static site behaves the same.

## Source + Goal linkage

- **Source:** promoted from inbox note `w8/148` on 2026-10-20 (`/qa-find-bugs-cli` sweeps 1241 and 1254, [evidence](../../evidence/cli-sweep1254/README.md)). The note's text is kept in [`done/148.md`](../done/148.md).
- **Goal linkage:** [ADR018](../../../../docs/ADR018-render-parity.md) logs rows and [ADR010](../../../../docs/ADR010-observability.md). render.com/docs/logging documents `Rndr-Id` and `requestID` as the request correlation contract.
- **Expected outcome:** migrated apps that log `Rndr-Id` keep working, and support can trace one request through edge and app logs.
- **Why now:** it builds directly on w8/m123's edge plugin, which every request already passes through.

## Design

- **t001:** in `clientipheaders`, always replace the request's `Rndr-Id` with a fresh random id (UUID-shaped, `crypto/rand`, Yaegi-safe) and set it on the response before calling `next`.
- **t002:** `accessLog.fields.headers.names: {Rndr-Id: keep}`, so the JSON line carries `request_Rndr-Id`. Confirm on the pinned Traefik that the access log records the middleware-set value, not the caller's. Update the field-allowlist test.
- **Dashboard (t003):** the request row renders the raw access-log line, which now carries `request_Rndr-Id`, and the Logs tab's text search finds it, so no new chip was added.
- **Limit:** an app that sends its own `Rndr-Id` response header adds a second value after the edge's. `Header.Get` still reads the edge's, and a wrapper that rewrote upstream headers would risk `Flush`/`Hijack` (SSE, websockets) inside Yaegi.
- **t003:** `parseLokiStreams` derives `requestID` from a request line's JSON. It stays a line field, never a Loki label (unbounded). It surfaces as a REST label, GraphQL `requestId` and the MCP entry, and the dashboard request row shows it.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m125
- **结论来源：** 本次复核证据 + 现有规则（loopx：线上验收由 worker 用 `scripts/qa-login.sh` / `bex` CLI 自行完成）。
- **复核结果：** 原卡点「发布流水线部署后再做线上 QA」已不成立。生产当前运行 pin `b33dd43c4428`（2026-10-29 06:51 UTC 由 deploy.yml + Argo 自动发布），本条目在 main 上已合入的修复都包含在内。线上验收不需要用户操作。
- **剩余工作：** 只剩线上验收（待验证，未完成）。部署不等于验收通过。
- **worker 下一步：** loopx 按原 DoD 和上方 BLOCKED 记录里的检查清单在生产执行线上验收：通过则按 done 规则关闭；不通过则修复后复验。fixture 按 ID 删除并留 404 证据。
- **用户门槛：** 无。

## Closeout — 2026-10-29 (`/loopx w8`)

Owned replay on production (pin `479bd3fe243b`), bex-canary, fixture tag `lx9c`: Free web service `srv-dbhhbuqv8vqc73fvn5o0` (`python:3.12-alpine` header-echo server that also logs `rndr-id=<value>`) and Free static site `srv-dbhhbtav8vqc73fvn5k0` (`mdn/beginner-html-site`).

- **Web service:** plain GETs returned `rndr-id` response headers, and the app received the same value each time. A request sending `Rndr-Id: forged-lx9c-123` got a freshly minted `4587130b-fb1e-4ca9-bc27-b09d906e1557` back, and the app received that minted value. `bex logs -r <id> --text 4587130b-…` returned the request line (label `requestID` and `request_/origin_/downstream_Rndr-Id` all equal) and the app's own line; `--text forged-lx9c` returned nothing on either fixture.
- **Static site:** plain and forged requests got fresh `rndr-id` values; `--text <id>` found the request line with that `requestID`.
- **`--type request -o json`:** refused on Hobby bex-canary (`HTTP request logs require the Pro plan or higher`, w8/m138); a read-only `--type request -o json --limit 2` on a pre-existing standard service in the `bex` workspace shows `requestID` equal to `request_Rndr-Id` on every entry.
- Not re-run: GraphQL `requestId`, MCP and the dashboard request row (outside the DoD; t003/t006 tests). Note for w8/m138 t012: on Hobby the default all-types `--text` search still returns request lines; once t012 lands, this check needs a Pro workspace.
- Cleanup: both fixtures deleted by ID, `GET` → 404.
