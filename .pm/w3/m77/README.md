# w3 · m77 — Agent-session transcript persistence: the headless recorder (ADR051)

**Worker:** worker3 **Goal:** every completed **fire-and-forget** agent session has a durable conversation transcript, so opening `/agents/{id}` replays the real conversation instead of "No conversation yet." **Status:** implementation complete + locally verified (backend `go build`/`go test`/`go vet` + `make lint-backend` green; unit + real-Postgres-gated coverage); **awaiting the operator live E2E run** (`scripts/agent-session-verify.sh` step 5b′ on prod, shared m41 gate) to satisfy the DoD and close (t006)

## Tasks (in order)

| id   | title                                                          | est | depends_on         |
| ---- | -------------------------------------------------------------- | --- | ------------------ |
| t001 | Gateway internal record endpoint (dial driver, tee replay)    | 60m | — — **DONE**       |
| t002 | Completer record-before-teardown trigger                      | 45m | t001 — **DONE**    |
| t003 | Env + config wiring + `.env.example`/CLAUDE.md/ADR051 sync     | 30m | t001 — **DONE**    |
| t004 | Simplify the changed code (`/simplify`)                        | 20m | t001, t002, t003 — **DONE** |
| t005 | Test coverage: unit + real-PG/E2E headless-then-open           | 45m | t004 — **DONE**    |
| t006 | Closeout                                                       | 10m | t005               |

## Definition of done

A fire-and-forget agent session created with **no browser ever attached**, after the Completer finalizes it and tears its sandbox down, has a **non-empty `agent_session_transcripts`** for that session; `GET /v1/agent-sessions/{id}/stream` (terminal replay) returns the full conversation and the dashboard shows it instead of "No conversation yet." The record path is **idempotent** (composes with the phase-2 live tee — no duplicate seqs, seeded from `AgentSessionTranscriptMaxSeq`) and **best-effort** (a record failure never strands a session in `running`). Proven by a real-Postgres unit/integration test and the extended `scripts/agent-session-verify.sh` headless-then-open leg on the prod substrate. bex-api gains no new `pods/exec`.

## Source + Goal linkage

- **Source:** [docs/ADR051-agent-session-transcript.md](../../../docs/ADR051-agent-session-transcript.md) (split from ADR047 D10; gap found in prod 2026-08-06). Extends **w3/m43** (transcript store + gateway attach listener) and **w3/m41** (phase-1 fire-and-forget). Related inbox note `w3/015` (transcript part timestamps) stays separate.
- **Goal linkage:** ADR008 pillar 5 (AI-native cloud coding-agent sessions) — the conversation is a first-class deliverable; without persistence the shipped phase-1 product records no history.
- **Expected outcome:** completed fire-and-forget sessions replay their full conversation; "No conversation yet." disappears with zero frontend change.
- **Why now:** the phase-1 product is **live** and **every** completed session is currently blank — a visible correctness gap on a shipped user-facing surface. The fix is mostly wiring over already-shipped primitives (driver hub full-history replay, the gateway `spliceDriverStream` tee, the Completer's gateway-exec boundary + teardown, the idempotent transcript store), so it is low-risk and high-visibility.
- **Render parity task OMITTED:** no new REST/GraphQL/MCP field or UI surface. The transcript is served over the existing **REST-only** `GET /v1/agent-sessions/{id}/stream` (ADR047 D9), and the dashboard already consumes its replay mode unchanged. This milestone is a persistence/backfill mechanism behind an existing surface, so there is nothing to fan out across surfaces.

## Implementation notes (what shipped)

- **Gateway recorder** (`internal/sshgateway/agentattach/record.go`): internal `POST /agent-record` on the exec listener (`:8081`), auth'd by `BEX_SANDBOX_EXEC_SECRET` (distinct from the browser ticket key), no edge route. Dials the still-live driver `/stream`, drains the full replay, and **batch-appends** it in one store call (no live consumer → no per-part round-trips, unlike the browser splice). Idempotent per turn via `AgentSessionTranscriptTurnRecorded`; seeds seq from `AgentSessionTranscriptMaxSeq` so a **redispatched (steered) turn concatenates** onto prior turns. Bounded by a dedicated 2-min drain timeout so a `[DONE]`-less driver can't stall teardown.
- **Completer trigger** (`internal/agentsessions/completion.go` `teardown`): records **before** `CancelAgentSessionSandbox` (the driver's in-memory history dies with the pod); best-effort — a failure is logged, never strands the session. `internal/sandbox` `RecordTranscript` mints an exec-secret-signed agent ticket (system subject, pod `<sandbox>-0`, turn) and POSTs the recorder; bex-api never gains `pods/exec`.
- **Turn stamping**: the attach ticket now carries `Turn` (`agentsessionticket.Claims`), and both the live tee and the recorder stamp `turn` on each part, which makes the per-turn idempotency guard robust against a live viewer that already teed a turn.
- **Known limits (documented, out of scope):** a live viewer that tees only *part* of a turn then drops makes the recorder skip that turn (partial > empty); the recorder assumes a per-turn driver hub (the fire-and-forget shape). Both are noted in ADR051.
- **Verified:** `go build`/`go test ./...`/`go vet` + `make lint-backend` green; unit tests (recorder tee/idempotency/concat/gone-pod/wrong-secret, Completer record-before-teardown + best-effort), real-Postgres `TurnRecorded` assertions, and `scripts/agent-session-verify.sh` step 5b′ (fire-and-forget replay non-empty). The prod E2E run (t006) is the remaining gate.
