# w32 · m381 — A command keeps its sandbox awake until it finishes, even after the client disconnects, as on Render

**Worker:** worker32 **Goal:** a command started in a sandbox (CLI `sandboxes exec`, the Render SDK's `exec`, REST, the connect-token run path or MCP `sandbox_exec`) keeps the sandbox active for as long as it runs, whether or not its client is still connected. The sandbox's execution history shows the command as running until it exits, then records its exit code. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design: how bex knows a command is still in progress without its client — **DONE** | 45m | — |
| t002 | The idle sweeper does not suspend a sandbox while a command it started is running; the idle window runs from that command's end — **DONE** | 2h | t001 |
| t003 | Execution history: a command whose client left stays running in the history until it exits, then carries its exit code — **DONE** | 1.5h | t001 |
| t004 | Blast radius: the gateway's 4 h session cap, bex-api and ssh-gateway restarts, MCP's buffered exec, file transfers, agent-session sandboxes — **DONE** | 30m | t002, t003 |
| t005 | Production walk with the release CLI — **DONE** | 45m | t002, t003, t004 |
| t006 | Render parity — **DONE** | 20m | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t006 |
| t009 | Closeout — **DONE** | 10m | t007, t008 |

## Definition of done

On production, with the released `bex` CLI against a new sandbox in `bex-canary` created with `--idle-timeout 300`:

- Start `bex sandboxes exec <sbx> -- bash -c 'sleep 480; date > ~/done'` and kill the CLI after 5 s, with SIGTERM and again with SIGINT. The sandbox stays `running` in `bex sandboxes list --all -o json` for the whole 480 s. `~/done` then exists, and the sandbox suspends only about 300 s after the command ended.
- `GET /v1/sandboxes/<sbx>/execs` shows that execution with no `stoppedAt` while it runs. Once it ends, the record carries `stoppedAt` at its exit and `exitCode: 0`, not the disconnect time with no exit code.
- The control still holds: a connected command longer than the idle timeout keeps the sandbox running, as on 2026-11-13.
- A background process left behind by a command that has finished does not keep the sandbox awake (Render: "Background processes do not keep a sandbox active after the command that started them finishes"). An idle sandbox still suspends on time.
- ADR014 D4's activity-signal paragraph and ADR104's suspend row describe the new rule.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep d91f, 2026-11-13, in `bex-canary` (`tea-daif693dqjvc73e7as3g`), with release `bex` v0.5.0 (Render CLI v2.35.0 pin) against `https://api.bex.co/v1/`, human device login, production pin `8acf1d116941`. The evidence is below.
- **Goal linkage:** pillar 5 sandboxes and [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first). Agents and CI jobs start long commands and stop watching them. The Render SDK times out on 45 s of silence (w32/m284), and laptops sleep.
- **Expected outcome:** a long command survives its client going away, as Render documents. An agent or CI job no longer loses work because a stream dropped.
- **Why now:** this is silent data loss in the most basic sandbox workflow. The process is killed and its output is never written, with no error anywhere, and the execution history says it stopped at the disconnect. w2/m322 (memory and processes across a suspension) would not fix it: Render does not suspend a sandbox with a command in progress at all, so a suspended command stalls even where memory survives.
- **Render parity:** included. The change alters idle suspension for REST, the connect-token run path and MCP exec (GraphQL has no exec), and the execution history on REST, GraphQL and MCP. The dashboard has no sandbox exec surface.

## Evidence (sweep d91f, 2026-11-13 ~03:40–03:56 UTC)

**Render's contract** (fetched 2026-11-13):

- render.com/docs/sandboxes-executing: "Disconnecting your client does not stop an executing command inside the sandbox. This includes interrupting the CLI or canceling a stream request."
- render.com/docs/sandboxes-creating#idle-timeout: "A sandbox is considered idle if no command execution or file transfer is in progress." It adds: "Executing a command or transferring files restarts this timeout." And: "Background processes do _not_ keep a sandbox active after the command that started them finishes."
- ADR014 D4 already quotes the idle rule and says "a command in progress is never idle". That is true only while the client's stream is open.

**bex on production:**

```text
# sbx-dbr8dc4hd5js73auepq0: the command outlives its client (matches Render)
$ timeout 6 bex sandboxes exec sbx-… -- bash -c 'echo start; sleep 25; echo done-term > ~/marker-term'
start                                                     (CLI killed, exit 124)
$ bex sandboxes exec sbx-… -- sh -c 'ps -o pid,args | grep "sleep 25"'
   18 sleep 25                                            (still running)
$ bex sandboxes exec sbx-… -- cat ~/marker-term           (~30 s later)
done-term
# the same with timeout -s INT (Ctrl-C): marker-int written

# sbx-dbr8eekhd5js73auepvg (--timeout 1800 --idle-timeout 300): suspended mid-command
03:42:56 $ timeout 5 bex sandboxes exec sbx-… -- bash -c 'date -u +%T > ~/started; sleep 480; date -u +%T > ~/marker-long'
03:43:01   (CLI killed; afterwards only `sandboxes list --all -o json` reads, which do not touch activity)
03:43:13   running
03:48:31   suspended                                      (the command had ~150 s left)
03:48:38 $ bex sandboxes exec sbx-… -- sh -c 'cat ~/started; cat ~/marker-long; ps -o pid,args | grep "sleep 480"'
03:42:57
cat: can't open '/home/render/marker-long': No such file or directory
no sleep 480 process                                      (resume 5.2 s, exit 0)

# execution history, GET /v1/sandboxes/sbx-…/execs (private read-only probe with the CLI's token)
stream  startedAt 03:42:57.143Z  stoppedAt 03:43:01.623Z  exitCode null   bash -c 'date … sleep 480 …'

# control, same sandbox: a connected 380 s command (longer than the 300 s idle timeout)
03:49:06 $ bex sandboxes exec sbx-… -- bash -c 'sleep 380; echo connected-ok; date -u +%T'
connected-ok
03:55:29                                                  (exit 0; list read `running` throughout)
```

**Root cause (traced at `f4e7accbc`):**

- `lego/backend/internal/sandbox/exec.go:147` `dialGateway` takes `holdActive` (`idle.go:206-240`), which re-touches `sandbox_activity` every minute while the stream is open and once more when it is released.
- `StreamExec` (`exec.go:319-352`) releases the hold and calls `stopExecution` with the sniffed exit code (`nil` if no exit event arrived) as soon as `io.Copy` returns. A client disconnect ends the copy, so bex counts the command as finished at that moment.
- Commands run through Kubernetes `pods/exec` without a TTY (`exec.go:101-122`, the gateway at `sshgateway/sandboxsse/sandboxsse.go:217-240`). Cutting the stream does not signal the process, so it keeps running, as on Render, but nothing in bex observes it any more.
- The idle sweeper (`idle.go:485` `suspendIfIdle`, every 30 s) sees `last_activity_at` older than `idleTimeoutSeconds` and pauses the sandbox. The rootfs commit deletes the pod, which kills the command (ADR042 D5).

**Same mechanism, not exercised:**

- The gateway's 4 h exec session cap (`sandboxsse.go:126`) ends a connected command's stream the same way.
- A bex-api or ssh-gateway rollout cuts open streams.
- MCP `sandbox_exec` (`ExecBuffered`, `exec.go:448-465`) stops reading at the 2 MiB output limit or when its request ends.

**Unverified:**

- Render's execution-history behavior for a disconnected command (`stoppedAt`/`exitCode` while it runs) is not documented. The DoD's history bullet follows from "does not stop an executing command".
- How Render treats a command still running when the sandbox's own runtime timeout expires. It is out of scope here: the sandbox ends as it does today.

## Blast radius — t004 (2026-11-14)

Every way a command's stream can end before the command does is handled the same way: the stream ends without the command's exit, its record stays open, and the registry answers for it.

| Case | Outcome after the change | Evidence |
| --- | --- | --- |
| The client leaves (Ctrl-C, SIGTERM, a dropped connection) | The command keeps the sandbox awake until it ends; the record closes at its own exit with its exit code | `TestADisconnectedCommandKeepsItsSandboxAwakeUntilItEnds`, `TestAStreamWithoutTheCommandsExitLeavesItsRecordOpen` |
| The gateway's 4 h exec session cap (`sandboxsse.go:126`) ends a connected stream | Same as a disconnect: the stream ends without an exit, the command runs on and is registered | The wrapper does not depend on the stream; the cap is unchanged |
| A bex-api rollout cuts the stream | `StreamExec`'s deferred stop never runs, so the record goes stale; the sweeper (open record) and history reads (stale record) probe the registry | `closeStaleExecutions`, `commandsKeepAwake` |
| An ssh-gateway rollout cuts the stream | bex-api sees the stream end without an exit, as on a disconnect | same path |
| MCP `sandbox_exec` stops reading at 2 MiB | `ExecBuffered` leaves the record open when the output is truncated; the command runs on and is registered | `exec.go` `ExecBuffered` |
| File transfers | Unchanged. A transfer whose client leaves is aborted: the upload's `tar -x` reads EOF and its stage is removed, and the download's `tar -c` or `cat` fails on its closed pipe. Its record still closes when the request ends (`file_connect.go`) | m380's transfer tests; no registry entry |
| Agent-session sandboxes | Unchanged: they never idle-suspend, and their commands run through `execShell` without a registry path | `execArgv(command, "", false)`; `TestAgentSessionSandboxesAreNotIdleManaged` |
| Sandboxes created before the change | The wrapper travels in each exec's argv, so every command started after the deploy is registered in any sandbox. A command started before it behaves as before | `execArgv` |
| A read-only root filesystem (an imported image) | `mkdir -p /run/bex-exec` fails, so the command runs unregistered, as before | `TestARegistryThatCannotBeMadeStillRunsTheCommand` |
| A sandbox suspended or terminated while a disconnected command runs | Not probed: its commands died with it, so open records end at their last refresh (history read) or at termination (`StopOpenSandboxExecutions`), as before | `TestASuspendedSandboxsStaleRecordsEndAtTheirLastRefresh` |

Nothing is left uncovered, so no follow-up is filed. The runtime timeout still ends a sandbox with a command running, which the milestone puts out of scope.

## Parked 2026-11-14 (`/loopx w32`): t005 waits on the production deploy

- **Shipped:** `882818b95` (the command registry, the sweeper and history reconciliation, the probe subject, ADR014 D4).
- **Gate:** production deploys are batched: `deploy.yml` deploys a push, or the hourly schedule once the pin is 6 h old. No user decision is needed. Once `git merge-base --is-ancestor 882818b95 <pinned sha>` holds, a worker runs the acceptance walk below with the released CLI in `bex-canary`.
- **ADR104 and ADR018:** their rows read 🚧 w32/m381 t005 until the walk passes; t006 flips them.

## Live check — 2026-11-14 (`/qa-find-bugs-cli` sweep 23)

- **Gate:** production pin `c99e7815ebec` (deployed 13:06 UTC) carries `882818b95`. Release `bex` v0.5.0, human device login, `bex-canary`. Sandbox `sbx-dbs5vn5v5sfs73boajn0` was created with `--wait --timeout 1800 --idle-timeout 300 --network-policy deny-all`.
- **Disconnect with SIGTERM passes:**
  - At 13:19:39 UTC, `timeout -s TERM 5 bex sandboxes exec <sbx> -- bash -c 'sleep 480; date -u +%T > ~/done'` started. The CLI was killed at 13:19:44.
  - Mid-run, REST `GET /v1/sandboxes/<sbx>/execs` listed the execution with `startedAt` and no `stoppedAt` or `exitCode`.
  - `bex sandboxes list --all -o json`, polled every 30 s, read `running` from 13:19:47 through 13:32:26. It read `suspending` at 13:32:57 and `suspended` from 13:33:27, which is 317–347 s after the command ended at 13:27:40.
  - `bex sandboxes exec <sbx> -- cat done` resumed the sandbox in 5.1 s and printed `13:27:40`.
  - The execution record then read `stoppedAt` `2026-11-14T13:27:40Z` and `exitCode: 0`. That `stoppedAt` has whole seconds, while a connected command's has microseconds (`13:35:30.243658Z`). This is noted only; Render's precision is unknown.
- **Disconnect with Ctrl-C, and a leftover background process, pass:**
  - At 13:35:46, `timeout -s INT 5 bex sandboxes exec <sbx> -- bash -c 'nohup sleep 900 >/dev/null 2>&1 & sleep 480; date -u +%T > ~/done2'` started.
  - The sandbox read `running` through 13:48:34 and `suspended` from 13:49:05. That is 318–348 s after the command ended at 13:43:47, while the background `sleep 900` it left behind was still alive (until ~13:50:46). Render: "Background processes do not keep a sandbox active after the command that started them finishes".
  - `cat done2` printed `13:43:47`. The execution record read `stoppedAt` `2026-11-14T13:43:47Z` and `exitCode: 0`.
- **Not run:** the connected-command control from 2026-11-13. Both sandboxes were stopped and read `terminated`.
- **Rechecked on pin `2d9bbae6bb8d`** (2026-11-14 22:48–23:04 UTC, with the rebuilt OpenSandbox controller; sandbox `sbx-dbseaf0rdlgs739t6b10`, since terminated): a CLI killed 5 s into `sleep 330` left the sandbox `running` until the command ended (22:57:25, `exitCode: 0`). It read `suspended` from 23:02:44, about 320 s later, so w9/m407's suspend storage cap did not refuse the idle suspend in `bex-canary`.

## Closed 2026-11-14 (`/loopx w32`)

- **t005:** `/qa-find-bugs-cli` sweep 23 ran every step with the release CLI on pin `c99e7815ebec` and rechecked it on `2d9bbae6bb8d` (the live check section): SIGTERM and SIGINT disconnects, the real `stoppedAt` and `exitCode: 0`, and a leftover background process that did not keep the sandbox up. The connected-command control from 2026-11-13 was not rerun; a connected command never relied on the registry.
- **t006 (Render parity):** every exec surface runs through the registry. REST stream exec (`sandbox/rest.go:176`) and the connect-token run path (`sandbox/connect.go:296`) call `StreamExec`, and MCP `sandbox_exec` (`sandbox/mcp.go:145`) calls `ExecBuffered`. Both dial through `dialGateway`, which wraps an ordinary sandbox's command with `execArgv(command, registry)` (`sandbox/exec.go:188`). History reads the same records on every surface: REST `/execs` (`rest.go:141`) and MCP `list_sandbox_executions` (`mcp.go:138`) call `ListExecutions`, and GraphQL `sandboxExecutions`/`sandboxExecution` (`graphql.go:196`, `:207`) call `listExecutionViews`/`GetExecution`. Each reconciles through `closeStaleExecutions` first. GraphQL has no exec, and the dashboard has no sandbox exec surface. ADR104's row no longer cites m381, and ADR018's idle row records the walk.
- **Noted, not filed:** a reconciled `stoppedAt` has whole seconds (the in-sandbox `date +%s`), while a connected command's has microseconds. Render's precision is undocumented.
