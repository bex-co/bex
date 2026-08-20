# w4 · m87 — Durable timing for replayed agent transcripts

**Worker:** worker4 **Goal:** preserve the elapsed timing of agent work as durable transcript data, so a terminal replay or reconnect renders the same `Worked for Ns` context as the original live conversation. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Timestamp every driver-emitted typed transcript part — **DONE** | 45m | w5/m66/t007 |
| t002 | Extend typed transcript schemas with legacy-safe timestamps — **DONE** | 30m | t001 |
| t003 | Derive durable work durations across live, replay, and reconnect — **DONE** | 45m | t002 |
| t004 | Render parity — **DONE** | 30m | t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004 |
| t007 | Closeout — **DONE** | 10m | t005, t006 |

## Definition of done

- Every new driver-emitted typed UI-message part carries an optional source timestamp in one documented UTC wire format, assigned once at the driver publication boundary.
- The gateway, durable transcript store, replay path, and live attach path forward timestamps byte-transparently; no database schema or gateway rewriting is introduced.
- Live delivery, terminal replay, and reconnect of the same transcript render the same non-negative elapsed `Worked for Ns` durations without double-counting replayed parts.
- Legacy and mixed transcripts without timestamps remain readable and use the existing fallback presentation rather than inventing time.
- Driver fixtures and dashboard tests cover live/replay equivalence, reconnect dedupe, clock/pathological input, and legacy fallback.

## Evidence

- **Driver:** `stampSourceTimestamp` in `lego/agent-image/driver/src/timestamp.ts` assigns ISO-8601 UTC `at` once (`Date.toISOString()` shape) at the mapper (`acp-map.ts`) and `session.ts` `publish` choke point; replay preserves the original. `npm test` in `lego/agent-image/driver` — 59 pass, including hub-history ≡ session-log `part.at` by `partIndex`. `w5/m66/t007` is still open on the board; the typed mapper this work extends is already the production seam.
- **Dashboard:** closed source intervals render `Worked for 40s` / `Thought for 12s` identically on one-frame replay and live tail; mixed/invalid timing keeps a bare `Worked`. Reconnect dedupe via `collapseDoubledParts` does not inflate elapsed. `yarn test src/features/agent-sessions` — 167 pass; `yarn lint` green.
- **Compatibility:** no gateway rewrite, no DB column, no REST/GraphQL/MCP `AgentSession` field change (`lego/backend/internal/agentsessions/graphql.go` unchanged). Mobile has no transcript renderer and ignores `at`.
- **Parity:** ADR018 documents part timing as a Bex-only additive on the UI-message stream; ADR047 D3 invariant 5 and ADR051 harvest notes record the same contract.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w4`, 2026-08-18, materializing `.pm/w3/015.md`; sequenced after `w5/m66` because that milestone replaces the exact ACP→typed-part mapping seam this work extends.
- **Goal linkage:** ADR008 pillar 5 and ADR047/ADR051: the agent conversation is a durable product deliverable, and persisted evidence should retain the timing context visible during live execution.
- **Expected outcome:** reopening a completed session shows meaningful persisted durations instead of degrading every activity group to a timeless `Worked`/`Thought` label.
- **Why now:** transcript persistence and replay are shipped, while `w5/m66` is consolidating typed part creation into one source. Adding timestamps immediately after that closeout avoids parallel edits and gives the new single mapping seam a complete durable contract.
- **Render parity:** included because the dashboard conversation changes visibly. Render has no agent-session equivalent, so the pass must document the Bex extension, verify existing REST/GraphQL/MCP session fields remain unchanged, and ensure every Bex transcript consumer interprets the additive timestamp consistently.
