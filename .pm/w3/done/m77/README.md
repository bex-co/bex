# w3 · m77 — Agent-session transcript persistence: the headless recorder (ADR051)

**Worker:** worker3 **Goal:** every completed **fire-and-forget** agent session has a durable conversation transcript, so opening `/agents/{id}` replays the real conversation instead of "No conversation yet." **Status:** done (audited 2026-08-30; revised log-harvest mechanism production-verified 2026-08-18)

## Tasks (in order)

| id   | title                                                          | est | depends_on         |
| ---- | -------------------------------------------------------------- | --- | ------------------ |
| t001 | Gateway internal record endpoint (dial driver, tee replay)    | 60m | — — **DONE**       |
| t002 | Completer record-before-teardown trigger                      | 45m | t001 — **DONE**    |
| t003 | Env + config wiring + `.env.example`/CLAUDE.md/ADR051 sync     | 30m | t001 — **DONE**    |
| t004 | Simplify the changed code (`/simplify`)                        | 20m | t001, t002, t003 — **DONE** |
| t005 | Test coverage: unit + real-PG/E2E headless-then-open           | 45m | t004 — **DONE**    |
| t006 | Closeout                                                       | 10m | t005 — **DONE**    |

## Definition of done

A fire-and-forget agent session created with **no browser ever attached**, after the Completer finalizes it and tears its sandbox down, has a **non-empty `agent_session_transcripts`** for that session; `GET /v1/agent-sessions/{id}/stream` (terminal replay) returns the full conversation and the dashboard shows it instead of "No conversation yet." The record path is **idempotent** (composes with the phase-2 live tee — no duplicate seqs, seeded from `AgentSessionTranscriptMaxSeq`) and **best-effort** (a record failure never strands a session in `running`). Proven by a real-Postgres unit/integration test and the extended `scripts/agent-session-verify.sh` headless-then-open leg on the prod substrate. bex-api gains no new `pods/exec`.

## Source + Goal linkage

- **Source:** [docs/ADR051-agent-session-transcript.md](../../../../docs/ADR051-agent-session-transcript.md) (split from ADR047 D10; gap found in prod 2026-08-06). Extends **w3/m43** (transcript store + gateway attach listener) and **w3/m41** (phase-1 fire-and-forget). Related inbox note `w3/015` (transcript part timestamps) stays separate.
- **Goal linkage:** ADR008 pillar 5 (AI-native cloud coding-agent sessions) — the conversation is a first-class deliverable; without persistence the shipped phase-1 product records no history.
- **Expected outcome:** completed fire-and-forget sessions replay their full conversation; "No conversation yet." disappears with zero frontend change.
- **Why now:** the phase-1 product is **live** and **every** completed session is currently blank — a visible correctness gap on a shipped user-facing surface. The fix is mostly wiring over already-shipped primitives (driver hub full-history replay, the gateway `spliceDriverStream` tee, the Completer's gateway-exec boundary + teardown, the idempotent transcript store), so it is low-risk and high-visibility.
- **Render parity task OMITTED:** no new REST/GraphQL/MCP field or UI surface. The transcript is served over the existing **REST-only** `GET /v1/agent-sessions/{id}/stream` (ADR047 D9), and the dashboard already consumes its replay mode unchanged. This milestone is a persistence/backfill mechanism behind an existing surface, so there is nothing to fan out across surfaces.

## Implementation notes (what shipped)

- The first `/agent-record` network-dial design was replaced on 2026-08-08 by the production-proven **log-harvest path** (`e8e611c5`): the driver writes ordinal-tagged UI-message parts to `/var/log/bex-agent/session.jsonl`; the Completer reads that file over the same isolated gateway `pods/exec` boundary used for status, appends from the current durable max sequence, and only then tears down the sandbox. The unused gateway recorder endpoint and its extra secret/env surface were removed.
- The append remains idempotent and composes with live attach: `(session_id, turn, part_index)` conflicts are harmless, redispatched turns concatenate, and a partial/incomplete terminal turn is retained with an honest bounded marker rather than fabricated output.
- Transcript harvest is best-effort with respect to lifecycle convergence: a missing/failed Pod produces a durable incomplete-history outcome and never strands the session in `running`. bex-api still gains no direct `pods/exec`; the gateway remains the sole holder.
- Local gates were green at implementation: backend build/test/vet/lint, focused Completer/gateway/store tests, and real-Postgres transcript assertions.

## Production evidence

- Workflow run `32194179340` deployed the recovery-complete implementation on 2026-08-18. The credential-free record is [`w5/m72/evidence/2026-08-18-git-delivery-recovery.md`](../../../w5/done/m72/evidence/2026-08-18-git-delivery-recovery.md).
- The main verifier session `ags-da2ege4qlbqc73e84d0g` ran fire-and-forget to terminal completion before attach. Its later attach replay returned the v1 marker, **121 durable transcript parts**, and `[DONE]`; steering appended a second completed turn. A separate live-attach session replayed 40 durable parts after reattach. The run reported `ALL AGENT-SESSION CHECKS PASSED` and cleaned up its exact session/sandbox fixtures.
- `w5/m72` also refreshed the production dashboard/session route and proved conversation history remained readable across refresh, while the post-migration gateway-role check eliminated the intervening `agent_session_turns` permission outage.
- Successful production deployment run `33295901592` (2026-08-30, commit `c10c4f37`) contains the complete mechanism and later recovery fixes.
