# w32 · m575 — A native service's own log writes `==> Running '<start command>'` each time its instance starts, as Render's does, free spin-ups and restarts included

**Worker:** worker32 **Goal:** the instance of a native service writes `==> Running '<start command>'` to its own output every time it starts the command, as Render's native instances do: at a deploy, after a crash or a restart, and when a free instance wakes after idling. The line reaches every app read (`bex logs -r <srv>`, REST, the subscribe stream, GraphQL, MCP, the service's Logs page) with the instance's own labels. A Dockerfile or prebuilt-image service writes none. A deploy's log shows the line once per start, never twice. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Settle which native launches Render's instance announces, and what else it writes at a start](t001.md) | 45m | — |
| t002 | [native-init writes `==> Running '<command>'` before it starts the command](t002.md) | 1h | t001 |
| t003 | [Record whether a release's image writes the line itself](t003.md) | 1h30m | t002 |
| t004 | [The deploy narration writes no second copy of the instance's line](t004.md) | 1h | t003 |
| t005 | [Record the line in the progress-line contract and ADR018](t005.md) | 20m | t004 |
| t006 | [Ship and verify on production](t006.md) | 1h | t005 |
| t007 | [Render parity](t007.md) | 30m | t006 |
| t008 | [Simplify](t008.md) | 15m | t007 |
| t009 | [Test coverage](t009.md) | 30m | t007 |
| t010 | [Closeout](t010.md) | 10m | t008, t009 |

## Definition of done

- On production, once the pin carries this milestone's commits, the released `bex` reads these logs in `bex-canary`, on services built after the pin:
  - A free native Node web service (`examples/hello-node`, start command `node main.js`). Its deploy writes `==> Running 'node main.js'` once per start of the command, in `bex logs -r <srv>` and on the dashboard's deploy page, with `type=app` and the pod as `instance`. After it spins down for idling, a request that wakes it writes the line again at the woken instance's start. A restart, and a start command that exits after the service went live, write it again at each new start.
  - The same line, with the same labels and id, on REST `GET /v1/logs`, the subscribe stream (`bex logs --tail`), GraphQL `logs`, MCP `list_logs` and the service's Logs page.
  - A free Dockerfile web service and a free prebuilt-image web service write no `Running` line at any start.
  - No reader shows the line twice for one start: the deploy page, the CLI's type-less deploy tail and `bex logs -r <srv>` each show it once.
  - Every fixture is deleted by ID, and each delete is confirmed by a 404.
- A release whose image predates the change (a rollback, a reused build, a service not rebuilt since) keeps w32/m560's narrated line in its deploy log, so no deploy loses the line it shows today.
- Every native launch t001 settles as announced writes the line, and every other launch writes none. A pre-deploy step's line is written once, whichever side writes it.
- The command is quoted as configured, with `$PORT` and other variables unexpanded, as in w32/m560's narration (frayel/devagent#62).
- Nothing new reaches the user's command: no extra environment variable in its `env`, and no rollout of any existing service on the operator upgrade (ADR004 § the drain is adopted on each service's next roll).
- Unit tests cover native-init's line (once per start, before the command's own first output, absent for a launch that does not announce), the narration's choice between its own copy and the instance's (by image, by service type, by launch), and every log surface. An envtest covers the operator's record, and a real-database store test covers its projection when the record reaches the deploy row.
- `docs/render-artifacts/live-deploy-following.md` § Platform progress-line contract and ADR018 § Logs, Build logs list the line with the Render logs it comes from.

## Source + Goal linkage

