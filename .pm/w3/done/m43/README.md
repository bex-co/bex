# w3 · m43 — Agent-session conversation API: gateway stream endpoint, transcript store, `api.bex.co` edge routing (ADR047 D9 target API shape)

**Worker:** worker3 **Goal:** ship the Vercel-AI-SDK-compatible conversation plane decided in ADR047 § D9 "Target API shape": one ticket-authenticated stream endpoint published under the primary API origin — `POST` submits a live prompt turn, `GET` replays the transcript then splices the live tail (terminal sessions: replay-only) — served by the gateway process under the D3 verbatim-forward guarantee, with the transcript store and the `attach-ticket` mint closing the reconnect/history gaps. **Status:** done — **live E2E green on prod 2026-08-04** (`scripts/agent-session-verify.sh`: create→draft PR + evidence, steering follow-up commit, non-`bex-agent/*` refusal, and the full conversation plane — attach-ticket mint, ticketless 401, attach+replay with `x-vercel-ai-ui-message-stream: v1` + `[DONE]`, live-attach streaming 63 teed parts, reattach replaying the durable transcript). Three real integration bugs were found + fixed + shipped during the run: Anthropic OAuth model keys now route to `CLAUDE_CODE_OAUTH_TOKEN`; the Completer's status read minted an empty-subject exec ticket (gateway 401 → sessions stranded in `running`) now uses a system subject + full observability logging; a failed turn's driver exited before the Completer could read `status:failed` and now stays alive. Follow-ups filed: crash-path stranding (`w3/012`) and the attach-ticket `url`-vs-SSE-base contract (`w3/013`). Earlier partial verification (2026-08-02): the feature is enabled on prod (create passes `enabled`+`ticketEnabled`, not 503), the stream endpoint is edge-routed to the gateway (`GET .../stream` without a ticket → 401 "invalid ticket" from the gateway), the `attach-ticket` verb is wired + authz-gated (403 on a nonexistent session), and — after a live probe caught a missing-CORS bug and it was fixed in place — the dashboard can reach the stream cross-origin (browser `fetch` returns a readable 401, no longer "Failed to fetch"). Unit: backend 47 pkgs incl. real-Postgres transcript round-trip, gateway attach replay/live/terminal/reject/CORS, driver `POST /turn`; driver 22 tests; `make lint-backend` 0 issues. **Still open (t010):** the full live-substrate E2E run (create → attach → replay → live turn → reattach) needs a disposable repo + the workspace's OpenBao model key + a prod caller token — the m41 operator-run gate; `scripts/agent-session-verify.sh` now has those legs, they just haven't been run green on prod.

## Gating

