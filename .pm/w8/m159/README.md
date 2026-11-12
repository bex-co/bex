# w8 · m159 — A deploy's build can be followed from the CLI: build logs combine with other types, and a build tail waits for the next build

**Worker:** worker8 **Goal:** `bex logs --type app,build` and `--type request,build` answer like any other type list, `bex logs --tail --type build` stays open until the service's next build and streams it, and the interactive `bex deploys create` ("Trigger a service deploy and stream logs in real time") shows the build it started. **Status:** in progress (2026-11-12: t001–t006 done and shipped; t007, the replay on production, waits for the deploy that carries it. Narrowed: the type-less history default is unchanged, and only the Render CLI's WebSocket tail follows every build; see t003)

## Tasks (in order)

| id   | title                                                                                              | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Log history accepts `build` together with `app` and `request` — **DONE**                                      | 60m | —          |
| t002 | A live tail with `build` in its types follows builds: it waits for the next one and replays none — **DONE**   | 90m | t001       |
| t003 | The type-less read and tail include build lines; the dashboard Logs tab keeps app and request only — **DONE** | 45m | t001, t002 |
| t004 | Render parity — **DONE**                                                                                      | 15m | t003       |
| t005 | Simplify — **DONE**                                                                                           | 15m | t004       |
| t006 | Test coverage — **DONE**                                                                                      | 30m | t004       |
| t007 | Closeout (live replay on production)                                                               | 30m | t006       |

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

## Unverified

- Render's answer to a type-less `GET /v1/logs` or subscription (no page states the default). The recommendation in t003 rests on the deploy page's **All logs** default and on the CLI's deploy stream, whose request has no type.
- Whether Render's CLI deploy stream shows build lines (no Render account in this sweep).
- The three "Unable to connect to logs" reconnects of the type-less tail: they may be the server's idle timeout on a tail with no lines; recheck once build lines flow (t002).
