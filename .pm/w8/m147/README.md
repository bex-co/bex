# w8 · m147 — `bex logs --tail` takes every filter Render's subscribe takes

**Worker:** worker8 **Goal:** a live tail accepts the same `level`, `statusCode`, `method`, `host` and `path` filters as Render's `GET /v1/logs/subscribe`, and applies them the way bex's own history query does, instead of answering 400. **Status:** in progress — t001–t007 done 2026-11-10; t008 (live DoD) after the deploy

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A tail with only request filters and no `type` follows request lines, as history does — **DONE** | 45m | — |
| [t002](done/t002.md) | An App tail filters app lines by `level` with the shipper's own parse — **DONE** | 90m | — |
| [t003](done/t003.md) | The dashboard keeps live tail on for the filters the stream now honors — **DONE** | 40m | t001, t002 |
| [t004](done/t004.md) | Blast radius: every subscribe caller and resource kind — **DONE** | 25m | t001, t002 |
| [t005](done/t005.md) | Render parity — **DONE** | 20m | t003, t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 40m | t005 |
| t008 | Closeout (live CLI DoD on production) | 20m | t007 |

## Definition of done

On production, against an owned free web service that produces both app and request lines, each of these runs for 60 s, exits 124 from `timeout`, prints no `Error:` and shows the matching lines live within 10 s of the request:

```sh
timeout 60 bex logs -r <srv-id> --tail --path /mk-<nonce> -o text          # request lines for that path only
timeout 60 bex logs -r <srv-id> --tail --status-code 404 -o text           # request lines with status 404 only
timeout 60 bex logs -r <srv-id> --tail --level error -o text               # app lines whose parsed level is error
timeout 60 bex logs -r <srv-id> --tail --type app --level info -o text     # same, explicit type
```

The history query with the same filters (no `--tail`) returns the same set of lines. `--tail --type predeploy` and other producerless types are still refused with 400, as today. Datastore tails are unchanged.

## Finding

### `bex logs --tail` refuses `--level`, and refuses `--path`/`--status-code`/`--method`/`--host` unless `--type request` is given