Hard gate: **w3/m41 t004 + t008** (live E2E + closeout — the stream/ticket contracts must be proven by working phase-1 code first; this is the promoted `009` note's hold, now materialized). m42 (hibernate/resume + driver resume-mode) is done and is consumed, not reimplemented. Companions promoted separately: the dashboard consumer is `w1/m64`; the token-metering proxy stays `w7/023`.

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Transcript store: control-plane tables + ordered part/turn write model                          | 60m | — — **DONE** |
| t002 | Gateway attach listener: ticket verify, nonce claim, verbatim SSE reverse-proxy + transcript tee | 60m | t001 — **DONE** |
| t003 | Replay mode: replay-then-live splice; terminal sessions replay-only + `[DONE]`                  | 45m | t002 — **DONE** |
| t004 | Live prompt turn: driver turn endpoint + gateway `POST` forward; steer-verb narrowing           | 60m | t002 — **DONE** |
| t005 | `attach-ticket` mint verb (REST/GraphQL/MCP)                                                   | 45m | t002 — **DONE** |
| t006 | `api.bex.co` edge path-routing to the gateway process; cookie-ignore; header preservation      | 45m | t002 — **DONE** |
| t007 | Render parity                                                                                  | 30m | t003, t004, t005, t006 — **DONE** |
| t008 | Simplify                                                                                       | 20m | t007 — **DONE** |
| t009 | Test coverage                                                                                  | 60m | t007 — **DONE** (unit + real-Postgres + driver; `scripts/agent-session-verify.sh` extended with the attach/replay/turn/reattach legs — the live prod *run* of it shares the m41 gate) |
| t010 | Closeout                                                                                       | 10m | t009 — **DONE**: green live-substrate run on prod — `scripts/agent-session-verify.sh` passes create→PR (PRs #1–6), steering (turns=2), branch refusal, and every conversation-API leg (attach-ticket mint, ticketless 401, attach+replay v1+[DONE], live-attach 63 teed parts, reattach replay from durable transcript). |

## Definition of done

- With a valid ticket, **any AI SDK client** (`useChat` + `DefaultChatTransport`, or curl) consumes `GET https://api.bex.co/v1/agent-sessions/{id}/stream`: a running session replays the stored transcript then follows live parts; a terminal session replays and closes with `[DONE]`; the `x-vercel-ai-ui-message-stream: v1` header arrives end to end and every byte matches what the driver emitted (verbatim-forward guarantee — no re-encoding, filtering, or injection).
- `POST …/stream` on a live session submits a prompt turn that the in-sandbox agent executes on the same branch; the turn's parts are teed into the transcript and visible to concurrently attached clients.
- `POST /v1/agent-sessions/{id}/attach-ticket` (+ GraphQL/MCP twins) mints a reconnect ticket for a running session after `can_operate`; tickets keep the 90s TTL + DB single-use nonce + subject/session/pod/namespace claims; a page-reload reattach works.
- The stream path is served by the **gateway process** behind the primary API origin via edge path-routing; cookies on that path are ignored (ticket is the only credential); the audit-assumption exception is documented.
- Steer narrowing: a live-session steer points to the stream `POST` (documented 409 or transparent absorption per t004's decision); idle-session redispatch is unchanged; the MCP `steer_agent_session` alias still works one-shot.
- Every stream byte teed durably; replay after gateway restart works (store, not memory, is the source).
- Backend + gateway suites and lint green; a live E2E extension of `scripts/agent-session-verify.sh` proves attach/replay/turn/reattach on the real substrate.

## Source + Goal linkage

- **Source:** promoted from held note `w3/009` (gateway proxy + transcript store, ADR047 wave 2) after the 2026-08-02 target-API-shape decision ([docs/ADR047-cloud-coding-agent-sessions.md](../../../docs/ADR047-cloud-coding-agent-sessions.md) § D9 "Target API shape" + § D3 verbatim-forward guarantee) fixed the public contract: same-origin publication, single conversation endpoint with replay mode, steer absorption, attach-ticket mint.
- **Goal linkage:** ADR008 pillar 5 — turns the shipped fire-and-forget product (m39/m41) into a live, externally consumable streaming API; unblocks `w1/m64` (dashboard) and `w11/m7` (mobile live attach).
- **Expected outcome:** the acp-ai-provider-produced UI-message stream is safely exposed as a first-class bex API on the primary origin; reconnect, multi-client attach, and terminal-session history all work off the durable transcript.
- **Why now:** m41 is implementation-complete pending only the live E2E, m42 removed the hibernation blocker, and the D9 decision froze the contract — the remaining hold is only the m41 closeout confirmation. Render parity task included: the attach-ticket verb and stream contract are tenant-facing across REST/GraphQL/MCP (m39 precedent; Render has no equivalent — bex-extension row).

## Out of scope

- Raw-ACP WebSocket IDE attach (phase 2+ — same ticket path, folds in later).
- Token metering / LLM proxy (`w7/023`, ADR047 D6).
- Optional JSON transcript read for MCP/REST parity and +/− diff stats in evidence — file as follow-up notes if not trivially absorbed.
