# w32 · m381 — A command keeps its sandbox awake until it finishes, even after the client disconnects, as on Render

**Worker:** worker32 **Goal:** a command started in a sandbox (CLI `sandboxes exec`, the Render SDK's `exec`, REST, the connect-token run path or MCP `sandbox_exec`) keeps the sandbox active for as long as it runs, whether or not its client is still connected. The sandbox's execution history shows the command as running until it exits, then records its exit code. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design: how bex knows a command is still in progress without its client | 45m | — |
| t002 | The idle sweeper does not suspend a sandbox while a command it started is running; the idle window runs from that command's end | 2h | t001 |
| t003 | Execution history: a command whose client left stays running in the history until it exits, then carries its exit code | 1.5h | t001 |
| t004 | Blast radius: the gateway's 4 h session cap, bex-api and ssh-gateway restarts, MCP's buffered exec, file transfers, agent-session sandboxes | 30m | t002, t003 |
| t005 | Production walk with the release CLI | 45m | t002, t003, t004 |
| t006 | Render parity | 20m | t005 |
| t007 | Simplify | 15m | t006 |
| t008 | Test coverage | 30m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

On production, with the released `bex` CLI against a new sandbox in `bex-canary` created with `--idle-timeout 300`:

- Start `bex sandboxes exec <sbx> -- bash -c 'sleep 480; date > ~/done'` and kill the CLI after 5 s, with SIGTERM and again with SIGINT. The sandbox stays `running` in `bex sandboxes list --all -o json` for the whole 480 s. `~/done` then exists, and the sandbox suspends only about 300 s after the command ended.
- `GET /v1/sandboxes/<sbx>/execs` shows that execution with no `stoppedAt` while it runs. Once it ends, the record carries `stoppedAt` at its exit and `exitCode: 0`, not the disconnect time with no exit code.
- The control still holds: a connected command longer than the idle timeout keeps the sandbox running, as on 2026-11-13.
- A background process left behind by a command that has finished does not keep the sandbox awake (Render: "Background processes do not keep a sandbox active after the command that started them finishes"). An idle sandbox still suspends on time.
- ADR014 D4's activity-signal paragraph and ADR104's suspend row describe the new rule.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep d91f, 2026-11-13, in `bex-canary` (`tea-daif693dqjvc73e7as3g`), with release `bex` v0.5.0 (Render CLI v2.35.0 pin) against `https://api.bex.co/v1/`, human device login, production pin `8acf1d116941`. The evidence is below.
- **Goal linkage:** pillar 5 sandboxes and [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first). Agents and CI jobs start long commands and stop watching them. The Render SDK times out on 45 s of silence (w32/m284), and laptops sleep.
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
