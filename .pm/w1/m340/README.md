# w1 · m340 — Live agent sessions in the terminal: SSE stream, `bex agent chat`, picker

**Worker:** worker1 **Goal:** a developer watches and steers a Cloud Agent session live from the terminal (Devin `devin --cloud` parity), and `--follow` streams instead of polling. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | SSE stream client with ticket re-mint, dedupe and fallbacks | 120m | — |
| t002 | SSE replay fixture and gateway twin test | 45m | t001 |
| t003 | `logs --follow` and `run`/`send --follow` on SSE | 45m | t001 |
| t004 | `bex agent chat` (alias `attach`): line-mode live session | 120m | t001 |
| t005 | Chat slash commands | 60m | t004 |
| t006 | Picker for an omitted `<session>`/`<automation>`, and shell completion | 60m | — |
| t007 | Document chat, streaming follow and the picker | 30m | t005, t006 |
| t008 | Live check on production | 45m | t007 |
| t009 | Render parity (Devin reference) | 15m | t002, t003, t008 |
| t010 | Simplify | 15m | t009 |
| t011 | Test coverage | 30m | t009 |
| t012 | Closeout | 10m | t011 |

## Definition of done

1. `bex agent chat <session>` on production shows the replayed transcript then the live tail; a follow-up typed during a running turn is queued and sent by steer when the turn ends; `/quit` and Ctrl-D detach and the session keeps running.
2. After a forced connection drop, `chat` reconnects with a freshly minted ticket and prints no duplicated part.
3. `logs --follow` and `run`/`send --follow` use the SSE stream and fall back to polling on 503 or `AGENT_SESSION_NOT_ATTACHABLE`; their exit codes are unchanged.
4. An omitted `<session>`/`<automation>` opens a picker on a TTY and exits 2 without one; shell completion offers ids and names within 2 s.
5. The SSE replay fixture is guarded by a gateway-side twin test.

## Source + Goal linkage

- **Source:** ADR106 D2 and D3 (picker, completion); ADR047 D9 (attach ticket, gateway SSE replay + live tail). Approved `/pm-brainstorm ADR106 for w1` item 3 via `/pmx`, 2026-11-11. Design: [ADR106](../../../docs/ADR106-cli-cloud-agents-automations.md).
- **Goal linkage:** ADR008 AI-native surfaces; Devin CLI parity (`devin --cloud`, `-r` picker).
- **Expected outcome:** the terminal becomes a first-class place to supervise agents; CI logs show agent output as it happens instead of on the next poll.
- **Why now:** the CLI polls `/transcript` with 2-15 s backoff although the gateway already serves the stream the dashboard reads. Depends on m339 for the lifecycle verbs and URL references the slash commands and picker use. Render parity is included with Devin as the reference product; no server change.