- **Severity:** major (the documented live-debugging journey fails for the filters customers reach for first; the history query with the same flags works).
- **Versions:** checkout build `bex vqa-460c283a6` (HEAD `460c283a6`, release ldflags), upstream Render CLI v2.35.0 `24ac2d60fdb2188b57992f0428916c4e426ac0c4`; installed release v0.4.0 (pin v2.29.0) uses the same subscribe request. Deployed revision unknown.
- **Context:** 2026-11-08 ~09:20–09:22Z, `https://api.bex.co/v1/`, workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`), human OAuth token (the workspace owner's own CLI login; the `.env` QA password was rejected by Kratos that day), non-TTY, `-o text`. `/qa-find-bugs-cli` sweep `a6b6`.
- **Repro:** create a free web service (`bex services create --name qa-<nonce>-web --type web_service --image docker.io/traefik/whoami:v1.10 --plan free --env-var WHOAMI_PORT_NUMBER=10000 -o json`), wait for `live`, then:

  ```text
  $ timeout 40 bex logs -r <srv-id> --tail --path /mk-a6b6 -o text
  Error: failed to tail logs (HTTP 400): {"error":"bad request: the live tail reads pod logs, so it cannot filter app or build lines by level/statusCode/method/path/host — query the logs API for those, or subscribe to request logs alone","id":"bad_request","message":"bad request: the live tail reads pod logs, so it cannot filter app or build lines by level/statusCode/method/path/host — query the logs API for those, or subscribe to request logs alone"}
  exit 1

  $ timeout 30 bex logs -r <srv-id> --tail --status-code 200 -o text      → same 400, exit 1
  $ timeout 15 bex logs -r <srv-id> --tail --level info -o text           → same 400, exit 1
  $ timeout 15 bex logs -r <srv-id> --tail --type app --level info -o text → same 400, exit 1

  # controls
  $ timeout 30 bex logs -r <srv-id> --path /mk-a6b6 -o text               → 3 request lines, exit 0 (history, no --type)
  $ timeout 35 bex logs -r <srv-id> --tail --type request --path /mk-a6b6 -o text → request lines stream, exit 124
  ```

  Live latency of an unfiltered tail, measured on the same run on a second fixture: app line 1 s, request line 3 s after the request.

- **Wire evidence:** the pinned client sends `GET /v1/logs/subscribe` (WebSocket) with `resource=<srv-id>&path=/mk-a6b6` (or `level=info`, `statusCode=200`), and no `type`. bex answers `400` with the body quoted above, before the upgrade.
- **Render contract:** the pinned client's `SubscribeLogsParams` (`pkg/client/types_gen.go:4260`) carries `Instance Host StatusCode Method Level Type Text Path`, identical to the list query. Render's API reference for [Subscribe to new logs](https://api-docs.render.com/reference/subscribe-logs) lists the same parameters: `host`, `statusCode`, `method` and `path` are described as "Filter request logs by…", and `level` as "Filter logs by their severity level." Nothing there marks a filter unsupported for streaming. [Render logging](https://render.com/docs/logging) labels method/status/host/path "HTTP request logs only." (Both read 2026-11-08.) Render's authenticated tail output was not replayed.
- **Attribution:** server (bex-api logs service). The client is unchanged upstream.
- **Root cause:** `lego/backend/internal/logs/service.go:445` `liveSubscribable` refuses `hasStoreOnlyFilters() && !onlyRequest()`. It is called pre-slot in `rest.go:185` and again in `FollowLogs` (`service.go:1043`). A query with request filters and no type therefore never reaches the request producer (the w4/m237 store poll, which already honors those filters), although history treats the same filters as request-only. `level` is refused for an App tail because the pod tail has no app-level parse. The JSON/logfmt severity parse exists only in the Alloy pipeline (`deploy/gitops/base/log-shipper.yaml:288-320`), while datastores already filter by level per line (`compileLevelFilter`, `service.go:858`). ADR010 line 188 describes this refusal, but ADR018 records no signed divergence, so parity applies.
- **Fix:** (1) when a tail has request-only filters (`statusCode`/`method`/`host`/`path`) and no explicit type (or type `request`), follow the request producer alone with those filters, exactly as history narrows. An explicit `--type app` together with a request-only filter keeps a clear 400 naming the filter. (2) For an App tail with `level`, parse each pod line's level with a Go port of the shipper's JSON/logfmt rule (same field names, same `unknown` bucket) and keep matches. Pin the port to `log-shipper.yaml` with a drift test, as `TestShipperRegex*` does. Combined `level` + request filters apply each to its own producer (request lines have no level) and must not 400.
- **Blast radius:** `liveSubscribable` has 2 callers (`rest.go:185`, `service.go:1043`). Multi-resource subscribe validates each resource separately. Datastore tails (`followDatastore`) keep their own contract. Build tails (`type=build`) are unaffected. GraphQL and MCP have no live-tail surface. The dashboard switches live off for structured filters (`dashboard/src/features/logs/locales/en.ts:88` "Live tail is off — request-log and structured filters can't be tailed.").
- **Regression checks:** the four DoD commands; `--tail --type predeploy` still 400; `--tail --type build` during a build still streams; an unfiltered tail still streams app and request lines; a datastore `--tail --level` still works.
- **Unverified:** Render's exact live output for these filters; whether Render returns app lines for `--tail --level` on lines without a severity field.
- **Dedupe:** `rg "live tail reads pod logs"` hits only the source. No open or done `.pm` item covers it. w4/m237 added the request-line poll and w5/m139 the datastore level tail; neither changed the App refusal.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop for w8, sweep `a6b6`, 2026-11-08 (user-authorized fixtures in `bex-canary`).
- **Goal linkage:** Render parity first (ADR008), the CLI hosting journey (ADR018 "Request / HTTP logs + structured filters" row, ADR010 log filters).
- **Expected outcome:** a customer debugging a live 500 can `bex logs --tail --status-code 500` or `--level error` and watch it happen, as on Render.
- **Why now:** live tail is the first debugging tool, and the refusal is a hard 400 rather than a degraded result. Both producers it needs already exist (store poll, pod follow), so the change is contained to filter routing plus one parser port.
- **Render parity included:** the change alters `GET /v1/logs/subscribe` and the dashboard live toggle.
