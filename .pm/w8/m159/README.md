# w8 · m159 — A deploy's build can be followed from the CLI: build logs combine with other types, and a build tail waits for the next build

**Worker:** worker8 **Goal:** `bex logs --type app,build` and `--type request,build` answer like any other type list, `bex logs --tail --type build` stays open until the service's next build and streams it, and the interactive `bex deploys create` ("Trigger a service deploy and stream logs in real time") shows the build it started. **Status:** in progress (2026-11-12: t001–t006 and t008 shipped; the sweep-1cee probe on pin `a5be11e4e517` passed DoD 1, 5 and 6 and half of 2–4, which t008 fixes; t007, the full replay, waits for the deploy that carries t008. Narrowed: the type-less history default is unchanged, and only the Render CLI's WebSocket tail follows every build; see t003)

## Tasks (in order)

| id   | title                                                                                              | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Log history accepts `build` together with `app` and `request` — **DONE**                                      | 60m | —          |
| t002 | A live tail with `build` in its types follows builds: it waits for the next one and replays none — **DONE**   | 90m | t001       |
| t003 | The type-less read and tail include build lines; the dashboard Logs tab keeps app and request only — **DONE** | 45m | t001, t002 |
| t004 | Render parity — **DONE**                                                                                      | 15m | t003       |
| t005 | Simplify — **DONE**                                                                                           | 15m | t004       |
| t006 | Test coverage — **DONE**                                                                                      | 30m | t004       |
| [t008](done/t008.md) | A live build tail prints none of the build's own output, only bex's narration and the upload lines, because it attaches to the build pod only once its init containers are over — **DONE** | 60m | t006 |
| [t009](t009.md) | A live build tail narrates in history's order: `==> Building from` before the build command, and the closing verdict before the stream ends | 45m | t008 |
| t007 | Closeout (live replay on production) | 30m | t008, t009 |

## Definition of done

On production, with the released `bex` CLI and a free web service that has deployed at least once:

1. `bex logs -r <srv> --type request,build --start <t> --end <t'> -o text` exits 0 and prints the window's request and build lines in time order; `--type app,build` and `--type app,request,build` the same. REST `GET /v1/logs?…&type=request&type=build` answers 200.
2. `bex logs -r <srv> --tail --type build -o text` started while no build runs stays open (no `close 4000`), prints nothing from earlier deploys, and prints the build lines of a deploy triggered a minute later as they arrive.
3. `bex logs -r <srv> --tail -o text` (no type) prints that deploy's build lines as they arrive, then the new instance's app lines.
4. The interactive `bex deploys create <srv>` (answer the commit prompt with Enter, confirm with `y`) shows the build lines of the deploy it started, not "No logs to show", while it builds.
5. The dashboard service Logs tab shows the same lines as before this milestone (app and request, no build lines); the deploy page's build log is unchanged.
6. `type=predeploy` keeps its own rule (it is not a Render type) or is folded in the same way; whichever, every surface answers the same.
7. Fixtures deleted afterwards.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` w8 loop sweep `53bf`, 2026-11-12 UTC, released `bex v0.5.0` (Render CLI v2.35.0, `24ac2d60fdb2`), `https://api.bex.co/v1/`, production pin `10acf5314735` (rolled 06:38–06:46 UTC; every case below ran after it), workspace `bex-canary`, QA human device login. Fixture: free Node web service `qa-20261112-53bf-commit` (`srv-dbqlpjmrbf4s73esu1k0`), deleted at the end of the sweep.
- **Severity:** major (a documented CLI usage exits 1, and no CLI command can watch a build as it runs). **Attribution:** server (bex-api logs service); the pinned client sends valid requests.
- **Goal linkage:** Render parity for logs (ADR018 "Build logs" and "Logs" rows); the CLI journey "trigger a deploy and watch it".
- **Expected outcome:** a user can watch a build from the terminal the way Render's CLI promises, and a documented `--type` list is never refused.
- **Why now:** the CLI help and Render's API both document `type` as a list that includes `build`, and every way to watch a build live from the CLI is broken today (below).

### Evidence

Counts are over the fixture's whole life, 06:29–07:21 UTC, through REST with the CLI's token (`GET /v1/logs?ownerId=tea-daif693dqjvc73e7as3g&resource=srv-dbqlpjmrbf4s73esu1k0&startTime=…&endTime=…&direction=forward&limit=100`):

