# w11 · m7 — Live agent attach and needs-decision steering

**Worker:** worker11 **Goal:** attach and reattach from mobile to a live agent session, reconstruct and follow the conversation without duplication, answer an explicit ACP-backed needs-decision pause, and survive app backgrounding. **Status:** todo (t003, t010–t012 done) — mobile implementation and interim evidence have landed provisionally, but formal completion remains gated on w11/m6/t009

## Gating

`w3/m43` is complete and its production conversation endpoint is the baseline, not a remaining gate. The research handoff found protocol gaps that must be corrected before another client consumes it: GET and POST require distinct `read`/`turn` tickets; reconnect needs a durable cursor instead of full replay; user turns must survive client loss; and transcript sequence allocation must remain monotonic across reconnect/redispatch. Tasks t010–t012 are actionable now and coordinate with the dashboard consumer `w1/m64`. Mobile implementation and physical-device proof remain gated on `w11/m6/t009`.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t010 | Harden action tickets, cursor replay, and session-global transcript ordering — **DONE** | 60m | w3/m43 |
| t011 | Persist idempotent user turns and reconstruct complete conversation history — **DONE** | 60m | t010 |
| t001 | Implement Expo AI SDK attach, replay, and live reconnect | 60m | w11/m6/t009, t011 |
| t002 | Render plans, diffs, terminal output, tools, and evidence compactly | 60m | t001 |
| t003 | Add explicit needs-decision lifecycle and prompt-turn contracts — **DONE** | 60m | t011 |
| t012 | Broker ACP elicitation and permission requests into durable decisions — **DONE** | 60m | t003 |
| t004 | Add mobile steering with background/foreground recovery | 60m | t001, t002, t012 |
| t005 | Add needs-decision/completion push and phase-1 fallback | 45m | t004 |
| t006 | Render parity | 30m | t005 |
| t013 | Verify and polish the mobile UI with Expo MCP | 45m | t006 |
| t007 | Simplify | 20m | t013 |
| t008 | Test coverage | 60m | t013 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

A session requesting input sends push to the exact conversation; attach reconstructs durable user and assistant turns, resumes after a server cursor, then follows live chunks. GET uses a one-time `read` ticket and POST a one-time `turn` ticket. The user replies once and the same session/branch continues. Disconnect, process death, ticket expiry, hibernation/redispatch, backgrounding, and multi-client attach neither lose nor duplicate accepted turns/output. ACP clarification and security permission requests remain distinct, are never inferred from transcript prose, and cannot silently grant persistent/bypass permission. Cross-workspace tickets and nonce reuse fail closed. When live attach is unavailable, the client clearly falls back to the phase-1 result/evidence view. Expo MCP evidence shows the affected mobile flows were interactively viewed and polished across representative phone sizes, themes, lifecycle states, keyboard/safe-area behavior, and conversation states before closeout.

## Source + Goal linkage

- **Source:** ADR048 M3, ADR047 D9, completed gateway milestone `w3/m43`, dashboard consumer `w1/m64`, and the 2026-08-16 Expo + Vercel AI SDK + ACP repository/official-source research handoff.
- **Goal linkage:** pillar 5's interactive supervision and the phone-shaped short-decision loop.
- **Expected outcome:** agents can ask for a decision without waiting for a desktop; a killed or reconnected phone reconstructs the complete conversation and continues once without weakening gateway trust.
- **Why now:** the gateway is shipped, so protocol ambiguity—not missing infrastructure—is now the risk. Correct the shared ticket/replay/turn contract before a second AI SDK client copies the dashboard's current full-replay and generic-ticket behavior; backend/driver work can proceed while physical-device closeout holds the mobile slice.
- **Render parity:** included for needs-decision/prompt mutations across REST/GraphQL/MCP; the live UI stream is a documented bex extension.
- **Mobile UI visual verification:** included because t001/t002/t004/t005 add user-visible native conversation, decision, lifecycle, and push flows; Expo MCP viewing and polish are required before simplify/tests/closeout.
