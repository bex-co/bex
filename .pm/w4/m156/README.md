# w4 · m156 — Report honest start times for successful deploys

**Worker:** worker4 **Goal:** successful deploy durations and log chronology reflect observed execution evidence **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Audit successful-skip evidence and sibling consumers | 35m | — |
| t002 | Preserve evidence or unknown starts on successful terminal skips | 55m | t001 |
| t003 | Render parity and live timestamp verification | 30m | t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Test coverage | 40m | t003 |
| t006 | Closeout | 15m | t004, t005 |

## Definition of done

- Fresh image creation and restart that skip observed progress never report the completion observation as their execution start. If no owned start evidence exists, REST/MCP omit it, GraphQL retains its established empty missing-value encoding, and dashboard detail/history show unknown duration with no invented start timeline step.
- Generation-attributed build evidence can preserve a true start; existing evidenced starts remain byte-for-byte unchanged. Created and queued skips, stale build windows, image/reuse and native-build cases have behavioral coverage.
- For newly handled terminal skips, synthetic start narration is omitted without evidence or uses the recorded build-window start; existing in-progress sampling timestamps are preserved, not promised to predate every app log. Queued/terminal and actual application logs remain. Build facts, Events and future analytics durations agree on known versus unknown execution time.
- Failure/cancel/supersede/deactivation contracts, eleven statuses, queue exclusion and resource authorization remain unchanged. The five App families are dispositioned explicitly; Database lifecycles remain out of scope.
- PG and memory implementations, API adapters and dashboard consumers are verified; appropriate backend/dashboard checks pass. Live image and native controls are captured, fixtures deleted, and session revoked.

## Source + Goal linkage

- **Source:** user-requested continuous qa-find-bugs sweep 34 using muse.env, filing in w4; [full evidence and research](finding.md).
- **Goal linkage:** ADR004 deployment lifecycle and ADR018 hosting parity; reliable facts for both dashboard users and API/MCP agents.
- **Expected outcome:** no fabricated zero-duration success or start banner after the application has already started.
- **Why now:** two independently triggered successful image rollouts reproduced the gap after the admission fix made this journey usable. Shared timestamp consumers require a coordinated fix rather than a display-only patch.
- **Render parity:** included because REST, GraphQL, MCP and the dashboard expose these timestamps; preserve missing-value compatibility and document vendor timing limits.
- **Estimate:** 190m across six tasks, including consumer audit and required closing tasks.