```text
(no type)                          200  11 lines, all request   (eight builds ran in the window)
type=request                       200  11 request
type=build                         200  100 build (hasMore true)
type=request&type=build            400  {"error":"bad request: log type \"build\" must be requested on its own","id":"bad_request",…}
type=app&type=request&type=build   400  same
```

The CLI sends the list exactly that way (pinned `pkg/client/client_gen.go` `NewListLogsRequest`: `form` style, `explode=true`, so `--type request,build` becomes `type=request&type=build`):

```text
$ bex logs -r srv-dbqlpjmrbf4s73esu1k0 --type request,build --start 2026-11-12T06:29:00Z --end 2026-11-12T07:21:00Z --limit 100 -o text
Error: error listing logs: received response code 400: bad request: log type "build" must be requested on its own      exit 1
$ bex logs -r srv-dbqlpjmrbf4s73esu1k0 --tail --type app,build -o text
Error: failed to tail logs (HTTP 400): {"error":"bad request: log type \"build\" must be requested on its own",…}      exit 1
```

Two tails started at 07:22:15, then `bex deploys create srv-dbqlpjmrbf4s73esu1k0 --wait` at 07:22:23 (live at 07:24:37); both read until 07:26:05:

```text
bex logs -r <srv> --tail -o text              0 lines; stderr: "Unable to connect to logs. Retrying in 1s." / "Log connection restored." three times
bex logs -r <srv> --tail --type build -o text
  2026-11-12 07:15:27  ==> Build queued                          ← the previous deploy's lines, replayed
  2026-11-12 07:16:09  ==> Building from https://github.com/bex-co/bex@85fe981
  2026-11-12 07:18:10  ==> Your service is live 🎉
  2026-11-12 07:22:15  ==> no running build is available to follow
  Error: websocket: close 4000: no running build is available to follow      exit 1, 8 s before the deploy began
```

The interactive `bex deploys create srv-dbqlpjmrbf4s73esu1k0` (driven in a pseudo-terminal: Enter at the commit prompt, `y` at the confirmation; deploy `dep-dbqmf3orcnqs73dmd11g`, created 07:15:27, live 07:18:10) opened its Logs view and showed, for the whole five minutes:

```text
tea-daif693dqjvc73e7as3g > Create Deploy for qa-20261112-53bf-commit > Logs
                 No logs to show. New log entries that match your search parameters will appear here.
```

The same window read with `--type build` holds that deploy's eight build lines (`==> Build queued` … `==> Your service is live 🎉`). The pinned client's deploy flow opens `TailLogsFlow` (`pkg/tui/flows/log.go:54-60`): the service, `StartTime` now, `Tail` true, no type.

### Expected (Render)

