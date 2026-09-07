# w5 · m85 — Crash-safe agent-session dispatch

**Worker:** worker5 **Goal:** Recover interrupted sandbox dispatches without stranded sessions or orphaned workloads. **Status:** done

## Source + Goal linkage

- **Source:** w5/050, observed production restart during m83; explicitly picked up by the user’s all-pending-w5 triage goal.
- **Goal linkage:** ADR047 cloud coding sessions must remain recoverable across control-plane restarts.
- **Expected outcome:** A process dying between sandbox creation and database binding leads to safe recovery or an actionable failure with cleanup.
- **Why now:** Current polling skips empty sandbox IDs forever; existing timeout lives only in the crashed process.
- **Parity:** Shared session status/failure semantics remain consistent across REST, GraphQL, MCP and dashboard. Render has no equivalent coding-session dispatch surface.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Persist recoverable dispatch intent and generation guards — **DONE** | 45m | — |
| t002 | Recover interrupted dispatches and clean orphaned sandboxes — **DONE** | 60m | t001 |
| t003 | Verify shared API and dashboard failure semantics — **DONE** | 20m | t002 |
| t004 | Simplify dispatch recovery changes — **DONE** | 20m | t003 |
| t005 | Exercise restart, race, and cleanup failure cases — **DONE** | 40m | t004 |
| t006 | Close out verified dispatch recovery — **DONE** | 10m | t005 |

## Definition of done

Persist dispatch intent and guard against stale generations/terminal resurrection. Reconcile interrupted dispatches, clean orphaned sandboxes, expose an actionable status through existing APIs, and prove restart and cleanup behavior with meaningful tests. Complete simplify review and affected checks, then ship.

## Verified implementation and limits — 2026-09-06

Acceptance writes a durable intent in the same transaction as the session/turn. Binding and abandonment serialize against the accepted turn, preventing late workers from resurrecting canceled/failed sessions or binding a newer turn. Intent metadata includes the previous sandbox ID so a restart during steering cannot lose its teardown obligation. Both migration backfill and poll-time discovery cover old replicas accepting work during a rolling upgrade.

After the existing 15-minute provisioning allowance, interrupted dispatches settle to an actionable failure through the existing shared status/failure-reason fields; users can retry. Cleanup targets only the exact generation or persisted predecessor, meters before/after termination, continues past individual termination failures, and retains a durable retry after outages or process loss. A deleted session's policy is cleaned repeatedly; an existing session's policy is preserved for newer turns.

Recovery runs after ordinary completion, with a 10-second budget, an indexed pending-first batch of 100, workspace-grouped discovery, and rescheduling before remote I/O. Successful cleanup tombstones remain checked hourly. They are intentionally retained because upstream has no cancellation fence proving a timed-out remote create can never materialize later. This bounds per-tick work, not retained tombstone storage; no arbitrary safe-retention claim is made.

### Validation

- Real Postgres regression cases cover acceptance/process loss, terminalization before cleanup, late create, retry isolation, predecessor custody, canceled/deleted sessions, old-replica acceptance/binding during upgrade, and 12 concurrent bind-versus-abandon races.
- Full backend run passed every non-store package. Store fixture assumptions were updated for the stricter single-bind contract. Its final complete race run passed on a fresh disposable Postgres database (38.481s).
- An unrelated replay-epoch test exposed a timing assumption: claims refreshed leases after the test's initial clock anchor. The test now anchors simulated expiry after those writes; no sleeps, retries, skips or production behavior changes were added.
- Agent-sessions, sandbox and metrics race suites passed; backend lint reported zero issues. Reuse, quality and efficiency reviews completed and actionable cleanup findings were fixed.
- Shared `failureReason` projection is already consumed by REST/MCP, GraphQL and the dashboard failure callout. No new API field or Render-equivalence claim is introduced; Render has no coding-session dispatch counterpart.
- No production crash/restart drill was performed for m85. Recovery proof is from real-Postgres concurrency tests and sandbox HTTP lifecycle tests. Disposable databases were removed after verification.
