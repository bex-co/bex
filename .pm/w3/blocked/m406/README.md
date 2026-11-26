# w3 · m406 — The conversation stream ends with `[DONE]` only when the turn has settled

**Worker:** worker3 **Goal:** the gateway's agent-session stream never ends with `[DONE]` while the session is still working. When a turn's sandbox has no address yet, the stream is held until it serves (or ends with a documented reconnect signal), so the CLI, the dashboard, the mobile app and any Vercel AI SDK client see the whole turn without their own workaround. **Status:** blocked (pin must carry `f40d9613db`: t005's production check, then t009; t001–t004 and t006–t008 done)

## Tasks (in order)

| id   | title                                                                       | est | depends_on |
| ---- | --------------------------------------------------------------------------- | --- | ---------- |
| [t001](done/t001.md) | Reproduce the early `[DONE]` and record what each client does — **DONE** | 45m | —          |
| [t002](done/t002.md) | Gateway: hold a working session's stream until its sandbox serves — **DONE** | 75m | t001       |
| [t003](done/t003.md) | Update the stream contract and the CLI — **DONE** | 45m | t002       |
| [t004](done/t004.md) | Dashboard (and mobile, if affected) follow a new turn through sandbox start — **DONE** | 45m | t002       |
| t005 | Live check on production                                                    | 30m | t003, t004 |
| [t006](done/t006.md) | Render parity (Devin reference) — **DONE** | 15m | t005       |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006       |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t006       |
| t009 | Closeout                                                                    | 10m | t008       |

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