- [List logs](https://api-docs.render.com/reference/list-logs) and [Subscribe to new logs](https://api-docs.render.com/reference/subscribe-logs), parameter `type`, an array: "Filter logs by their type. Types include `app` for application logs, `request` for request logs, and `build` for build logs." Nothing restricts combining them.
- Subscribe: "Open a websocket connection to subscribe to logs matching the provided filters. Logs are streamed in real-time as they are generated." A subscription is a filter over what is generated from now on, so a build tail opened before a build has nothing to close over and replays nothing.
- [CLI reference](https://render.com/docs/cli-reference): `render deploys create` is "Trigger a service deploy and stream logs in real time"; `render logs --type` is "Filter logs by comma-separated log types"; `--tail` is "Stream new logs".
- Render's deploy page is its log explorer scoped to the deploy's window, with an **All logs** default offering **All logs / Application logs / Build logs** (bex's own authenticated walk, `docs/render-artifacts/live-deploy-following.md:56`): the unfiltered view of a deploy includes its build lines. [Logging](https://render.com/docs/logging) says of the service explorer only that "HTTP request logs appear alongside application logs".

### Root cause

- `LogQuery.validate` refuses `build` (and `predeploy`) beside any other type (`lego/backend/internal/logs/service.go:335-361`), because the store selector cannot express the union: `lokiTypeMatcher` (`lego/backend/internal/logs/loki.go:520-546`) selects app and request streams by `container`, and build streams by `type="build"`. w6/m131 (2026-08-28) judged the 400 "a deliberate, clearly-worded validation rule, not a user-facing defect" and removed the union branches; `TestBuildTypeIsNeverCombined` (`loki_test.go:179-197`) pins that. This predates the Render-parity rule of 2026-10-17, which overrides it.
- A type-less read is app and request only (`lokiTypeMatcher`'s `len(q.Types) == 0` branch, "Build is deliberately NOT in this union"), although `LogQuery.wants` (`service.go:423-431`) says an empty list wants every type.
- The live tail gives build its own path only when it is the sole type (`service.go:1188-1190`, `followBuildLogs`); every other tail is `followPods` over the app's pods plus the request poll, with no build producer. `followBuildLogs` answers a subscription with no running build with `ErrBuildNotRunning` (`service.go:74`), a terminal close `4000`, after replaying the latest deploy's synthesized progress lines (`progress.go`).

## Post-release probe (`/qa-find-bugs-cli` sweep 1cee, 2026-11-12): DoD 2–4 half pass

- **Pin:** `a5be11e4e517`, deployed 10:05 UTC; it carries `11b4f7186`. Released `bex v0.5.0`, free Node web service `qa-20261112-1cee-web` (`srv-dbqpg1pn6kgc7383720g`), 10:42–11:13 UTC.
- **DoD 1 passes.** `--type request,build`, `app,build`, `app,request,build`: exit 0, with lines of every asked type in one time order (newest first, as single-type reads). REST `type=request&type=build` answers 200 with both types.
- **DoD 2 half passes.** A `--type build` tail with no build stayed open 7 minutes with no `close 4000` and printed nothing old. Across the next deploy it printed only `==> Build queued`, `==> Building from …`, the three upload lines and `==> Your service is live`: 6 of the 16 build lines the window's history holds. Every line the build's own steps write (`==> Using Node …`, `==> Running build command …`, the `yarn` output) is missing. See t008.
- **DoD 3 and 4 half pass** with the same six lines: the type-less tail interleaves them with the new instance's app lines, and the interactive `bex deploys create` shows them instead of "No logs to show".
- **A failing build** (`… && exit 3`) shows none of its output or its exit line on the live tail. See t008.
- **DoD 5:** GraphQL `logs` with no type returns app and request lines only. The dashboard itself was not opened (no browser in this sweep).
- **DoD 6:** `predeploy` keeps its own rule; REST `type=predeploy&type=build` and `type=app&type=predeploy` answer `400 log type "predeploy" must be requested on its own`.
- **Seen beside it, filed elsewhere:** the type-less tail opens with the instance's earlier app lines (w8/288). The `==> Build failed:` text quotes bex's toolchain steps (w4/m324 t009).

## Unverified

- Render's answer to a type-less `GET /v1/logs` or subscription (no page states the default). The recommendation in t003 rests on the deploy page's **All logs** default and on the CLI's deploy stream, whose request has no type.
- Whether Render's CLI deploy stream shows build lines (no Render account in this sweep).
- The three "Unable to connect to logs" reconnects of the type-less tail: they may be the server's idle timeout on a tail with no lines; recheck once build lines flow (t002).

## Post-release probe — QA loop round 195 (2026-11-12, pin `9a5b95e96210`, which carries t008's `d22cac15a`)

The tail was read over SSE (`GET /v1/logs/subscribe?…&type=build`, `Accept: text/event-stream`), which runs the same `followBuild` as the CLI's WebSocket.

- **t008's first acceptance line holds on this transport.** A tail opened right after `POST …/deploys` printed `==> Using Node 24 (default)`, `==> Running build command 'npm install && echo QA195-BUILD-MARK'...`, npm's output and `QA195-BUILD-MARK` before `==> Uploading build...`, the same lines the history holds.
- **Its third line half holds.** A build command ending `&& exit 3` showed `QA195-FAIL-MARK` and `error: build command … exited with code 3` live. But the stream ended at 13:02:42.6, before the row closed at 13:02:58.9, so the tail never showed `==> Build failed`.
- **New, filed as t009:** `==> Building from …` arrives after `==> Build successful` (or after the error line), though history stamps it before the build command. The narration follower is read only before the pod and after its streams end.
- **Not filed, by design:**
  - With no build running, the SSE form sends the stored narration, then `event: error` `"no running build is available to follow"`, then closes (`ErrBuildNotRunning`). A browser `EventSource` that ignores that event reconnects every ~3 s, and the resume (`Last-Event-ID` → `Since`, inclusive, w6/m93) re-sends the last line each time. The dashboard dedupes by line key (`dashboard/src/features/deploys/hooks/use-deploy-logs.ts:225-227`).
  - A browser WebSocket from `dashboard.bex.co` is refused `403` at the handshake (an origin check). The CLI authenticates with a key.

