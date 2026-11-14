# w1 · m406 — The conversation stream ends with `[DONE]` only when the turn has settled

**Worker:** worker1 **Goal:** the gateway's agent-session stream never ends with `[DONE]` while the session is still working. When a turn's sandbox has no address yet, the stream is held until it serves (or ends with a documented reconnect signal), so the CLI, the dashboard, the mobile app and any Vercel AI SDK client see the whole turn without their own workaround. **Status:** todo

## Tasks (in order)

| id   | title                                                                       | est | depends_on |
| ---- | --------------------------------------------------------------------------- | --- | ---------- |
| t001 | Reproduce the early `[DONE]` and record what each client does               | 45m | —          |
| t002 | Gateway: hold a working session's stream until its sandbox serves           | 75m | t001       |
| t003 | Update the stream contract and the CLI                                      | 45m | t002       |
| t004 | Dashboard (and mobile, if affected) follow a new turn through sandbox start | 45m | t002       |
| t005 | Live check on production                                                    | 30m | t003, t004 |
| t006 | Render parity (Devin reference)                                             | 15m | t005       |
| t007 | Simplify                                                                    | 15m | t006       |
| t008 | Test coverage                                                               | 30m | t006       |
| t009 | Closeout                                                                    | 10m | t008       |

## Definition of done

1. Gateway test: attaching to a session that is working but whose sandbox address does not resolve yet gets no `[DONE]` until the session settles. The stream splices the live driver output once the sandbox serves, or ends without `[DONE]` under the documented reconnect rule. A terminal session still replays and ends with `[DONE]`, as before.
2. `lego/backend/internal/agenttranscript/cli_stream_contract_test.go` and `lego/cli/testdata/agent-stream-replay.sse` state the rule. The CLI's `streamUntilSettled` reconnects only for dropped connections.
3. On production, `bex agent send <id> "…" --follow` on a settled QA session (a new turn starts a new sandbox) prints the turn from start to end. The dashboard's open conversation shows the new turn live without a reload.

## Source + Goal linkage

- **Source:** found in the w1/m340 live check, 2026-11-12. An early `[DONE]` arrived before the new turn's sandbox served and ended `--follow`. It was worked around in the client: `lego/cli/internal/agent/stream.go` `streamUntilSettled` reattaches after an early `[DONE]`. Gateway: `lego/backend/internal/sshgateway/agentattach/agentattach.go` `streamAgentAttach` ("A terminal/gone session (ipErr) replays and closes with `[DONE]`"). A working session whose next sandbox is still starting takes the same `ipErr` path. The dashboard (`dashboard/src/features/agent-sessions/components/session-conversation-impl.tsx`) treats `[DONE]` as the end of a terminal session's stream. Whether it stalls in this window is unverified; t001 checks it.
- **Goal linkage:** the [ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md) D9 stream contract (terminal = replay + `[DONE]`), so `[DONE]` should mean settled; [ADR106](../../../docs/ADR106-cli-cloud-agents-automations.md) D3. The reference is Devin, whose session chat follows a new turn live; t006 verifies this.
- **Expected outcome:** one stream rule every client can rely on; no client-side reattach workaround needed.
- **Why now:** v0.6.0 shipped the CLI's workaround to users. Every other client of the same endpoint (dashboard, mobile, AI SDK consumers per ADR047 D9) otherwise needs its own.
- **Render parity included** with Devin as the reference: the change alters the stream API's behavior and the dashboard.
