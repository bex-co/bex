# w10 · m16 — Agent-session transcripts: store completed items, show signal, never crash on replay

**Worker:** worker10 **Goal:** A long agent turn is stored as a few hundred **completed items** (messages, reasoning blocks, tool calls with input/output/status, usage/finish) instead of one row per AI-SDK stream chunk. Today one 10-minute turn is 5,602 rows / 2.9 MB, and replaying it crashes the UI. After this milestone the dashboard replays any session without crashing. It leads with the signal: final answer, PR/diff, plan, grouped tool activity and one live status line. Reasoning and raw tool output are collapsed or on demand. **Status:** in progress (t001–t006 done 2026-10-26, verified in production — see done/t006.md; replay-bytes target 41% vs ≤25% on the long session, the rest is real tool output)

## Tasks (in order)

| id   | title                                                                                             | est  | depends_on       |
| ---- | ------------------------------------------------------------------------------------------------- | ---- | ---------------- |
| t001 | Dashboard replay never crashes: `useChat` throttle, surface partial-load errors, no "ended" flash — **DONE** | 45m  | —                |
| t002 | Persist completed items, not chunks: coalesce deltas / tool-input / ACP tool updates at the tee — **DONE**   | 2h   | —                |
| t003 | Cap tool output inline (head+tail, `truncated` flag) and drop internal/duplicate ACP payloads — **DONE**     | 1h   | t002             |
| t004 | Compact existing transcripts to the item model (migration/backfill), incl. the 5,000-part cap — **DONE**     | 1h   | t002, t003       |
| t005 | Signal-first conversation view: grouped tool line, collapsed reasoning, single status line — **DONE**        | 1.5h | t002             |
| t006 | Verify on production with `ags-dbf8nggkbcjs73a5r3u0` and a fresh long turn — **DONE**                        | 30m  | t001, t004, t005 |
| t007 | Render parity (transcript stream/REST/MCP shapes, dashboard + mobile consistency)                 | 20m  | t006             |
| t008 | Simplify                                                                                          | 20m  | t007             |
| t009 | Test coverage                                                                                     | 30m  | t007             |
| t010 | Closeout                                                                                          | 10m  | t008, t009       |

## Definition of done

- `https://dashboard.bex.co/agents/ags-dbf8nggkbcjs73a5r3u0` renders the whole turn after a hard reload: 96 reasoning blocks, 13 text blocks, ~300 tool calls. No React #185, no aborted stream, and no "This session ended before a conversation was recorded." flash.
- A fresh long production turn stores ≤ ~10% of today's row count for the same work, with no `*-delta`, `tool-input-start` or raw `data-acp-info` rows. Its replay is ≤ ~25% of today's bytes, and normal turns never hit the transcript cap.
- Tool output is stored head+tail-capped (≈30k chars) with an explicit `truncated` marker that the UI shows.
- The dashboard shows by default: final text, PR/diff, plan, "Ran N commands · Read M files" groups, and one status line. Reasoning and raw tool output open on demand.
- The backend real-DB suite, `make lint` and the driver/dashboard/mobile suites are green, and Prettier is clean.

## Source + Goal linkage

- **Source:** user report 2026-10-25 on `ags-dbf8nggkbcjs73a5r3u0` ("never succeeded"; "many chats while running but only one after it ends"; "'This session ended before a conversation was recorded.' flashes"). The read-only investigation found:
  - The session did succeed (PR #110).
  - The gateway delivers all 5,605 frames, and an offline AI-SDK replay rebuilds all 409 parts.
  - In the browser, `useChat` hits **React error #185** (maximum update depth) on the burst replay and aborts the stream at 1.4s (`ERR_ABORTED`). The error stays hidden because the conversation is non-empty.
  - `deriveConversationState` (`dashboard/src/features/agent-sessions/lib/conversation-state.ts:40`) returns `ended` before checking `resuming`, which causes the flash.
  - The harvest cap `maxTranscriptParts = 5000` (`lego/backend/internal/agentsessions/completion.go:536`) marked the turn incomplete.
- **Industry research (same day):**
  - The Vercel AI SDK persists final `UIMessage` parts, with chunks only in a short-lived resumable buffer.
  - OpenAI / Codex App Server use thread → turn → item with `item/completed`.
  - Anthropic returns thinking summarized or omitted.
  - Claude Code's JSONL stores one line per message or tool use, with large outputs spilled to files.
  - OpenHands and Cursor truncate tool output head+tail with a flag.
- **Sample composition:**
  - `data-acp-info` is 35% of bytes, mostly duplicate tool output plus internal agent notices.
  - Tool outputs are 33%.
  - 3,835 `reasoning-delta` rows hold 51 KB of text but take 399 KB stored (8×).
  - User-visible text is 0.4%.
- **Goal linkage:** ADR047/ADR051/ADR090 agent sessions (ADR008 AI-native pillar); the transcript is the customer's record of what the agent did.
- **Expected outcome:** long sessions are readable and fast to reopen; storage and replay shrink about 10×; customers see what was done and delivered, not token noise.
- **Why now:** a real production session is unreadable today, and every long turn will hit the same crash and cap.
- **Render parity included (t007):** the transcript surfaces (stream replay shape, REST/MCP if exposed) and the dashboard/mobile views change. There is no render.com equivalent, so this is a cross-surface consistency check.
- **Out of scope / follow-ups:**
  - downloadable full tool logs in object storage (the inline head+tail cap ships first);
  - paged `summary`/`full` history loading;
  - model-side reasoning summaries (Anthropic `display: summarized`, OpenAI reasoning summaries).
