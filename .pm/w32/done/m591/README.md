# w32 · m591 — A failed deploy's one `==> Exited with status N` comes after the start command's first exit, as Render's does

**Worker:** worker32 **Goal:** when a deploy fails because its start command exited, its deploy log (`bex logs --type build`, the interleaved `bex logs`, the live tail, the deploy page, GraphQL and MCP) places its one `==> Exited with status N` and troubleshooting line right after the start command's first exit, with that run's exit status, rather than at the deploy's close after the third run. Later restarts follow it, and the close adds no second verdict. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [The operator records the first non-OOM exit of the release's app container in `status.releaseRollout`](done/t001.md) — **DONE** | 1h15m | — |
| t002 | [The control plane projects the first exit onto the deploy row and into `DeployProgress`](done/t002.md) — **DONE** | 1h | t001 |
| t003 | [Narrate `==> Exited with status N` and the troubleshooting line at the first exit, and verify the live tail](done/t003.md) — **DONE** | 1h15m | t002 |
| t004 | [Record the first-exit timing in the progress-line contract and ADR018](done/t004.md) — **DONE** | 20m | t003 |
| t005 | [Ship and verify on production](done/t005.md) — **DONE** | 45m | t003, t004 |
| t006 | [Render parity](done/t006.md) — **DONE** | 30m | t005 |
| t007 | [Simplify](done/t007.md) — **DONE** | 15m | t006 |
| t008 | [Test coverage](done/t008.md) — **DONE** | 30m | t006 |
| t009 | [Closeout](done/t009.md) — **DONE** | 10m | t007, t008 |

## Definition of done

