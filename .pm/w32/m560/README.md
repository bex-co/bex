# w32 · m560 — A deploy log narrates its rollout as Render's does: `==> Deploying...`, the `WEB_CONCURRENCY` default and `==> Running '<start command>'`

**Worker:** worker32 **Goal:** between a build's end and its verdict, a deploy's log (`bex logs --type build`, the deploy page, the live tail) shows the lines Render writes there, each at a moment the operator recorded. `==> Deploying...` comes when the release's rollout starts. `==> Setting WEB_CONCURRENCY=N by default, based on available CPUs in the instance` follows when its instance gets that default. On a native runtime, `==> Running '<start command>'` comes each time its app container starts. **Status:** todo (t001–t007, t010, t011 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [The operator records each release's rollout start, its app container's starts, its launched command and its `WEB_CONCURRENCY` default](done/t001.md) — **DONE** | 2h | — |
| t002 | [The control plane projects the record onto the deploy row and into `DeployProgress`](done/t002.md) — **DONE** | 1h30m | t001 |
| t003 | [`==> Deploying...` at the rollout's recorded start](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [`==> Setting WEB_CONCURRENCY=N by default, based on available CPUs in the instance` after it](done/t004.md) — **DONE** | 30m | t003 |
| t005 | [`==> Running '<start command>'` at each recorded start of a native runtime's app container](done/t005.md) — **DONE** | 45m | t003 |
| t006 | [The live build tail carries the rollout lines as they are recorded](done/t006.md) — **DONE** | 45m | t003, t004, t005 |
| t007 | [Record the lines in the progress-line contract and ADR018](done/t007.md) — **DONE** | 20m | t003, t004, t005 |
| t008 | [Ship and verify on production](t008.md) | 45m | t006, t007 |
| t009 | [Render parity](t009.md) | 30m | t008 |
| t010 | [Simplify](done/t010.md) — **DONE** | 15m | t009 |
| t011 | [Test coverage](done/t011.md) — **DONE** | 30m | t009 |
| t012 | [Closeout](t012.md) | 10m | t010, t011 |

## Definition of done

- On production, once the pin carries this milestone's commits, the released `bex` reads these logs in `bex-canary`:
  - A free native Node web service with a start command. `bex logs -r <srv> --type build -o text` shows `==> Build successful 🎉`, then `==> Deploying...`, then `==> Setting WEB_CONCURRENCY=1 by default, based on available CPUs in the instance`, then `==> Running '<its start command>'`, then its verdict, in that order.
  - A free Dockerfile web service shows `==> Deploying...` and the `WEB_CONCURRENCY` line, and no `==> Running` line.
  - A free static site shows none of the three.
  - Every fixture is deleted by ID, and each delete is confirmed by a 404.
- Each line is stamped with a moment the operator recorded: the rollout's start, or an app container's start. None is synthesized from the deploy row's created, started or finished stamps. `==> Deploying...` never sorts before the build's own last line.
- Unit tests cover each line's presence and absence by service type (web, private service, background worker, static site, cron job, Dockerfile, prebuilt image) and by outcome (live, `update_failed`, `build_failed`, canceled before and during the rollout). An envtest covers the operator's record, and a real-database store test covers its projection.
- `docs/render-artifacts/live-deploy-following.md` § Platform progress-line contract and ADR018 § Logs, Build logs list the three lines with the Render logs they come from.

## Source + Goal linkage

- **Source:** inbox note [w32/082](../done/082.md), filed 2026-11-22 from routine `cli-qa`'s follow-up (production pin `89642635b9ce`, released `bex` v0.9.0) and promoted the same day by `/loopx w32`.
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first)) in the deploy log, the surface a user reads first when a deploy fails. Render's log names the deploy step and the start command it runs. bex's stays silent for up to a minute.
- **Expected outcome:** a bex deploy log reads like Render's from the build's end to the verdict. The user sees which command ran and when the instance started, before a `==> Exited with status N`.
- **Why now:** w8/393 gave bex Render's `==> Exited with status N` verdict, and w32/081 gave it Render's static-site verdict. The start command those verdicts refer to is still not in the log. The routine's own journey hit the gap on its first native service.
- **Render parity task included:** the lines are a tenant-facing surface (REST, SSE, GraphQL and MCP log reads, the CLI and the dashboard's deploy page).

## Render's lines (triage, 2026-11-22)

The triage read 30 public GitHub issues from 2025-12 to 2026-11 that quote Render deploy logs. They were found by searching for `Setting WEB_CONCURRENCY=1 by default`.

- **Native runtimes.**
  - DerekSopok/EMS-Kahoot#3 (Node): `==> Build successful 🎉` at 22:14:12.19, `==> Deploying...` at 22:14:46.27, the `WEB_CONCURRENCY` line at 22:14:46.37, `==> Running 'npm start'` at 22:14:58.31, then `==> Exited with status 1` and the troubleshooting line at 22:15:02.93. `==> Running 'npm start'` appears again at 22:15:06.54.
  - frayel/devagent#62 (Python): `Build successful` at 09:43:11.57, `Deploying...` at 09:43:12.55, `WEB_CONCURRENCY` at 09:43:12.65, then `==> Running 'uvicorn app.main:app --host 0.0.0.0 --port $PORT'` at 09:43:21.30. `$PORT` is printed unexpanded.
  - The same sequence appears in vincentbellemare90-commits/Mon-bot#1 (`Running 'python bot.py'`, again after `Timed Out`), Jeanajsja/SGSDev#1 (gunicorn, then live), lelewithheart/Pixel-Breach#10 (`Running 'node multiplayer/server.js'`, then live), render-examples/workflow-agents-workshop-ts#1, un-reader/unreader#5 and Bubba898/luchadores-web#5.
- **Dockerfile services.**
  - darthjee/ward#109: BuildKit's `#20 exporting to image`, then `Deploying...`, then `WEB_CONCURRENCY`, then the app's own output, then `Exited with status 1`.
  - Collective-Intelligence-Labs/omnichain-payments-evm#46: `Pushing image to registry...`, `Upload succeeded`, `Deploying...`, `WEB_CONCURRENCY`, then live.
  - shlinkio/shlink#2612, openclaw/openclaw#3754 and unknownsideofme/LATCH#46 match.
  - None of them has a `Running` line. HKUDS/nanobot#6607 has none either, though it sets a Docker Command (`dockerCommand: gateway`): `Deploying...`, `WEB_CONCURRENCY`, `Exited with status 128`.
- **What they show.**
  - `==> Deploying...` opens the rollout of every built service, native or Docker, between 0.1 s and 34 s after the build's last line.
  - The `WEB_CONCURRENCY` line follows about 0.1 s later. [Render's docs](https://render.com/docs/environment-variables) set that default on web and private services created after 2025-12-08, to the plan's CPU count. bex sets it from its own cutoff (`webConcurrencyCutoff`, 2026-10-21, `lego/operator/internal/controller/web_concurrency.go`), so the line follows bex's own rule.
  - `==> Running '<start command>'` appears only on a native runtime. It quotes the command as configured, comes 9–12 s after `Deploying...`, and comes again each time the command starts again.
- **Not observed:**
  - a background worker's or a private service's deploy log, which is assumed to match a web service's without the port scan;
  - a prebuilt image's: Qxad/openlist-for-PaaS#2 opens `==> Starting service...` and then the `WEB_CONCURRENCY` line, but its owner tried both an image and a repository, and it may be a restart;
  - where a pre-deploy command sits;
  - which log `type` Render gives these lines. bex keeps its narration on `type=build`, as `live-deploy-following.md` records.

  t003 to t005 look for a log that settles each of these, and they record what they find.

- **Static sites print none of them** (w32/081: `==> Uploading build...`, then `==> Your site is live 🎉`).

## bex today (`9150e598e`)

- The native build pod writes `==> Build successful 🎉` (`lego/operator/internal/build/build.go:314`). A Dockerfile build ends with BuildKit's own output. Nothing in `lego/` writes `==> Deploying...`, the `WEB_CONCURRENCY` line or `==> Running '…'`. The only copy is a CLI skill's sample log (`lego/cli/internal/skills/pack/skills/bex-debug/references/log-analysis.md:117`).
- `progressLines` (`lego/backend/internal/logs/progress.go:353-431`) narrates only these moments:
  - the deploy row's created, started and finished stamps;
  - the port scan (`port_scan_since` and the columns after it, w8/385);
  - the close.

  No recorded moment marks the rollout's start or an app container's start. `status.buildRun.finishedAt` is the build Job's end in whole seconds, which can sort before the pod's nanosecond-stamped `Build successful` line. `status.releaseStart` is the dispatch.

- On production (routine `cli-qa`, 2026-11-22, pin `89642635b9ce`), a free native Node service whose start command is `true` logged `==> Build successful 🎉` at 09:11:43. Nothing followed for 55 s, then `==> Exited with status 0`.