- **Source:** inbox note [w32/088](../done/088.md), filed 2026-11-22 from w32/085's search of public Render logs, and promoted the same day by `/loopx w32`.
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first)) in a service's application log. On Render a spin-up or a restart is marked in the service's own log by the line that names the command it starts. A bex service's log shows no such line, so a user reading it cannot tell where one instance's run ends and the next begins.
- **Expected outcome:** a bex native service's log reads like Render's at every start: `==> Running '<command>'`, then the command's own output, whether the start came from a deploy, a crash, a restart or a free spin-up.
- **Why now:** w32/m560 writes the line only inside a deploy's `type=build` narration, at the starts the operator recorded while the rollout ran. w32/086 confirmed that the line is the instance's own and separate from the deploy's verdict. The note estimated ~1h for native-init alone, but native-init is baked into each image at build, so an older image never writes the line. The narration must keep its copy for those images and drop it for the new ones, or the deploy page shows the line twice. That takes an operator record, possibly a migration, and the narration besides native-init: about 7h over ten tasks, so it is a milestone.
- **Render parity task included:** the line is a tenant-facing surface (REST, SSE, GraphQL and MCP log reads, the CLI, the dashboard's Logs page and deploy page).

## Render's line (public logs, 2026-11-22)

- **ivanseibel/tretta** `docs/work/reports/assets/T014-public/04b-render-logs.json` at `b0f216c985`: 60 records of Render's log API for a free native Node web service on 2026-10-18, re-read for this triage. `\x1b[32m\x1b[1m==>\x1b(B\x1b[m \x1b[1mRunning 'pnpm --filter @tretta/server start'\x1b(B\x1b[m` comes at 16:09:33.800, 17:17:23.280 and 19:02:34.091, when no deploy ran. Each follows the previous run's `Command failed with signal "SIGTERM"` (16:06:29, 16:26:41, 17:32:18): free spin-downs and spin-ups. It comes again at 19:10:21.132, 24 s after the deploy's `==> Deploying...` (19:09:57.362), with the new instance's output after it.
  - Every `Running` record carries `resource`, `instance` (`srv-…-<suffix>`), `level: info` and `type: app`, and a UUID id, as the app's own output does.
  - The platform's lines (`==> Deploying...`, the `WEB_CONCURRENCY` line, the live line and the URL block) carry `resource` and `type: app` only, with no `instance` or `level` and ids of another shape. The build's lines carry `resource`, `level` and `type: app`.
  - The `\x1b(B\x1b[m` reset is what `tput sgr0` prints: a shell in the instance writes the line. Its green `==>` differs from the platform's blue step marker. The colours are w32/089's.
  - No `Using Node.js version` line comes at an instance's start: the version lines are the build's only, for Node.
- **DerekSopok/EMS-Kahoot#3** (native Node) and **Sherbivs/SensaiOS#2** (a native background worker): the line again after each exit of the command. In SensaiOS#2 it also comes after the worker's live line, so a start after the deploy is over is the instance's too.
- **Bubba898/luchadores-web#5** (2026-02-01, native Bun): each start writes `==> Using Bun version 1.3.4 (default)` and its docs line before `==> Running 'yarn start'`. t001 checks whether other runtimes do.
- **A pre-deploy step** (w32/m572's README): a native step writes `==> Running '<command>'` after `==> Starting pre-deploy: <command>`; a Dockerfile or prebuilt image's step writes none (ClipFarmVB/ClipFarm#173, ganjasan/mapsurvey#72, BerriAI/litellm#9549, briankeane/alloy-client-portal#8). Render's native launcher therefore names whatever command it runs, not only the start command.
- **Not observed:** a cron job run's log and a one-off job's log on a native runtime, and the type a `type=build` read gives the line (w32/085).

## bex today (`81bfb51e07`)

- `native-init` (`lego/operator/cmd/native-init/main.go:68`, `run`) links the secret files, then starts its arguments and supervises them. It writes nothing to stdout, and to stderr only when it cannot start the command.
- It is the ENTRYPOINT of every native service image, copied in at build from bex's own image (`nativeEntrypoint`, `lego/operator/internal/build/nativeinit.go`). An image built before a change to native-init keeps the old binary for as long as the build is kept (5 to 30 builds by plan).
- Its arguments are `["/bin/bash", "-c", <command>]` for a service's start command (the image's CMD, or the container's args, `nativeStartArgs`, `lego/operator/internal/controller/deployment_projection.go:351`) and for a cron run or a one-off run (`cronPodSpec`, `lego/operator/internal/controller/app_controller.go:6255`). A pre-deploy step runs `["/bin/sh", "-c", <command>]` (`preDeployLaunch`, `app_controller.go:8026`).
- The command reaches native-init as configured: `execution.KubernetesLiteral` escapes Kubernetes' `$(VAR)` expansion, and `$PORT` is left to bash.
- The deploy narration writes `==> Running '<command>'` as a `type=build` platform line at each app-container start the operator recorded while the rollout ran, at most ten, none after the close (`rolloutLines`, `lego/backend/internal/logs/progress.go:584`, from `status.releaseRollout.appStarts`, w32/m560). It writes the pre-deploy step's at the step container's start (`preDeployLines`, `progress.go:513`, w32/m572). The pre-deploy pod's own stdout reaches `type=build` reads through Alloy (`LogTypePreDeploy`, `lego/backend/internal/logs/service.go:55`).
- The dashboard's deploy page interleaves the service's `type=app` lines over the deploy's window with its build and pre-deploy lines, and keeps one copy of a record two legs both return (`dashboard/src/features/deploys/hooks/use-deploy-logs.ts`). The CLI's type-less deploy tail carries the app lines too (w32/084). A line the instance writes and a line the narration writes are two records with different ids, so both would show.