- On production, once the pin carries this milestone's commits, the released `bex` reads these logs in `bex-canary` on free fixtures:
  - A free native Node web service (`examples/hello-node`) whose start command exits 1 (three runs before settling `update_failed`). In the interleaved deploy log (`bex logs -r <srv>`, a type-less live `--tail` and the dashboard's deploy page merge), run 1's `==> Running '<command>'` and output are followed by `==> Exited with status 1` and `==> Common ways to troubleshoot your deploy: https://render.com/docs/troubleshooting-deploys`, then runs 2 and 3's `==> Running '<command>'` lines and output, with no second verdict at the close. In `bex logs -r <srv> --type build`, the one pair is stamped at run 1's exit (before runs 2 and 3 and before `finished_at`) with nothing after it.
  - A free Dockerfile web service whose command exits 1 reads the same order without the `Running` lines: run 1's output, the one `==> Exited with status 1` and troubleshooting pair, then runs 2 and 3's output, with nothing more at the close.
  - REST `GET /v1/logs`, the subscribe stream, GraphQL `logs`, MCP `list_logs` and the dashboard's deploy page merge return the same lines, IDs and timestamps, keeping Render's ANSI forms (`PlatformFailure` red on `Exited`, `PlatformStep` blue on the troubleshooting line, w32/m576).
  - Every fixture is deleted by ID, and each delete is confirmed by a 404.
- For a pre-w32/m575 native image whose `Running` lines are narrated in `type=build` (`InstanceAnnounces: false`), unit tests show `bex logs -r <srv> --type build` reading `==> Running '<command>'`, `==> Exited with status N` and the troubleshooting line, then the other two `Running` lines, with nothing more at the close.
- The pair is never emitted while the deploy is still `update_in_progress`, and a deploy whose later run succeeds (`live` / `deactivated`) keeps today's lines until `w32/085` settles whether Render fails at the first exit.
- Legacy `update_failed` rows with no recorded first exit, out-of-memory kills, rollout timeouts, build failures and pre-deploy failures keep their existing closing lines.
- Envtest covers the operator's first-exit record, a real-database store test covers its projection onto `deploys`, and unit tests cover `progressLines`, `closingLines`, ordering against nanosecond app output and `AppStartedAt`, and the live tail.
- `docs/render-artifacts/live-deploy-following.md` § Platform progress-line contract and `docs/ADR018-render-parity.md` § Logs, Build logs cite the Render logs below.

## Source + Goal linkage

- **Source:** inbox note [w32/090](../../done/090.md), filed 2026-11-22 from [w32/086](../../done/086.md)'s triage and promoted 2026-11-23 by `/loopx w32`.
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first)) in the deploy log of a failing deploy. On Render the verdict follows the first failing run so the user reads the failure right beside the first traceback; on bex it waits until after the third run, making the exit code read as the last run's.
- **Expected outcome:** a failing bex deploy's log places its single `==> Exited with status N` and troubleshooting pair immediately after the start command's first exit, matching Render's log layout on both native and Docker services.
- **Why now:** w32/m560 added rollout narration, w32/m575 moved native `==> Running '<command>'` into the instance's own `type=app` stream, and w32/m576 added Render's ANSI forms. Moving the verdict pair to the first exit requires a new CRD status field + codegen in `lego/types` & `lego/operator`, a SQL migration + store/reconciler projection in `lego/backend`, narration + live-tail updates in `progress.go`, docs, and a post-deploy production walk in `bex-canary` (~6h15m across 9 tasks).
- **Render parity task included:** the deploy log verdict appears on every tenant-facing log surface (REST, SSE/WebSocket subscribe, GraphQL, MCP, CLI and the dashboard's deploy page).

## Render's placement (public issue logs, w32/086 triage 2026-11-22)

- **DerekSopok/EMS-Kahoot#3** (native Node): `==> Running 'npm start'` at `22:14:58.31`, npm's error, `==> Exited with status 1` and the troubleshooting line at `22:15:02.93`, then `==> Running 'npm start'` at `22:15:06.54`.
- **rkendel1/freq#187 and #191** (Docker): run 1's output, the pair at `17:45:22.21` (status 1) and at `18:22:03.32` (status 2), then run 2's output. In #191 a port-scan line follows at `18:22:15.68`.
- **darthjee/ward#109** (Docker): run 1, the pair at `01:32:22.77`, run 2 (yarn's `Command failed with exit code 1.` at `01:32:28.24`), a port-scan line at `01:32:30.86`, then run 3 from `01:32:49.19`. Neither later run gets a pair.
- **muppetbrown/mana_meeples_singles_market#63** (native Node): the pair at `05:25:58.31`, `==> Running 'node server.js'` again at `05:25:59.10`. **omkarwarik02/DevLens#1** has the pair, then `==> Running 'node src/index.js'`.
- **One exception:** **muppetbrown/mana_meeples_singles_market#84**: run 1's output ends at `18:21:58.24` with no pair, and the pair follows run 2's at `18:22:04.28`.
- **Open question (left to [w32/085](../../blocked/085.md)):** Render may fail the deploy at the first exit and restart the instance after that (its deploy page would then show the later runs because its window runs shortly after `finishedAt`, w32/084). bex holds the deploy open for two restarts (`crashSettleRestarts = 2`, w4/454). This milestone does not block on that question: it records the first exit and places the verdict there once the deploy settles `update_failed`, leaving an eventually-live deploy unchanged.

## bex today (`5aac2dc93b`)

- `ReleaseRolloutStatus` (`lego/types/v1alpha1/app_types.go:1887`) records `Generation`, `StartedAt`, `AppStarts`, `Command`, `WebConcurrency` and `InstanceAnnounces`, with no first-exit record.
- `recordReleaseRollout` (`lego/operator/internal/controller/release_rollout.go:51-62`) calls only `withAppStarts`; `permanentRolloutCrash` (`lego/operator/internal/controller/app_controller.go:4514`) settles after `crashSettleRestarts = 2` (three runs).
- `closingLines` / `rendersVerdict` (`lego/backend/internal/logs/progress.go:902-944`) emit `==> Exited with status N` (`renderlog.PlatformFailure`) and `troubleshootLine` (`renderlog.PlatformStep`) at `d.FinishedAt`, using the last exit (`end_exit_code`).
- Since w32/m575, newly built native images set `InstanceAnnounces: true` and write `==> Running '<command>'` from `native-init` into `type=app`, while older images (`InstanceAnnounces: false`) keep `rolloutLines`'s `type=build` copies. In both cases the interleaved deploy log shows all three runs before the verdict.

## Unblocked (2026-11-23)

**Source:** verified evidence (`/pm unblock w32/m591`, run by the `/loopx w32` worker for this milestone). **Affected:** t005, t006, t009. **Finding:** the gate ``pin must carry `d34935a3c8` `` holds. The newest pin commit on `origin/main` is `cded300699` (`chore(deploy): pin platform images to 9fdf7e0a1171`), and `git merge-base --is-ancestor d34935a3c8 9fdf7e0a1171` succeeds. On the control plane (`167.233.216.116`), `bex-controller-manager` and both `bex-api` pods are ready on `ghcr.io/bex-co/bex-operator@sha256:df6e69bd31bf…` (matching `deploy/gitops/base/bex.yaml`), and `dashboard` is ready on `ghcr.io/bex-co/bex-dashboard@sha256:4b110f45db91…` (matching `deploy/gitops/base/dashboard.yaml`).

**Remaining gates:** none. **Next:** run t005's live walk on two free disposable fixtures in `bex-canary` with the released `bex` v0.9.0 and a human device login (a native Node web service from `examples/hello-node` and a Dockerfile web service from `examples/hello-go`, each exiting 1), check t006's surfaces (REST `GET /v1/logs` and `/v1/logs/subscribe`, GraphQL `logs`, MCP `list_logs`, CLI text/JSON/live `--tail`, and dashboard deploy page `useDeployLogs` merge), delete every created fixture by ID, confirm 404s, and close out (t009).

## Closed (2026-11-23)

The definition of done holds:

- **Production walk.** On pin `9fdf7e0a1171` (`bex-operator@sha256:df6e69bd31bf…`, `bex-dashboard@sha256:4b110f45db91…`), the released `bex` v0.9.0 walked both free disposable fixtures in `bex-canary` ([t005 § Walk](done/t005.md)):
  - Native Node web service `srv-dc28806dfl0s73a7e9bg` (disposable, `examples/hello-node`, `sh -c 'echo run-failed; exit 1'`): on deploy 2 (`dep-dc28aveurl5c7380qplg`, `update_failed`), interleaved history (`--type app --type build`, REST, GraphQL `useDeployLogs` merge, MCP) returned all 10 rollout lines in exact Render order — `Deploying...`, `WEB_CONCURRENCY=1`, run 1's `==> Running 'sh -c 'echo run-failed; exit 1''` (`type=app`, green `Instance` ANSI) + `run-failed`, the single red `==> Exited with status 1` (`PlatformFailure`) + blue troubleshooting line (`PlatformStep`) stamped at run 1's exit (`18:28:42.999999999Z`, 19.4 s before `finishedAt`), then runs 2 and 3's `Running` + `run-failed`, with nothing more at close. `--type build` ended at the verdict pair with zero `Running` lines. Deploy 1 (`dep-dc28806dfl0s73a7e9c0`) verified type-less live `--tail` delivery of all three runs + the single verdict pair at run 1's exit (`18:23:13.999999999Z`) and zero premature emission while `update_in_progress`.
  - Dockerfile web service `srv-dc2880edfl0s73a7e9dg` (disposable, `examples/hello-go`, `dockerCommand: "sh -c 'echo docker-failed; exit 1'"`): on deploy 2 (`dep-dc289f6url5c7380qpd0`, `update_failed`), interleaved history returned all 7 rollout lines in exact Render order with zero `Running` lines — `Deploying...`, `WEB_CONCURRENCY=1`, run 1 `docker-failed`, the single red `==> Exited with status 1` + blue troubleshooting pair at run 1's exit (`18:25:18.999999999Z`, 19.7 s before `finishedAt`), then runs 2 and 3 `docker-failed`, with nothing more at close; live `--tail` delivered the exact verdict pair once settled and zero while `update_in_progress`.
  - Both disposable fixtures (`srv-dc28806dfl0s73a7e9bg` and `srv-dc2880edfl0s73a7e9dg`, disposable) were deleted by ID and answered `HTTP 404` on `GET /v1/services/<id>`.
- **Surfaces.** REST `GET /v1/logs`, `GET /v1/logs/subscribe` (WebSocket `--tail` + SSE `type=build`), GraphQL `logs`, MCP `list_logs`, CLI `bex logs` (`-o text`, `-o json`, `--tail`), and the dashboard's `useDeployLogs` merge + `ansi.ts` (verified directly on live GraphQL payloads under Node 24 since this host has no Playwright browser) all returned identical IDs, timestamps, labels, and Render ANSI bytes with zero new drift ([t006](done/t006.md)).
- **Code, tests, and docs.** Operator first-exit recording, migration `0337`, control-plane projection, `progressLines`/live-tail updates, unit/envtest/real-Postgres tests ([t001](done/t001.md)–[t004](done/t004.md), [t007](done/t007.md), [t008](done/t008.md)), and updates to `docs/render-artifacts/live-deploy-following.md` and `docs/ADR018-render-parity.md` shipped in `d34935a3c8`.
