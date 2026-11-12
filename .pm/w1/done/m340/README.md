# w1 · m340 — Live agent sessions in the terminal: SSE stream, `bex agent chat`, picker

**Worker:** worker1 **Goal:** a developer watches and steers a Cloud Agent session live from the terminal (Devin `devin --cloud` parity), and `--follow` streams instead of polling. **Status:** done (2026-11-12; verified on production, see Live check)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | SSE stream client with ticket re-mint, dedupe and fallbacks — **DONE** | 120m | — |
| t002 | SSE replay fixture and gateway twin test — **DONE** | 45m | t001 |
| t003 | `logs --follow` and `run`/`send --follow` on SSE — **DONE** | 45m | t001 |
| t004 | `bex agent chat` (alias `attach`): line-mode live session — **DONE** | 120m | t001 |
| t005 | Chat slash commands — **DONE** | 60m | t004 |
| t006 | Picker for an omitted `<session>`/`<automation>`, and shell completion — **DONE** | 60m | — |
| t007 | Document chat, streaming follow and the picker — **DONE** | 30m | t005, t006 |
| t008 | Live check on production — **DONE** | 45m | t007 |
| t009 | Render parity (Devin reference) — **DONE** | 15m | t002, t003, t008 |
| t010 | Simplify — **DONE** | 15m | t009 |
| t011 | Test coverage — **DONE** | 30m | t009 |
| t012 | Closeout — **DONE** | 10m | t011 |

## Definition of done

1. `bex agent chat <session>` on production shows the replayed transcript then the live tail; a follow-up typed during a running turn is queued and sent by steer when the turn ends; `/quit` and Ctrl-D detach and the session keeps running.
2. After a forced connection drop, `chat` reconnects with a freshly minted ticket and prints no duplicated part.
3. `logs --follow` and `run`/`send --follow` use the SSE stream and fall back to polling on 503 or `AGENT_SESSION_NOT_ATTACHABLE`; their exit codes are unchanged.
4. An omitted `<session>`/`<automation>` opens a picker on a TTY and exits 2 without one; shell completion offers ids and names within 2 s.
5. The SSE replay fixture is guarded by a gateway-side twin test.

## Source + Goal linkage

- **Source:** ADR106 D2 and D3 (picker, completion); ADR047 D9 (attach ticket, gateway SSE replay + live tail). Approved `/pm-brainstorm ADR106 for w1` item 3 via `/pmx`, 2026-11-11. Design: [ADR106](../../../../docs/ADR106-cli-cloud-agents-automations.md).
- **Goal linkage:** ADR008 AI-native surfaces; Devin CLI parity (`devin --cloud`, `-r` picker).
- **Expected outcome:** the terminal becomes a first-class place to supervise agents; CI logs show agent output as it happens instead of on the next poll.
- **Why now:** the CLI polls `/transcript` with 2-15 s backoff although the gateway already serves the stream the dashboard reads. Depends on m339 for the lifecycle verbs and URL references the slash commands and picker use. Render parity is included with Devin as the reference product; no server change.

## Live check (t008, 2026-11-12)

A locally built `bex` (QA device login) ran on production in the `bex` workspace. `chat` needs a terminal, so it ran on a real pseudo-terminal driven by a script (`pty.fork`), with input typed on a schedule.

| Check | Result |
| --- | --- |
| `chat` with a follow-up typed mid-turn | `chat --max-spend 1 "Run 'sleep 45' … OK"` showed the header `ags-… · model · working`. The turn streamed: the prompt, `▸ Terminal sleep 45`, then `OK`. The line typed at 25 s printed `queued — sends when this turn ends`. When turn 1 settled (`── turn 1 succeeded · $0.006184 ──`), the queued line went out as turn 2. Turn 2's prompt and `DONE` streamed; `/quit` left with exit 0 |
| Reattaching prints nothing twice | Turn 2 reattached with a fresh ticket. The connection replays the whole transcript, and turn 1's items were not printed again |
| `chat --read-only` | Attached to a running routine session: header with repo, branch, model, state and spend. A typed line answered `read-only: messages are not sent`. Ctrl-D left (`it keeps running`), exit 0, with no steer and no cancel |
| `run --follow` over the stream | `run … --follow` printed the live turn (`▸ Terminal sleep 30`, `OK`) through provisioning, then the session; exit 0 |
| `logs --follow` | On a routine session that finished during the read: 221 streamed lines. With error placement normalized, they matched the polled transcript's 218 lines exactly. On a quiet running session it timed out with exit 124 and the "still working … `bex agent wait` resumes" message |
| Picker and completion | `bex agent get` on a terminal listed the 20 most recent sessions; `1` picked the first and printed it. `bex __complete agent get ""` returned ids with labels in under a second |

Found and fixed during the check:

- **An early `[DONE]` ended `--follow`.** Right after a create or steer, the gateway can answer before the turn's sandbox serves: it replays and closes with `[DONE]`, and `--follow` ended there. The stream loop now returns only once the session has settled, reattaching with backoff. Regression test: `TestFollowReattachesAfterAnEarlyDone`.
- **Fence-only error previews.** A tool error preview could be the bare code-fence line; both printers now skip fence lines.
- **Turn summary without a reason.** The summary showed no reason when the API sent `failureReason` without a code; it now does.
- **Picker labels.** Labels skip an empty repo.

**Not done live.** A drop forced in the middle of a turn was not provoked on production: there is no harmless way to cut one connection there. Reattaching is exercised live (chat's turn 2 and the early-`[DONE]` path). Tests pin the mid-turn cut (`TestLogsFollowStreamsAndReconnects`: a connection cut halfway, then a fresh ticket, with each item printed once).

**Production context.** One fixture session failed at 09:36 with `sandbox create failed` while w1/144's platform-pool roll was replacing nodes. A probe at 09:42 succeeded. Every m340 fixture session has been deleted.
