# w3 · m406 — The conversation stream ends with `[DONE]` only when the turn has settled

**Worker:** worker3 **Goal:** the gateway's agent-session stream never ends with `[DONE]` while the session is still working. When a turn's sandbox has no address yet, the stream is held until it serves (or ends with a documented reconnect signal), so the CLI, the dashboard, the mobile app and any Vercel AI SDK client see the whole turn without their own workaround. **Status:** done

## Tasks (in order)

| id   | title                                                                       | est | depends_on |
| ---- | --------------------------------------------------------------------------- | --- | ---------- |
| [t001](done/t001.md) | Reproduce the early `[DONE]` and record what each client does — **DONE** | 45m | —          |
| [t002](done/t002.md) | Gateway: hold a working session's stream until its sandbox serves — **DONE** | 75m | t001       |
| [t003](done/t003.md) | Update the stream contract and the CLI — **DONE** | 45m | t002       |
| [t004](done/t004.md) | Dashboard (and mobile, if affected) follow a new turn through sandbox start — **DONE** | 45m | t002       |
| [t005](done/t005.md) | Live check on production — **DONE** | 30m | t003, t004 |
| [t006](done/t006.md) | Render parity (Devin reference) — **DONE** | 15m | t005       |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006       |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t006       |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t008       |

## Definition of done

1. Gateway test: attaching to a session that is working but whose sandbox address does not resolve yet gets no `[DONE]` until the session settles. The stream splices the live driver output once the sandbox serves, or ends without `[DONE]` under the documented reconnect rule. A terminal session still replays and ends with `[DONE]`, as before.
2. `lego/backend/internal/agenttranscript/cli_stream_contract_test.go` and `lego/cli/testdata/agent-stream-replay.sse` state the rule. The CLI's `streamUntilSettled` reconnects only for dropped connections.
3. On production, `bex agent send <id> "…" --follow` on a settled QA session (a new turn starts a new sandbox) prints the turn from start to end. The dashboard's open conversation shows the new turn live without a reload.

## Source + Goal linkage

- **Source:** found in the w1/m340 live check, 2026-11-12. An early `[DONE]` arrived before the new turn's sandbox served and ended `--follow`. It was worked around in the client: `lego/cli/internal/agent/stream.go` `streamUntilSettled` reattaches after an early `[DONE]`. Gateway: `lego/backend/internal/sshgateway/agentattach/agentattach.go` `streamAgentAttach` ("A terminal/gone session (ipErr) replays and closes with `[DONE]`"). A working session whose next sandbox is still starting takes the same `ipErr` path. The dashboard (`dashboard/src/features/agent-sessions/components/session-conversation-impl.tsx`) treats `[DONE]` as the end of a terminal session's stream. Whether it stalls in this window is unverified; t001 checks it.
- **Goal linkage:** the [ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md) D9 stream contract (terminal = replay + `[DONE]`), so `[DONE]` should mean settled; [ADR106](../../../../docs/ADR106-cli-cloud-agents-automations.md) D3. The reference is Devin, whose session chat follows a new turn live; t006 verifies this.
- **Expected outcome:** one stream rule every client can rely on; no client-side reattach workaround needed.
- **Why now:** v0.6.0 shipped the CLI's workaround to users. Every other client of the same endpoint (dashboard, mobile, AI SDK consumers per ADR047 D9) otherwise needs its own.
- **Render parity included** with Devin as the reference: the change alters the stream API's behavior and the dashboard.

## t001 — what production does (2026-11-26)

Two sessions in the `bex` workspace (both disposable, deleted after), read with a raw SSE client (`attach-ticket`, then `GET streamUrl`) polling the session every 0.5 s:

