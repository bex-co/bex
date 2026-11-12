# w8 · m151 — A sandbox exec runs as long as its command does

**Worker:** worker8 **Goal:** `bex sandboxes exec` streams a command's output and exit code for as long as the command runs, bounded by the sandbox and the gateway session, not by a 210 s write deadline; an execution record never stays open after its stream is gone. **Status:** done (2026-11-12: t001–t006 shipped 2026-11-10; t007's production replay passed — two 300 s execs exited 6 and 3 with their records closed)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The exec stream lifts the long-resume write deadline once the gateway accepts — **DONE** | 30m | — |
| [t002](done/t002.md) | An execution record whose stream ended without a stop is closed — **DONE** | 60m | — |
| [t003](done/t003.md) | Record what is knowable about the 40 s "no exit event" ending — **DONE** | 15m | — |
| [t004](done/t004.md) | Render parity — **DONE** | 15m | t001, t002 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004 |
| t007 | Closeout — **DONE** (live long exec on production) | 20m | t006 |

## Definition of done

1. A unit test proves the exec stream's write deadline is cleared once the gateway accepts, so a stream outlives 210 s; file-connect routes keep their reset.
2. An execution record left open by a bex-api restart reads as stopped (with `stoppedAt`, no `exitCode`) instead of `running` forever.
3. On production: `bex sandboxes exec <sbx> -- sh -c 'for i in $(seq 15); do sleep 20; echo t$i; done; exit 6'` prints `t1`…`t15` and exits 6 after about 300 s, `sh -c 'sleep 300; exit 3'` exits 3, and both records carry `stoppedAt` and `exitCode`. Fixtures stopped and deleted afterwards.

## Source + Goal linkage

- **Source:** promoted from [w8/243](../done/243.md) on 2026-11-10 (`/qa-find-bugs-cli` w8 loop sweep `54a1`, released `bex v0.5.0`, pin `1672bb379`). Finding and evidence are kept there.
- **Goal linkage:** sandboxes (pillar 5, ADR014/ADR042), matched to Render's sandbox exec, which documents no duration limit.
- **Expected outcome:** a test suite, `npm install` or build in a sandbox finishes instead of dying at 3½ minutes with an HTTP/2 `INTERNAL_ERROR`.
- **Why now:** w2/m278's wake-on-exec deadline (2026-11-07) cut every exec at 210 s, and v0.5.0 just shipped the command to customers.
- **Render parity included:** the change is behind REST `POST /v1/sandboxes/{id}/execs` streaming and the execution history list.

## Blocked — 2026-11-10

Everything but t007 is done and shipped. t007 replays the Definition of done on production, which needs (1) the scheduled 19:00 UTC deploy (production deploys run at 19:00/00:00/05:00 UTC since `4630a48a7`) and (2) a signed-in run. `scripts/qa-login.sh` is rejected by Kratos (rc 1 on 2026-11-10, as on 2026-11-08), so the QA password must be fixed or the user must re-authorize their own CLI login with run-owned `bex-canary` fixtures, as they did for the 2026-11-08 sweep.
