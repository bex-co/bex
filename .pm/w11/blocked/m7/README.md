# w11 · m7 — Live agent attach and needs-decision steering

**Worker:** worker11 **Goal:** attach and reattach from mobile to a live agent session, replay then follow the transcript without duplication, answer an explicit needs-decision pause, and survive app backgrounding. **Status:** blocked — but no longer on what this line used to say. The ADR047 phase-2 gateway **shipped in `w3/m43`** (attach listener with verbatim SSE replay + live splice, durable transcript store, driver `POST /turn`, `attach-ticket`), so the protocol/storage contract this milestone was waiting to consume now exists. The remaining hard gate is `w11/m6/t009`, which is blocked on physical-device verification. All 9 tasks are unimplemented.

## Gating

Hard gate: `w11/m6/t009` plus completion of `w3/m43` (the promoted gateway proxy + transcript store — the ADR047 § D9 target-API conversation endpoint). Consume that protocol/storage contract and coordinate with the dashboard consumer `w1/m64` (the promoted `w5/035`); this milestone owns only the mobile client and mobile push integration.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Implement ticketed attach, transcript replay, and live reconnect | 60m | w11/m6/t009, w3/009 |
| t002 | Render plans, diffs, terminal output, tools, and evidence compactly | 60m | t001 |
| t003 | Add explicit needs-decision lifecycle and prompt-turn contracts | 60m | t002 |
| t004 | Add mobile steering with background/foreground recovery | 60m | t003 |
| t005 | Add needs-decision/completion push and phase-1 fallback | 45m | t004 |
| t006 | Render parity | 30m | t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 60m | t006 |
| t009 | Closeout | 10m | t008 |

## Definition of done

A session requesting input sends push to the exact conversation; attach replays stored transcript before live chunks; the user replies once and the same session/branch continues. Disconnect, ticket expiry, backgrounding, and multi-client attach neither lose nor duplicate accepted turns/output. Cross-workspace tickets and nonce reuse fail closed. When phase-2 gateway configuration is absent, the client clearly falls back to the phase-1 result/evidence view.

## Source + Goal linkage

- **Source:** ADR048 M3, ADR047 phase 2, held gateway note `w3/009`, and held dashboard UI note `w5/035`.
- **Goal linkage:** pillar 5's interactive supervision and the phone-shaped short-decision loop.
- **Expected outcome:** agents can ask for a decision without waiting for a desktop, while transcript and gateway trust remain in their existing owners.
- **Why now:** task boundaries are arranged now, but execution is deliberately blocked on the gateway/transcript contract so mobile cannot invent reconnection semantics.
- **Render parity:** included for needs-decision/prompt mutations across REST/GraphQL/MCP; the live UI stream is a documented bex extension.