| Path | Timeline |
| --- | --- |
| Steer on a settled session | Steer at 24.2 s: `redispatching`, no sandbox, `attach-ticket` answers 409 `AGENT_SESSION_NOT_ATTACHABLE`. At 28.9 s `running` with the new sandbox; the attach at 29.3 s replayed turn 1, then spliced turn 2 live; the driver's `[DONE]` came at 33.5 s. The session read `running` until 37.95 s, about 4.5 s later, when the Completer settled it. The CLI's reattach-while-working loop attached three more times in that window, each answered at once with the replay and `[DONE]` (the driver's stream had closed) |
| Create | `creating` (409) until 5.4 s, then the attach spliced the turn live; `[DONE]` at 8.7 s; `completed` by 9.0 s |

No attach ended before its sandbox served: once `sandboxId` was set, the driver answered at once. The `[DONE]` w1/m340 saw before the session settled is the driver's turn-end `[DONE]`, read against a session the Completer had not settled yet. The gateway still ended a working session's stream with `[DONE]` whenever it could not reach the ticket's sandbox (`agentattach.go` `streamAgentAttach`: a pod without an address, or a driver that did not answer). Working phases that carry such a sandbox: a sandbox that died mid-turn before the Completer notices (its pod is terminated, so `PodIP` fails), an editor start (`resuming` binds `sandbox_id` before the sandbox serves, `store/agentsession_editor.go` `BindAgentSessionEditorSandbox`), and a dispatch recorded `creating` with a sandbox that is not running.

- **Dashboard** (QA user, 1440 px, production): a follow-up sent at 5.6 s; `redispatching` at 6.8 s; sandbox bound at 10.6 s; the column opened `GET …/stream` at 11.5 s, which stayed open until the driver's `[DONE]` at 42.3 s; the reply rendered at 42.4 s while the session still read `running` (it settled at 51.1 s). The column already follows a new turn live: it re-attaches when the session's turn, sandbox or terminal state changes (`attachSignal`).
- **Mobile**: reads no stream. The session screen polls GraphQL (`mobile/src/features/agent-sessions/detail/session-detail-screen.tsx`), so `[DONE]` does not reach it.

## What shipped (t002–t004)

- **Gateway** (`lego/backend/internal/sshgateway/agentattach/agentattach.go`): `[DONE]` ends a read stream only once its turn has settled. A working session whose sandbox does not serve is held with the 15 s heartbeat while the gateway retries the pod and the driver (0.5 s doubling to 5 s, a 10 s header timeout per try) for up to `agentsessions.SSHWakeTimeout` (16 min). After each failed try it reads the session: settled or deleted gets the rest of the durable transcript, then `[DONE]`; a session on another sandbox, the bound, a driver connection that ends without the driver's `[DONE]`, a client that stopped reading and a store failure close without `[DONE]`. The replay became `durableReplay`, which keeps its place for that second pass. The gateway's database role already reads `agent_sessions`; the role proof now counts `GetAgentSession` as part of the attach surface.
- **CLI** (`lego/cli/internal/agent/stream.go`): after `[DONE]` it reads the session until it settles, attaching again only when a queued message has started a later turn than the ticket's (`turns` on the mint); after a cut it attaches again, even when the session settled meanwhile, so the rest of the turn prints.
- **Dashboard** (`transport.ts`, `session-conversation-impl.tsx`): the transport reads whether a stream ended with `[DONE]` (the SDK's parser drops the sentinel); the column attaches again to a stream closed without it while the session works, 1 s to 15 s apart, at most 8 times in a row.
- **Contract**: ADR047 § w3/m406 and its D9 table row; `lego/cli/testdata/agent-stream-replay.sse` states the rule, and `cli_stream_contract_test.go` pins its single closing `[DONE]`.

## Render parity (Devin reference) (t006)

- **Devin.** A session that went to sleep wakes when a message is sent to it, and its chat carries on with that turn ([Usage](https://docs.devin.ai/admin/billing/usage): "You can wake the session up at any time by sending another message"; [2026 release notes](https://docs.devin.ai/release-notes/2026): a sleeping session starts waking as soon as you type, so Devin "is usually ready to work the moment you press send"). bex's follow-up turn does the same: a fresh sandbox, and the conversation follows it live (t001: the dashboard showed the reply while the session still read `running`).
- **Across bex's surfaces.** One rule for the one stream: the gateway, `bex agent` and the dashboard read `[DONE]` as settled and a close without it as a reason to attach again. The attach-ticket mint (REST `POST /v1/agent-sessions/{id}/attach-ticket`, GraphQL, MCP) is unchanged; MCP and the mobile app read no stream.
- **Drift.** None to file.

## Simplify (t007)

`/simplify` ran three reviews (reuse, quality, efficiency) over the change. Fixed: the wait bound reuses `agentsessions.SSHWakeTimeout`, the bound the Terminal and an editor's SSH login already wait for a starting sandbox, instead of a second number; the live half is one loop (`followLive`) whose first pass is the first dial, the pod is resolved there, and a replay-only ticket is `claims.Pod == ""`; `liveCut` is the zero value, so a forgotten case never sends `[DONE]`; a held stream whose client stopped reading ends; the driver client times out a driver that sends no headers in 10 s; a store failure on the read path closes without an `error` part (the SDK stops on that part before it sees the close, so the dashboard would not attach again); `errorAndDone` lost its duplicate; the CLI paces the attach for a queued turn; the dashboard caps repeated cuts at 8, uses `useEffectEvent` and one `attachOnce` helper, and checks for `[DONE]` once at the end of the stream instead of per chunk. Kept: the `durableReplay` struct (the settled-while-waiting pass needs its place), per-try session reads (a primary-key read every 0.5–5 s while a stream waits).

## Live check on production (t005, 2026-11-26)

Production pin `11695fb886cb` (carries `f40d9613db`). A `bex` built from main at `f6d5b7665b` with the QA user's device login, in the `bex` workspace, on one session (`ags-dc45i73i676s73bu75h0`, disposable, deleted after). Turn 1 settled first; turns 2–4 each went to that settled session with `bex agent send <id> --follow "Run 'sleep 15' …, then reply with <word> …"`, so each started a new sandbox. The CLI's requests were read from its HTTP/2 trace (paths only), and the dashboard ran in a headless Chromium on the QA session, read every 250 ms and never reloaded. Times are seconds after the steer.

| Turn | CLI | Dashboard |
| --- | --- | --- |
| 3, at 1440 px | Two attach tickets at 0.6 and 3.1 s were refused (no sandbox yet; the CLI read the session between them). The third, at 6.6 s, opened **the one stream** of the turn at 6.8 s. It printed the turn from 7.1 s (prompt, setup, `▸ Terminal sleep 15`), then `MANGO` at 26.6 s, where the stream ended with `[DONE]`. After that the CLI only read the session (26.7, 28.9, 32.3, 37.0 s) until it settled, and exited 0 at 37.4 s with `succeeded (turn 3)` | `Redispatching` at 4.2 s, `Running` at 9.5 s. It opened the stream at 9.7 s, rendered `MANGO` at 26.6 s with the header still reading `Running`, and the stream closed at 26.7 s |
| 4, at 390 px | The same shape: tickets at 0.3 and 2.8 s refused, then one stream at 6.5 s. `PAPAYA` at 26.3 s, session reads until it settled, exit 0 at 36.8 s | It saw `Running` at 19.0 s, on its next session read. Its stream at 19.4 s replayed the turn so far and spliced the rest, and it rendered `PAPAYA` at 26.4 s with the header still reading `Running`. At `Completed` (40.4 s) it attached once more (the replay, then `[DONE]`, in 0.6 s) |
| 2, at 1440 px | One stream, the same shape. `[DONE]` at 24.0 s, settled and exited 0 at 34.9 s | Its stream was open from 9.6 s to 24.0 s. The reply was on screen after the turn without a reload (this run's marker word was misspelled, so it has no render time) |

DoD 3 holds. `--follow` printed each turn from start to end on one connection. It did not attach again after the turn-end `[DONE]`, although the session had not settled yet: the CLI's session reads saw it settle about 10 s later (t001 measured 4.5 s). The dashboard showed each new turn live, without a reload, at 1440 and at 390 px. Cleanup: the session was deleted, `bex logout` ran, and the QA user's Kratos session was logged out.

Seen during the check. None of these is m406's: each lives in code m406 did not change, and none blocks the DoD.

- **Dashboard: earlier turns lose their tool rows after an in-place re-attach.** After the open column attached again (to a new turn, or once the session settled), the earlier turns showed their text but not their `Used 1 tool` and `Ran 1 command` rows. A fresh load showed every turn's rows. The gateway's replay carries every turn's tool calls, each with its own id (`bex-python-1`…`4`, one `Terminal` per turn). The cause is likely the fold `keepLastReplay` (`dashboard/src/features/agent-sessions/lib/collapse-doubled-parts.ts`, from 2026-08): the SDK updates a re-sent tool part in place, in the older copy that the fold drops.
- **Dashboard at 390 px: a running session's header hides its state.** The Files button covers the status badge (only `Ru` shows) and the title. The settled header fits.
- **CLI: the live render prints empty tool calls.** Each turn printed two `▸ Terminal {}` lines before `▸ Terminal sleep 15`. `bex agent logs` shows one Terminal call per turn (`chunkRenderer` in `lego/cli/internal/agent/stream.go`).

## Unblocked (2026-11-26)

- **Affected:** w3/m406 t005, then t009.
- **Source:** verified evidence. The newest `chore(deploy): pin platform images to 11695fb886cb` (`ec6211f2ca`) on `origin/main` carries the gate's commit: `git merge-base --is-ancestor f40d9613db 11695fb886cb` exits 0. The gate (pin must carry `f40d9613db`) holds.
- **Remaining gates:** none. The milestone returns from `blocked/` with t005 open.
- **Next:** t005's production check as the QA user: `bex agent send <id> "…" --follow` with a locally built `bex` on a settled session prints the turn in one connection, and the dashboard shows the turn live at 1440 and 390 px. Then t009 closes the milestone.
