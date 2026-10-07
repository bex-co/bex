# w5 · m132 — A sandbox-capacity failure carries a code, and the dashboard's upgrade callout decides by it

**Worker:** worker5 **Goal:** A session that failed for sandbox capacity records `SANDBOX_CAPACITY_LIMIT` beside its sentence and serves it as `failureReasonCode` on REST, GraphQL and MCP. The dashboard's "Upgrade plan" callout decides by that code, never by the reason's wording. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Agent sessions store a failure code beside the failure sentence — **DONE** | 1h | — |
| t002 | The agent-session view carries `failureReasonCode` on every surface — **DONE** | 45m | t001 |
| t003 | The dashboard's capacity callout decides by the code — **DONE** | 45m | t002 |
| t004 | Render parity — **DONE** | 20m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 30m | t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

- A dispatch refused for sandbox capacity stores `failure_reason_code = 'SANDBOX_CAPACITY_LIMIT'` in the same write as its sentence.
  - Every other failure stores an empty code, and every write that clears or replaces the reason clears or replaces the code with it.
  - Sessions that failed for capacity before the migration are backfilled, whether their sentence sits in `failure_reason` or, from before w5/m80 (9724686a7), in `status`.
- REST, GraphQL and MCP agent-session reads carry `failureReasonCode`. REST and MCP omit the key when it is empty, and GraphQL returns null.
- The dashboard's failure callout shows the upgrade action exactly when the code is `SANDBOX_CAPACITY_LIMIT`. A reason worded differently but carrying the code shows it. A reason that only mentions "sandbox capacity" does not.
- The backend suite on real Postgres, dashboard `yarn test` and `make lint` pass.

## Source + Goal linkage

- **Source:** promoted from inbox `w5/122` (found by w5/m130's t001 sweep, 2026-10-07). The note:
  - w5/m130 moved every dashboard refusal decision onto codes, but `isSandboxCapacityFailure` (`dashboard/src/features/agent-sessions/lib/mapper.ts`) still decides a failed session's "Upgrade plan" callout from `"sandbox capacity"` in its stored `failureReason` or `status` text. Rewording that reason would silently drop the callout.
  - bex-api stores `sandbox.CapacityFailureReason` ("sandbox capacity reached") as the session's `failureReason`. This is a stored status string, not a refusal, so it carries no code.
  - Fix: add a machine-readable reason code beside the sentence, as w5/m129 did for Key Value (`statusReason` with `statusReasonCode`), expose it on REST, GraphQL and MCP's agent-session reads, and have the mapper decide by it.
  - Test: a differently worded reason carrying the capacity code still shows the callout; a reason that only mentions "sandbox capacity" without the code does not.
- **Triage (2026-10-07):** the code is known only where the failure happens: `sandbox.IsCapacityLimit(err)` in `agentsessions.dispatch`. `sandbox.CodeSandboxCapacityLimit` is documented as the code a caller recording the refusal as a session failure should match on. Deriving the code from the stored sentence on read would only move the wording match into bex-api, and a reworded constant would still drop it for sessions already stored. So the code is stored when the failure is written, as `service_event_facts.reason_code` is.
- **Goal linkage:**
  - ADR006: one view across REST, GraphQL and MCP.
  - w5/m130's rule: the dashboard branches on codes, not text.
- **Expected outcome:** a free-tier user who hits the workspace sandbox cap keeps getting the upgrade action whatever the sentence says, in any language the dashboard renders.
- **Why now:** this is the last text-based decision w5/m130's sweep found, and the refusal code it needs already exists.
- **Render parity included:** the change adds a field to the agent-session view on REST, GraphQL and MCP and changes a dashboard decision. Render has no agent sessions, so parity here is consistency across bex's own surfaces.

## Result (2026-10-07)

- **The code is stored with the failure.** Migration 0144 adds `agent_sessions.failure_reason_code`.
  - `AbandonAgentDispatch` takes a `store.AgentSessionFailure{Reason, Code}`. The dispatch path records `SANDBOX_CAPACITY_LIMIT` beside `sandbox capacity reached` in the same write.
  - It is the only writer of a code. A new turn, a recorded dispatch, another failure, a finalize and a hibernation expiry each clear it.
  - The backfill codes sessions refused for capacity before the migration, whether the sentence sits in `failure_reason` or, before w5/m80, in `status`.
- **Every surface serves it.** REST, GraphQL and MCP agent-session reads carry `failureReasonCode`. REST and MCP omit it when empty, and GraphQL answers null.
- **The dashboard decides by it.** `isSandboxCapacityFailure` is `phase === "failed" && failureReasonCode === SANDBOX_CAPACITY_LIMIT` and reads no sentence.
  - A dashboard test reads `sandbox.CodeSandboxCapacityLimit` from bex-api's Go source, so the two cannot drift.
  - A store test pins the backfill's code to the same constant.
- **Simplify (t005).**
  - The first draft kept the code through a finalize with an unchanged sentence, for the reaper's unclaim. Review showed that a coded row never holds the sandbox a claim needs. It also showed that an old replica's steer during a rolling deploy clears the sentence but not the code, so that rule kept a stale code when both sentences were empty. A finalize now clears the code unconditionally.
  - The doc comments on `sandbox.CapacityFailureReason` and `IsCapacityLimit` no longer invite matching on the sentence.
  - The tests share an MCP client helper and a real-Postgres setup, and the Open in Zed test uses the shared session mock.
- **Tests (t006).** Fifteen mutants each fail a test:
  - each of the six writers;
  - the view, the dispatch branch and GraphQL's null;
  - the backfill's `status` clause and its code;
  - four on the dashboard: the phase check, the mapping, a wording match and code drift.
- **Known residual.** During a rolling deploy, an old replica can steer a capacity-coded session and then fail it again. It writes its own sentence without clearing the code, so the callout offers an upgrade beside the wrong sentence until the next write. This needs a user retry inside the rollout window, and it heals on the next steer.
- **Suites.**
  - Backend: the agent-session, store (real Postgres) and sandbox packages pass.
  - Dashboard: `yarn lint` and `yarn test` (4,391 tests) pass.
  - Go: `make lint` passes across the four modules.
  - The full real-database backend suite runs in the ship gate.
